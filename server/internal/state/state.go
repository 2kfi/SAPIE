// Package state owns the SQLite store and server-assigned SteamIDs.
package state

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sapie/sapie/server/migrations"
)

const SteamIDBase uint64 = 76561197960265728

// Steam lobby SteamID layout: universe 8 bits, account type 4 bits,
// instance 20 bits, account ID 32 bits (little-endian field order).
const (
	steamUniversePublic       uint64 = 1
	steamAccountTypeChat      uint64 = 8
	steamChatInstanceFlags    uint64 = 0x60000 // Lobby | MMSLobby
	steamAccountTypeShift     uint64 = 52
	steamUniverseShift        uint64 = 56
	steamAccountInstanceShift uint64 = 32
)

// LobbySteamID composes a Steam-shaped lobby SteamID. The low 32 bits stay
// equal to accountID so CSteamID.GetAccountID() keeps resolving the lobby.
func LobbySteamID(accountID uint32) uint64 {
	return steamUniversePublic<<steamUniverseShift |
		steamAccountTypeChat<<steamAccountTypeShift |
		steamChatInstanceFlags<<steamAccountInstanceShift |
		uint64(accountID)
}

// IsLobbySteamID reports whether id has the public Chat Lobby|MMSLobby layout.
func IsLobbySteamID(id uint64) bool {
	const prefixMask = uint64(0xFFFFF)<<steamAccountInstanceShift | 0xF<<steamAccountTypeShift | 0xFF<<steamUniverseShift
	const expected = steamChatInstanceFlags<<steamAccountInstanceShift | steamAccountTypeChat<<steamAccountTypeShift | steamUniversePublic<<steamUniverseShift
	return id&prefixMask == expected && uint32(id) != 0
}

type Account struct {
	SteamID     uint64 `json:"steam_id"`
	Username    string `json:"username"`
	PersonaName string `json:"persona_name"`
	CreatedAt   int64  `json:"created_at"`
}

// AuthAccount is internal: PasswordHash must never reach the admin API.
type AuthAccount struct {
	Account
	PasswordHash string
}

type Store struct {
	db                 *sql.DB
	mu                 sync.Mutex
	nextID             uint64
	nextLobbyAccountID uint32
	lobbyRequests      map[[2]uint64]uint64 // (owner, client request ID) -> assigned lobby ID
}

func Open(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if path == ":memory:" {
		dsn = "file::memory:?_pragma=foreign_keys(1)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// One connection keeps :memory: stable and serialises SQLite writes.
	db.SetMaxOpenConns(1)
	for _, sql := range migrations.Files {
		if _, err := db.Exec(sql); err != nil {
			db.Close()
			return nil, err
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS lobby_metadata (
		lobby_id INTEGER NOT NULL REFERENCES lobbies(lobby_id) ON DELETE CASCADE,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		PRIMARY KEY (lobby_id, key))`); err != nil {
		db.Close()
		return nil, err
	}
	// Databases created before M3 lack the lobby member limit column.
	var columns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('lobbies') WHERE name = 'member_limit'`).Scan(&columns); err != nil {
		db.Close()
		return nil, err
	}
	if columns == 0 {
		if _, err := db.Exec(`ALTER TABLE lobbies ADD COLUMN member_limit INTEGER NOT NULL DEFAULT 0`); err != nil {
			db.Close()
			return nil, err
		}
	}
	// Databases created before M4 lack the achievement definition columns.
	for _, col := range []struct{ name, ddl string }{
		{"display_name", `ALTER TABLE achievements ADD COLUMN display_name TEXT NOT NULL DEFAULT ''`},
		{"description", `ALTER TABLE achievements ADD COLUMN description TEXT NOT NULL DEFAULT ''`},
		{"hidden", `ALTER TABLE achievements ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0`},
		{"icon", `ALTER TABLE achievements ADD COLUMN icon TEXT NOT NULL DEFAULT ''`},
	} {
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('achievements') WHERE name = ?`, col.name).Scan(&columns); err != nil {
			db.Close()
			return nil, err
		}
		if columns == 0 {
			if _, err := db.Exec(col.ddl); err != nil {
				db.Close()
				return nil, err
			}
		}
	}
	var max sql.NullInt64
	if err := db.QueryRow(`SELECT MAX(steam_id) FROM accounts`).Scan(&max); err != nil {
		db.Close()
		return nil, err
	}
	next := SteamIDBase + 1
	if max.Valid && uint64(max.Int64) >= next {
		next = uint64(max.Int64) + 1
	}
	nextLobby := uint32(1)
	if err := db.QueryRow(`SELECT COALESCE(MAX(lobby_id & 0xFFFFFFFF), 0) FROM lobbies`).Scan(&max); err != nil {
		db.Close()
		return nil, err
	}
	if max.Valid && max.Int64 > 0 {
		nextLobby = uint32(uint64(max.Int64)&0xFFFFFFFF) + 1
		if nextLobby == 0 {
			nextLobby = 1
		}
	}
	return &Store{db: db, nextID: next, nextLobbyAccountID: nextLobby,
		lobbyRequests: make(map[[2]uint64]uint64)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// AdminToken returns the persisted token, or "" on a fresh database.
func (s *Store) AdminToken() (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = 'admin_token'`).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}

func (s *Store) SetAdminToken(token string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES ('admin_token', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, token)
	return err
}

func (s *Store) CreateAccount(username, passwordHash, personaName string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextID
	if _, err := s.db.Exec(
		`INSERT INTO accounts (steam_id, username, password_hash, persona_name) VALUES (?, ?, ?, ?)`,
		id, username, passwordHash, personaName,
	); err != nil {
		return 0, err
	}
	s.nextID++
	return id, nil
}

func (s *Store) AccountByUsername(username string) (*AuthAccount, error) {
	var a AuthAccount
	err := s.db.QueryRow(
		`SELECT steam_id, username, persona_name, created_at, password_hash FROM accounts WHERE username = ?`,
		username,
	).Scan(&a.SteamID, &a.Username, &a.PersonaName, &a.CreatedAt, &a.PasswordHash)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ListAccounts() ([]Account, error) {
	rows, err := s.db.Query(`SELECT steam_id, username, persona_name, created_at FROM accounts ORDER BY steam_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	accounts := []Account{}
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.SteamID, &a.Username, &a.PersonaName, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Store) DeleteAccount(steamID uint64) error {
	res, err := s.db.Exec(`DELETE FROM accounts WHERE steam_id = ?`, steamID)
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
	return nil
}

func (s *Store) CreateSession(steamID uint64, tokenHash string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token_hash, steam_id, expires_at) VALUES (?, ?, ?)`,
		tokenHash, steamID, expiresAt.Unix(),
	)
	return err
}

func (s *Store) ResolveSession(tokenHash string) (uint64, error) {
	var steamID uint64
	err := s.db.QueryRow(
		`SELECT steam_id FROM sessions WHERE token_hash = ? AND expires_at > CAST(strftime('%s', 'now') AS INTEGER)`,
		tokenHash,
	).Scan(&steamID)
	return steamID, err
}

const ticketNonceSize = 32

// TicketHash derives the opaque 32-byte ticket from its issuing identity and nonce.
func TicketHash(steamID uint64, appid uint32, nonce []byte) []byte {
	var input [8 + 4 + ticketNonceSize]byte
	binary.BigEndian.PutUint64(input[:8], steamID)
	binary.BigEndian.PutUint32(input[8:12], appid)
	copy(input[12:], nonce)
	sum := sha256.Sum256(input[:])
	return sum[:]
}

// IssueTicket records a ticket so it can be verified later. data holds the
// per-session number (4 bytes) followed by the nonce the ticket was derived from.
func (s *Store) IssueTicket(steamID uint64, appid, number uint32, nonce []byte, expiresAt time.Time) error {
	data := make([]byte, 4+len(nonce))
	binary.BigEndian.PutUint32(data[:4], number)
	copy(data[4:], nonce)
	_, err := s.db.Exec(`INSERT INTO auth_tickets (steam_id, appid, data, expires_at) VALUES (?, ?, ?, ?)`,
		steamID, appid, data, expiresAt.Unix())
	return err
}

// VerifyTicket reports whether ticket was issued to steamID for appid with
// that number and has not expired.
func (s *Store) VerifyTicket(ticket []byte, steamID uint64, appid, number uint32) (bool, error) {
	rows, err := s.db.Query(`SELECT data, expires_at FROM auth_tickets WHERE steam_id = ? AND appid = ?`, steamID, appid)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	now := time.Now().Unix()
	for rows.Next() {
		var data []byte
		var expiresAt int64
		if err := rows.Scan(&data, &expiresAt); err != nil {
			return false, err
		}
		if expiresAt <= now || len(data) != 4+ticketNonceSize || binary.BigEndian.Uint32(data[:4]) != number {
			continue
		}
		if subtle.ConstantTimeCompare(ticket, TicketHash(steamID, appid, data[4:])) == 1 {
			return true, nil
		}
	}
	return false, rows.Err()
}

const (
	ConnectStrKey   = "connect_str"
	PersonaStateKey = "persona_state"
)

// Presence holds the fields exposed by the friends and users HTTP endpoints.
type Presence struct {
	SteamID      uint64            `json:"steam_id"`
	PersonaName  string            `json:"persona_name"`
	PersonaState int32             `json:"persona_state"`
	Appid        uint32            `json:"appid"`
	RichPresence map[string][]byte `json:"rich_presence"`
	LobbyID      uint64            `json:"lobby_id"`
	ConnectStr   string            `json:"connect_str"`
}

type Lobby struct {
	LobbyID     uint64   `json:"lobby_id"`
	Owner       uint64   `json:"owner"`
	Appid       uint32   `json:"appid"`
	MemberLimit uint32   `json:"member_limit"`
	Type        uint32   `json:"type"`
	Joinable    bool     `json:"joinable"`
	Members     []uint64 `json:"members"`

	// Values and AssignRequestID are game-socket inputs, never API output.
	Values          map[string]string `json:"-"`
	AssignRequestID uint64            `json:"-"`
}

// LobbySearch is a discovery query. A zero field means no constraint.
type LobbySearch struct {
	Requester  uint64
	Appid      uint32
	Joinable   bool
	MinMembers uint32
	MaxMembers uint32
	Filters    []LobbyFilter
}

type LobbyFilter struct {
	Key     string
	String  *string
	Numeric *int64
	Compare LobbyComparison
}

type LobbyComparison uint8

const (
	CompareEqual LobbyComparison = iota
	CompareNotEqual
	CompareGreater
	CompareGreaterEqual
	CompareLess
	CompareLessEqual
)

const (
	LobbyTypePrivate     uint32 = 0
	LobbyTypeFriendsOnly uint32 = 1
	LobbyTypePublic      uint32 = 2
	LobbyTypeInvisible   uint32 = 3
)

var (
	ErrLobbyNotFound         = errors.New("lobby not found")
	ErrLobbyUnavailable      = errors.New("lobby full or not joinable")
	ErrLobbyNotOwner         = errors.New("only the lobby owner may change it")
	ErrLobbyNotAuthoritative = errors.New("lobby ID was not issued by this server")
)

// AddFriend records both directions so either side can resolve the other.
func (s *Store) AddFriend(a, b uint64) error {
	if a == b {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO friendships (steam_id, friend_id) VALUES (?, ?), (?, ?)`, a, b, b, a,
	)
	return err
}

func (s *Store) FriendsOf(steamID uint64) ([]uint64, error) {
	rows, err := s.db.Query(`SELECT f.friend_id FROM friendships f WHERE f.steam_id = ?
		AND NOT EXISTS (SELECT 1 FROM blocks b WHERE
			(b.steam_id = f.steam_id AND b.blocked_id = f.friend_id) OR
			(b.steam_id = f.friend_id AND b.blocked_id = f.steam_id))
		ORDER BY f.friend_id`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) FriendsPresence(steamID uint64) ([]Presence, error) {
	rows, err := s.db.Query(`SELECT f.friend_id, a.persona_name, COALESCE(p.appid, 0),
		COALESCE(p.lobby_id, 0), COALESCE(p.rich_presence, X'')
		FROM friendships f JOIN accounts a ON a.steam_id = f.friend_id
		LEFT JOIN presence p ON p.steam_id = f.friend_id
		WHERE f.steam_id = ?
		AND NOT EXISTS (SELECT 1 FROM blocks b WHERE
			(b.steam_id = f.steam_id AND b.blocked_id = f.friend_id) OR
			(b.steam_id = f.friend_id AND b.blocked_id = f.steam_id))
		ORDER BY f.friend_id`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	friends := []Presence{}
	for rows.Next() {
		var p Presence
		var blob []byte
		if err := rows.Scan(&p.SteamID, &p.PersonaName, &p.Appid, &p.LobbyID, &blob); err != nil {
			return nil, err
		}
		p.RichPresence = decodeRichPresence(blob)
		p.ConnectStr = string(p.RichPresence[ConnectStrKey])
		delete(p.RichPresence, ConnectStrKey)
		friends = append(friends, p)
	}
	return friends, rows.Err()
}

func (s *Store) PresenceOf(steamID uint64) (Presence, error) {
	var p Presence
	var name sql.NullString
	var blob []byte
	err := s.db.QueryRow(`SELECT a.persona_name, COALESCE(p.appid, 0),
		COALESCE(p.lobby_id, 0), COALESCE(p.rich_presence, X'')
		FROM accounts a LEFT JOIN presence p ON p.steam_id = a.steam_id WHERE a.steam_id = ?`,
		steamID,
	).Scan(&name, &p.Appid, &p.LobbyID, &blob)
	if err != nil {
		return Presence{}, err
	}
	p.SteamID = steamID
	p.PersonaName = name.String
	p.RichPresence = decodeRichPresence(blob)
	p.ConnectStr = string(p.RichPresence[ConnectStrKey])
	delete(p.RichPresence, ConnectStrKey)
	return p, nil
}

func (s *Store) UpsertPresence(p Presence) error {
	rich := p.RichPresence
	if rich == nil {
		rich = map[string][]byte{}
	}
	if p.ConnectStr != "" {
		rich[ConnectStrKey] = []byte(p.ConnectStr)
	}
	blob, err := json.Marshal(rich)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO presence (steam_id, appid, persona_name, lobby_id, rich_presence, updated_at)
		VALUES (?, ?, ?, ?, ?, CAST(strftime('%s', 'now') AS INTEGER))
		ON CONFLICT(steam_id) DO UPDATE SET appid = excluded.appid, persona_name = excluded.persona_name,
		lobby_id = excluded.lobby_id, rich_presence = excluded.rich_presence, updated_at = excluded.updated_at`,
		p.SteamID, p.Appid, p.PersonaName, p.LobbyID, blob)
	return err
}

func (s *Store) SetConnectStr(steamID uint64, connect string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.PresenceOf(steamID)
	if err != nil {
		return err
	}
	p.ConnectStr = connect
	return s.UpsertPresence(p)
}

func (s *Store) UpdatePresence(steamID uint64, name string, appid uint32, lobbyID uint64, rich map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.PresenceOf(steamID)
	if err != nil {
		return err
	}
	p.Appid = appid
	p.LobbyID = lobbyID
	if name != "" {
		p.PersonaName = name
	}
	p.RichPresence = rich
	return s.UpsertPresence(p)
}

func decodeRichPresence(blob []byte) map[string][]byte {
	rich := map[string][]byte{}
	if len(blob) == 0 {
		return rich
	}
	if err := json.Unmarshal(blob, &rich); err != nil {
		return rich
	}
	return rich
}

func (s *Store) CreateLobby(owner uint64, appid, lobbyType, memberLimit uint32) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createLobbyLocked(owner, appid, lobbyType, memberLimit)
}

// createLobbyLocked issues a LobbySteamID from the server's own sequence.
func (s *Store) createLobbyLocked(owner uint64, appid, lobbyType, memberLimit uint32) (uint64, error) {
	id := LobbySteamID(s.nextLobbyAccountID)
	if _, err := s.db.Exec(`INSERT INTO lobbies (lobby_id, owner_id, appid, type, member_limit) VALUES (?, ?, ?, ?, ?)`,
		id, owner, appid, lobbyType, memberLimit); err != nil {
		return 0, err
	}
	s.nextLobbyAccountID++
	if s.nextLobbyAccountID == 0 {
		s.nextLobbyAccountID = 1
	}
	_, err := s.db.Exec(`INSERT INTO lobby_members (lobby_id, steam_id) VALUES (?, ?)`, id, owner)
	return id, err
}

// JoinLobby enforces the lobby's visibility, joinable flag, appid and member
// limit. A repeat join is a no-op so retried client JOIN packets stay harmless.
func (s *Store) JoinLobby(lobbyID, steamID uint64, appid uint32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var joinable int
	var limit, lobbyType, lobbyAppid uint32
	var owner uint64
	err := s.db.QueryRow(`SELECT joinable, member_limit, type, appid, owner_id FROM lobbies WHERE lobby_id = ?`, lobbyID).
		Scan(&joinable, &limit, &lobbyType, &lobbyAppid, &owner)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLobbyNotFound
	}
	if err != nil {
		return err
	}
	var already uint32
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lobby_members WHERE lobby_id = ? AND steam_id = ?`, lobbyID, steamID).Scan(&already); err != nil {
		return err
	}
	if already > 0 {
		return nil
	}
	blocked, err := s.IsBlocked(owner, steamID)
	if err != nil {
		return err
	}
	if blocked {
		return ErrPairBlocked
	}
	var blockedMembers uint32
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lobby_members lm JOIN blocks b ON
		(b.steam_id = lm.steam_id AND b.blocked_id = ?) OR (b.blocked_id = lm.steam_id AND b.steam_id = ?)
		WHERE lm.lobby_id = ?`, steamID, steamID, lobbyID).Scan(&blockedMembers); err != nil {
		return err
	}
	if blockedMembers > 0 {
		return ErrPairBlocked
	}
	if joinable == 0 {
		return ErrLobbyUnavailable
	}
	if appid != 0 && lobbyAppid != 0 && appid != lobbyAppid {
		return ErrLobbyUnavailable
	}
	switch lobbyType {
	case LobbyTypePrivate:
		return ErrLobbyUnavailable // invite-only; invites are not persisted
	case LobbyTypeFriendsOnly:
		var friends uint32
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM friendships WHERE steam_id = ? AND friend_id = ?`, owner, steamID).Scan(&friends); err != nil {
			return err
		}
		if friends == 0 {
			return ErrLobbyUnavailable
		}
	}
	var members uint32
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lobby_members WHERE lobby_id = ?`, lobbyID).Scan(&members); err != nil {
		return err
	}
	if members >= limit {
		return ErrLobbyUnavailable
	}
	_, err = s.db.Exec(`INSERT INTO lobby_members (lobby_id, steam_id) VALUES (?, ?)`, lobbyID, steamID)
	return err
}

// UpsertLobby records the snapshot an owner sends over the game socket so the
// router and the HTTP list see client-created lobbies. Existing snapshots are
// merged by adding members; removals arrive as LEAVE packets.
func (s *Store) UpsertLobby(sender uint64, l Lobby) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	joinable := 0
	if l.Joinable {
		joinable = 1
	}
	var owner uint64
	err := s.db.QueryRow(`SELECT owner_id FROM lobbies WHERE lobby_id = ?`, l.LobbyID).Scan(&owner)
	switch {
	case err == nil:
		if owner != sender || l.Owner != owner {
			return ErrLobbyNotOwner
		}
		_, err = s.db.Exec(`UPDATE lobbies SET appid = ?, type = ?, member_limit = ?, joinable = ? WHERE lobby_id = ?`,
			l.Appid, l.Type, l.MemberLimit, joinable, l.LobbyID)
	case errors.Is(err, sql.ErrNoRows):
		if l.Owner != sender {
			return ErrLobbyNotOwner
		}
		_, err = s.db.Exec(`INSERT INTO lobbies (lobby_id, owner_id, appid, type, member_limit, joinable)
			VALUES (?, ?, ?, ?, ?, ?)`, l.LobbyID, l.Owner, l.Appid, l.Type, l.MemberLimit, joinable)
	}
	if err != nil {
		return err
	}
	for _, id := range l.Members {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO lobby_members (lobby_id, steam_id) VALUES (?, ?)`, l.LobbyID, id); err != nil {
			return err
		}
	}
	return nil
}

// SyncLobby applies an owner's game-socket snapshot. A nonzero AssignRequestID
// claims a new lobby and always gets a server-issued ID; later snapshots must
// carry an ID this server already issued to that owner.
func (s *Store) SyncLobby(sender uint64, l Lobby) (uint64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	created := false
	key := [2]uint64{sender, l.AssignRequestID}
	if l.AssignRequestID != 0 {
		id, ok := s.lobbyRequests[key]
		if !ok {
			var err error
			id, err = s.createLobbyLocked(sender, l.Appid, l.Type, l.MemberLimit)
			if err != nil {
				return 0, false, err
			}
			if s.lobbyRequests == nil {
				s.lobbyRequests = make(map[[2]uint64]uint64)
			}
			s.lobbyRequests[key] = id
			created = true
		}
		l.LobbyID = id
	} else {
		if !IsLobbySteamID(l.LobbyID) {
			return 0, false, ErrLobbyNotAuthoritative
		}
		var owner uint64
		if err := s.db.QueryRow(`SELECT owner_id FROM lobbies WHERE lobby_id = ?`, l.LobbyID).Scan(&owner); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, false, ErrLobbyNotAuthoritative
			}
			return 0, false, err
		}
		if owner != sender {
			return 0, false, ErrLobbyNotOwner
		}
	}

	joinable := 0
	if l.Joinable {
		joinable = 1
	}
	if _, err := s.db.Exec(`UPDATE lobbies SET appid = ?, type = ?, member_limit = ?, joinable = ? WHERE lobby_id = ?`,
		l.Appid, l.Type, l.MemberLimit, joinable, l.LobbyID); err != nil {
		return 0, created, err
	}
	for _, id := range l.Members {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO lobby_members (lobby_id, steam_id) VALUES (?, ?)`, l.LobbyID, id); err != nil {
			return 0, created, err
		}
	}
	if _, err := s.db.Exec(`DELETE FROM lobby_metadata WHERE lobby_id = ?`, l.LobbyID); err != nil {
		return 0, created, err
	}
	for k, v := range l.Values {
		if _, err := s.db.Exec(`INSERT INTO lobby_metadata (lobby_id, key, value) VALUES (?, ?, ?)`, l.LobbyID, k, v); err != nil {
			return 0, created, err
		}
	}
	return l.LobbyID, created, nil
}

// LobbyMetadata returns the persisted key/value data for a lobby.
func (s *Store) LobbyMetadata(lobbyID uint64) (map[string][]byte, error) {
	rows, err := s.db.Query(`SELECT key, value FROM lobby_metadata WHERE lobby_id = ?`, lobbyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[string][]byte)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		values[key] = []byte(value)
	}
	return values, rows.Err()
}

// SearchLobbies returns matching lobbies. Blocked peers are excluded both as
// owner and as members, so a block hides a lobby from discovery and join alike.
func (s *Store) SearchLobbies(q LobbySearch) ([]uint64, error) {
	lobbies, err := s.Lobbies()
	if err != nil {
		return nil, err
	}
	matched := make([]uint64, 0, len(lobbies))
	for _, l := range lobbies {
		if l.Type == LobbyTypePrivate {
			continue
		}
		if q.Appid != 0 && l.Appid != q.Appid {
			continue
		}
		if q.Joinable && !l.Joinable {
			continue
		}
		members := uint32(len(l.Members))
		if q.MinMembers != 0 && members < q.MinMembers {
			continue
		}
		if q.MaxMembers != 0 && members > q.MaxMembers {
			continue
		}
		blocked, err := s.IsBlocked(q.Requester, l.Owner)
		if err != nil {
			return nil, err
		}
		if blocked {
			continue
		}
		hidden := false
		for _, member := range l.Members {
			blocked, err := s.IsBlocked(q.Requester, member)
			if err != nil {
				return nil, err
			}
			if blocked {
				hidden = true
				break
			}
		}
		if hidden {
			continue
		}
		if len(q.Filters) != 0 {
			values, err := s.LobbyMetadata(l.LobbyID)
			if err != nil {
				return nil, err
			}
			if !metadataMatches(values, q.Filters) {
				continue
			}
		}
		matched = append(matched, l.LobbyID)
	}
	return matched, nil
}

func metadataMatches(values map[string][]byte, filters []LobbyFilter) bool {
	for _, f := range filters {
		raw, ok := values[f.Key]
		if !ok {
			return false
		}
		if f.Numeric != nil {
			n, err := strconv.ParseInt(string(raw), 10, 64)
			if err != nil {
				return false
			}
			if !compareInt64(n, *f.Numeric, f.Compare) {
				return false
			}
			continue
		}
		if f.String == nil || !compareString(string(raw), *f.String, f.Compare) {
			return false
		}
	}
	return true
}

func compareString(a, b string, op LobbyComparison) bool {
	switch op {
	case CompareEqual:
		return a == b
	case CompareNotEqual:
		return a != b
	case CompareGreater:
		return a > b
	case CompareGreaterEqual:
		return a >= b
	case CompareLess:
		return a < b
	case CompareLessEqual:
		return a <= b
	}
	return false
}

func compareInt64(a, b int64, op LobbyComparison) bool {
	switch op {
	case CompareEqual:
		return a == b
	case CompareNotEqual:
		return a != b
	case CompareGreater:
		return a > b
	case CompareGreaterEqual:
		return a >= b
	case CompareLess:
		return a < b
	case CompareLessEqual:
		return a <= b
	}
	return false
}

func (s *Store) DeleteLobby(lobbyID, sender uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM lobbies WHERE lobby_id = ? AND owner_id = ?`, lobbyID, sender)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrLobbyNotOwner
	}
	s.dropLobbyRequests(lobbyID)
	return nil
}

func (s *Store) dropLobbyRequests(lobbyID uint64) {
	for key, id := range s.lobbyRequests {
		if id == lobbyID {
			delete(s.lobbyRequests, key)
		}
	}
}

// LeaveLobby removes a member and hands an empty-owner lobby to its lowest
// remaining SteamID, or deletes it when the last member leaves.
func (s *Store) LeaveLobby(lobbyID, steamID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var owner uint64
	if err := s.db.QueryRow(`SELECT owner_id FROM lobbies WHERE lobby_id = ?`, lobbyID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLobbyNotFound
		}
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM lobby_members WHERE lobby_id = ? AND steam_id = ?`, lobbyID, steamID); err != nil {
		return err
	}
	if owner != steamID {
		return nil
	}
	var next sql.NullInt64
	if err := s.db.QueryRow(`SELECT MIN(steam_id) FROM lobby_members WHERE lobby_id = ?`, lobbyID).Scan(&next); err != nil {
		return err
	}
	if !next.Valid {
		_, err := s.db.Exec(`DELETE FROM lobbies WHERE lobby_id = ?`, lobbyID)
		if err == nil {
			s.dropLobbyRequests(lobbyID)
		}
		return err
	}
	_, err := s.db.Exec(`UPDATE lobbies SET owner_id = ? WHERE lobby_id = ?`, uint64(next.Int64), lobbyID)
	return err
}

func (s *Store) ChangeLobbyOwner(lobbyID, sender, newOwner uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var owner uint64
	if err := s.db.QueryRow(`SELECT owner_id FROM lobbies WHERE lobby_id = ?`, lobbyID).Scan(&owner); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrLobbyNotFound
		}
		return err
	}
	if owner != sender {
		return ErrLobbyNotOwner
	}
	var members uint32
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lobby_members WHERE lobby_id = ? AND steam_id = ?`, lobbyID, newOwner).Scan(&members); err != nil {
		return err
	}
	if members == 0 {
		return ErrLobbyUnavailable
	}
	_, err := s.db.Exec(`UPDATE lobbies SET owner_id = ? WHERE lobby_id = ?`, newOwner, lobbyID)
	return err
}

func (s *Store) LobbyMembers(lobbyID uint64) ([]uint64, error) {
	rows, err := s.db.Query(`SELECT steam_id FROM lobby_members WHERE lobby_id = ? ORDER BY steam_id`, lobbyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		members = append(members, id)
	}
	return members, rows.Err()
}

func (s *Store) Lobby(lobbyID uint64) (Lobby, error) {
	var l Lobby
	var joinable int
	err := s.db.QueryRow(`SELECT lobby_id, owner_id, appid, member_limit, type, joinable FROM lobbies WHERE lobby_id = ?`, lobbyID).
		Scan(&l.LobbyID, &l.Owner, &l.Appid, &l.MemberLimit, &l.Type, &joinable)
	if errors.Is(err, sql.ErrNoRows) {
		return Lobby{}, ErrLobbyNotFound
	}
	if err != nil {
		return Lobby{}, err
	}
	l.Joinable = joinable != 0
	l.Members, err = s.LobbyMembers(lobbyID)
	return l, err
}

func (s *Store) Lobbies() ([]Lobby, error) {
	rows, err := s.db.Query(`SELECT lobby_id, owner_id, appid, member_limit, type, joinable FROM lobbies ORDER BY lobby_id`)
	if err != nil {
		return nil, err
	}
	lobbies := []Lobby{}
	for rows.Next() {
		var l Lobby
		var joinable int
		if err := rows.Scan(&l.LobbyID, &l.Owner, &l.Appid, &l.MemberLimit, &l.Type, &joinable); err != nil {
			rows.Close()
			return nil, err
		}
		l.Joinable = joinable != 0
		lobbies = append(lobbies, l)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range lobbies {
		// The single DB connection cannot run a nested query while rows are open.
		lobbies[i].Members, err = s.LobbyMembers(lobbies[i].LobbyID)
		if err != nil {
			return nil, err
		}
	}
	return lobbies, nil
}

type Achievement struct {
	SteamID     uint64 `json:"steam_id"`
	Appid       uint32 `json:"appid"`
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Hidden      bool   `json:"hidden"`
	Icon        string `json:"icon"`
	Achieved    bool   `json:"achieved"`
	UnlockTime  int64  `json:"unlock_time"`
}

type Stat struct {
	Name  string `json:"name"`
	Value []byte `json:"value"`
	Appid uint32 `json:"-"`
}

type UserStats struct {
	SteamID      uint64        `json:"steam_id"`
	Stats        []Stat        `json:"stats"`
	Achievements []Achievement `json:"achievements"`
}

// UnlockAchievement stores the first unlock time; later reports only refresh metadata.
func (s *Store) UnlockAchievement(steamID uint64, a Achievement) (Achievement, error) {
	hidden := 0
	if a.Hidden {
		hidden = 1
	}
	_, err := s.db.Exec(`INSERT INTO achievements
		(steam_id, appid, name, achieved, unlock_time, display_name, description, hidden, icon)
		VALUES (?, ?, ?, 1, CAST(strftime('%s', 'now') AS INTEGER), ?, ?, ?, ?)
		ON CONFLICT(steam_id, appid, name) DO UPDATE SET achieved = 1,
		display_name = excluded.display_name, description = excluded.description,
		hidden = excluded.hidden, icon = excluded.icon`,
		steamID, a.Appid, a.ID, a.DisplayName, a.Description, hidden, a.Icon)
	if err != nil {
		return Achievement{}, err
	}
	return s.AchievementOf(steamID, a.Appid, a.ID)
}

func (s *Store) AchievementOf(steamID uint64, appid uint32, name string) (Achievement, error) {
	var a Achievement
	var achieved, hidden int
	err := s.db.QueryRow(`SELECT steam_id, appid, name, achieved, unlock_time,
		display_name, description, hidden, icon
		FROM achievements WHERE steam_id = ? AND appid = ? AND name = ?`, steamID, appid, name).
		Scan(&a.SteamID, &a.Appid, &a.ID, &achieved, &a.UnlockTime,
			&a.DisplayName, &a.Description, &hidden, &a.Icon)
	if err != nil {
		return Achievement{}, err
	}
	a.Achieved = achieved != 0
	a.Hidden = hidden != 0
	return a, nil
}

func (s *Store) StatsOf(steamID uint64) (UserStats, error) {
	result := UserStats{SteamID: steamID, Stats: []Stat{}, Achievements: []Achievement{}}
	rows, err := s.db.Query(`SELECT name, value, appid FROM stats WHERE steam_id = ? ORDER BY name`, steamID)
	if err != nil {
		return UserStats{}, err
	}
	for rows.Next() {
		var stat Stat
		var value []byte
		if err := rows.Scan(&stat.Name, &value, &stat.Appid); err != nil {
			rows.Close()
			return UserStats{}, err
		}
		stat.Value = append([]byte(nil), value...)
		result.Stats = append(result.Stats, stat)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return UserStats{}, err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT steam_id, appid, name, achieved, unlock_time,
		display_name, description, hidden, icon FROM achievements
		WHERE steam_id = ? ORDER BY appid, name`, steamID)
	if err != nil {
		return UserStats{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Achievement
		var achieved, hidden int
		if err := rows.Scan(&a.SteamID, &a.Appid, &a.ID, &achieved, &a.UnlockTime,
			&a.DisplayName, &a.Description, &hidden, &a.Icon); err != nil {
			return UserStats{}, err
		}
		a.Achieved = achieved != 0
		a.Hidden = hidden != 0
		result.Achievements = append(result.Achievements, a)
	}
	return result, rows.Err()
}

func (s *Store) LobbiesOf(steamID uint64) ([]uint64, error) {
	rows, err := s.db.Query(`SELECT lobby_id FROM lobby_members WHERE steam_id = ? ORDER BY lobby_id`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uint64{}
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, ErrLobbyNotFound)
}
