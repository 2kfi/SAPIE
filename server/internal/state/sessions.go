package state

import "time"

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
