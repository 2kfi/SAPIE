package state

import (
	"database/sql"
	"time"
)

const sessionIDPrefixLen = 12

// SessionInfo is the safe public view of a stored session. ID is a prefix of
// the token hash, never the raw token or the full hash.
type SessionInfo struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`
	ExpiresAt int64  `json:"expires_at"`
	Current   bool   `json:"current"`
}

func SessionID(tokenHash string) string {
	if len(tokenHash) < sessionIDPrefixLen {
		return tokenHash
	}
	return tokenHash[:sessionIDPrefixLen]
}

func (s *Store) SessionsOf(steamID uint64, currentHash string) ([]SessionInfo, error) {
	rows, err := s.db.Query(`SELECT token_hash, created_at, expires_at FROM sessions
		WHERE steam_id = ? AND expires_at > CAST(strftime('%s', 'now') AS INTEGER)
		ORDER BY created_at, token_hash`, steamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sessions := []SessionInfo{}
	for rows.Next() {
		var hash string
		var info SessionInfo
		if err := rows.Scan(&hash, &info.CreatedAt, &info.ExpiresAt); err != nil {
			return nil, err
		}
		info.ID = SessionID(hash)
		info.Current = hash == currentHash
		sessions = append(sessions, info)
	}
	return sessions, rows.Err()
}

func (s *Store) RevokeSession(steamID uint64, id string) (bool, error) {
	if len(id) != sessionIDPrefixLen {
		return false, nil
	}
	res, err := s.db.Exec(`DELETE FROM sessions WHERE steam_id = ? AND substr(token_hash, 1, ?) = ?`, steamID, sessionIDPrefixLen, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) RevokeAllSessions(steamID uint64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE steam_id = ?`, steamID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) RevokeSessionByHash(tokenHash string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteExpiredSessions() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= CAST(strftime('%s', 'now') AS INTEGER)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteExpiredSessionsFor keeps repeated logins from accumulating dead rows
// for the same account between global cleanup ticks.
func (s *Store) DeleteExpiredSessionsFor(steamID uint64, now time.Time) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE steam_id = ? AND expires_at <= ?`, steamID, now.Unix())
	return err
}

// RotateSession consumes oldHash and issues newHash in one transaction. The
// new expiry never exceeds capExpiresAt, so a chain of rotations cannot extend
// a session past the lifetime granted at the last password login.
func (s *Store) RotateSession(oldHash, newHash string, capExpiresAt time.Time) (uint64, int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	var steamID uint64
	var expiresAt int64
	err = tx.QueryRow(`SELECT steam_id, expires_at FROM sessions
		WHERE token_hash = ? AND expires_at > CAST(strftime('%s', 'now') AS INTEGER)`, oldHash).
		Scan(&steamID, &expiresAt)
	if err != nil {
		return 0, 0, err
	}
	if cap := capExpiresAt.Unix(); expiresAt > cap {
		expiresAt = cap
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE token_hash = ?`, oldHash); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(`INSERT INTO sessions (token_hash, steam_id, expires_at) VALUES (?, ?, ?)`,
		newHash, steamID, expiresAt); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	return steamID, expiresAt, nil
}

func (s *Store) PasswordHashOf(steamID uint64) (string, error) {
	var hash string
	err := s.db.QueryRow(`SELECT password_hash FROM accounts WHERE steam_id = ?`, steamID).Scan(&hash)
	return hash, err
}

// ChangePassword writes the new hash and kills every session except the one
// that made the change, in one transaction.
func (s *Store) ChangePassword(steamID uint64, currentHash, newHash string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE accounts SET password_hash = ? WHERE steam_id = ?`, newHash, steamID)
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
	if _, err := tx.Exec(`DELETE FROM sessions WHERE steam_id = ? AND token_hash <> ?`, steamID, currentHash); err != nil {
		return err
	}
	return tx.Commit()
}
