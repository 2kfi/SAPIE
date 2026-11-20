package state

import (
	"database/sql"
	"errors"
)

var (
	ErrSelfRequest    = errors.New("cannot send a request to yourself")
	ErrPairBlocked    = errors.New("pair is blocked")
	ErrAlreadyFriends = errors.New("already friends")
	ErrRequestExists  = errors.New("request already exists")
	ErrNotRecipient   = errors.New("only the recipient may answer a request")
	ErrNotSender      = errors.New("only the sender may cancel a request")
	ErrRequestInvalid = errors.New("request not actionable")
	ErrSearchQuery    = errors.New("search query must be 2 to 64 characters")
)

// FriendRequest is the public view of a request. PersonaName names the other
// participant, so a caller never needs a full account row.
type FriendRequest struct {
	ID          int64  `json:"id"`
	FromSteamID uint64 `json:"from_steam_id"`
	ToSteamID   uint64 `json:"to_steam_id"`
	PersonaName string `json:"persona_name"`
	Message     string `json:"message"`
	CreatedAt   int64  `json:"created_at"`
	Status      string `json:"status"`
}

type FriendRequests struct {
	Incoming []FriendRequest `json:"incoming"`
	Outgoing []FriendRequest `json:"outgoing"`
}

type BlockedUser struct {
	SteamID     uint64 `json:"steam_id"`
	PersonaName string `json:"persona_name"`
	CreatedAt   int64  `json:"created_at"`
}

type SearchUser struct {
	SteamID     uint64 `json:"steam_id"`
	PersonaName string `json:"persona_name"`
}

const maxSearchResults = 10

func (s *Store) AccountBySteamID(steamID uint64) (Account, error) {
	var a Account
	err := s.db.QueryRow(`SELECT steam_id, username, persona_name, created_at FROM accounts WHERE steam_id = ?`, steamID).
		Scan(&a.SteamID, &a.Username, &a.PersonaName, &a.CreatedAt)
	return a, err
}

func (s *Store) IsBlocked(a, b uint64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM blocks WHERE
		(steam_id = ? AND blocked_id = ?) OR (steam_id = ? AND blocked_id = ?)`, a, b, b, a).Scan(&n)
	return n > 0, err
}

func (s *Store) areFriends(a, b uint64) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM friendships WHERE steam_id = ? AND friend_id = ?`, a, b).Scan(&n)
	return n > 0, err
}

// SendFriendRequest reuses an existing pending request from the same sender.
// Blocked pairs, existing friends and self requests are not distinguishable
// from a missing account at the HTTP layer.
func (s *Store) SendFriendRequest(from, to uint64, message string) (FriendRequest, error) {
	if from == to {
		return FriendRequest{}, ErrSelfRequest
	}
	if _, err := s.AccountBySteamID(to); err != nil {
		return FriendRequest{}, err
	}
	blocked, err := s.IsBlocked(from, to)
	if err != nil {
		return FriendRequest{}, err
	}
	if blocked {
		return FriendRequest{}, ErrPairBlocked
	}
	friends, err := s.areFriends(from, to)
	if err != nil {
		return FriendRequest{}, err
	}
	if friends {
		return FriendRequest{}, ErrAlreadyFriends
	}
	var existing int64
	err = s.db.QueryRow(`SELECT id FROM friend_requests WHERE status = 'pending' AND
		((from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)) ORDER BY id LIMIT 1`,
		from, to, to, from).Scan(&existing)
	if err == nil {
		return s.FriendRequestByID(existing, from)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FriendRequest{}, err
	}
	// A declined row keeps its pair unique, so a later request reopens it.
	var prior int64
	err = s.db.QueryRow(`SELECT id FROM friend_requests WHERE from_id = ? AND to_id = ?`, from, to).Scan(&prior)
	if err == nil {
		if _, err := s.db.Exec(`UPDATE friend_requests SET status = 'pending', message = ?,
			created_at = CAST(strftime('%s', 'now') AS INTEGER) WHERE id = ?`, message, prior); err != nil {
			return FriendRequest{}, err
		}
		return s.FriendRequestByID(prior, from)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return FriendRequest{}, err
	}
	res, err := s.db.Exec(`INSERT INTO friend_requests (from_id, to_id, message) VALUES (?, ?, ?)`, from, to, message)
	if err != nil {
		return FriendRequest{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return FriendRequest{}, err
	}
	return s.FriendRequestByID(id, from)
}

func (s *Store) FriendRequestByID(id int64, viewer uint64) (FriendRequest, error) {
	var r FriendRequest
	err := s.db.QueryRow(`SELECT fr.id, fr.from_id, fr.to_id,
		CASE WHEN fr.from_id = ? THEN a2.persona_name ELSE a1.persona_name END,
		fr.message, fr.created_at, fr.status
		FROM friend_requests fr
		JOIN accounts a1 ON a1.steam_id = fr.from_id
		JOIN accounts a2 ON a2.steam_id = fr.to_id
		WHERE fr.id = ?`, viewer, id).
		Scan(&r.ID, &r.FromSteamID, &r.ToSteamID, &r.PersonaName, &r.Message, &r.CreatedAt, &r.Status)
	return r, err
}

func (s *Store) FriendRequestsOf(steamID uint64) (FriendRequests, error) {
	result := FriendRequests{Incoming: []FriendRequest{}, Outgoing: []FriendRequest{}}
	rows, err := s.db.Query(`SELECT fr.id, fr.from_id, fr.to_id, other.persona_name,
		fr.message, fr.created_at, fr.status
		FROM friend_requests fr
		JOIN accounts other ON other.steam_id = CASE WHEN fr.from_id = ? THEN fr.to_id ELSE fr.from_id END
		WHERE fr.status = 'pending' AND (fr.from_id = ? OR fr.to_id = ?)
		ORDER BY fr.created_at DESC, fr.id DESC`, steamID, steamID, steamID)
	if err != nil {
		return FriendRequests{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var r FriendRequest
		if err := rows.Scan(&r.ID, &r.FromSteamID, &r.ToSteamID, &r.PersonaName, &r.Message, &r.CreatedAt, &r.Status); err != nil {
			return FriendRequests{}, err
		}
		if r.ToSteamID == steamID {
			result.Incoming = append(result.Incoming, r)
		} else {
			result.Outgoing = append(result.Outgoing, r)
		}
	}
	return result, rows.Err()
}

// AcceptFriendRequest writes the accepted graph and status in one transaction.
// Inserting both directions with OR IGNORE makes repeated accepts harmless.
func (s *Store) AcceptFriendRequest(id int64, steamID uint64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var to uint64
	var status string
	if err := tx.QueryRow(`SELECT to_id, status FROM friend_requests WHERE id = ?`, id).Scan(&to, &status); err != nil {
		return err
	}
	if to != steamID {
		return ErrNotRecipient
	}
	switch status {
	case "accepted":
		return tx.Commit()
	case "pending":
	default:
		return ErrRequestInvalid
	}
	from, err := s.requestSender(tx, id)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE friend_requests SET status = 'accepted' WHERE id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO friendships (steam_id, friend_id) VALUES (?, ?), (?, ?)`,
		from, to, to, from); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) requestSender(tx *sql.Tx, id int64) (uint64, error) {
	var from uint64
	err := tx.QueryRow(`SELECT from_id FROM friend_requests WHERE id = ?`, id).Scan(&from)
	return from, err
}

func (s *Store) answerFriendRequest(id int64, steamID uint64, status string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var to uint64
	var current string
	if err := tx.QueryRow(`SELECT to_id, status FROM friend_requests WHERE id = ?`, id).Scan(&to, &current); err != nil {
		return err
	}
	if to != steamID || current != "pending" {
		return ErrRequestInvalid
	}
	if _, err := tx.Exec(`UPDATE friend_requests SET status = ? WHERE id = ?`, status, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeclineFriendRequest(id int64, steamID uint64) error {
	return s.answerFriendRequest(id, steamID, "declined")
}

func (s *Store) CancelFriendRequest(id int64, steamID uint64) error {
	res, err := s.db.Exec(`DELETE FROM friend_requests WHERE id = ? AND from_id = ? AND status = 'pending'`, id, steamID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		var from uint64
		if err := s.db.QueryRow(`SELECT from_id FROM friend_requests WHERE id = ?`, id).Scan(&from); err != nil {
			return err
		}
		if from == steamID {
			return ErrRequestInvalid
		}
		return ErrNotSender
	}
	return nil
}

func (s *Store) RemoveFriend(a, b uint64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM friendships WHERE
		(steam_id = ? AND friend_id = ?) OR (steam_id = ? AND friend_id = ?)`, a, b, b, a)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	if _, err := tx.Exec(`DELETE FROM friend_requests WHERE status = 'pending' AND
		((from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?))`, a, b, b, a); err != nil {
		return err
	}
	return tx.Commit()
}

// BlockUser hides both directions of presence and cancels pending requests.
// The friendship rows stay so unblocking restores the previous graph.
func (s *Store) BlockUser(steamID, blockedID uint64) error {
	if steamID == blockedID {
		return ErrSelfRequest
	}
	if _, err := s.AccountBySteamID(blockedID); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR IGNORE INTO blocks (steam_id, blocked_id) VALUES (?, ?)`, steamID, blockedID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM friend_requests WHERE status = 'pending' AND
		((from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?))`, steamID, blockedID, blockedID, steamID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) UnblockUser(steamID, blockedID uint64) error {
	_, err := s.db.Exec(`DELETE FROM blocks WHERE steam_id = ? AND blocked_id = ?`, steamID, blockedID)
	return err
}

func (s *Store) BlockedUsers(steamID uint64) ([]BlockedUser, error) {
	rows, err := s.db.Query(`SELECT b.blocked_id, a.persona_name, b.created_at
		FROM blocks b JOIN accounts a ON a.steam_id = b.blocked_id
		WHERE b.steam_id = ? ORDER BY b.created_at DESC, b.blocked_id`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []BlockedUser{}
	for rows.Next() {
		var u BlockedUser
		if err := rows.Scan(&u.SteamID, &u.PersonaName, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SearchUsers matches an exact username or a leading prefix. It never returns
// password hashes or full account rows and is capped at maxSearchResults.
func (s *Store) SearchUsers(viewer uint64, query string) ([]SearchUser, error) {
	if len(query) < 2 || len(query) > 64 {
		return nil, ErrSearchQuery
	}
	rows, err := s.db.Query(`SELECT steam_id, persona_name FROM accounts
		WHERE steam_id <> ? AND (lower(username) = lower(?) OR substr(lower(username), 1, ?) = lower(?))
		ORDER BY username LIMIT ?`, viewer, query, len(query), query, maxSearchResults)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []SearchUser{}
	for rows.Next() {
		var u SearchUser
		if err := rows.Scan(&u.SteamID, &u.PersonaName); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}
