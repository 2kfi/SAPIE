// Package state owns the SQLite store and server-assigned SteamIDs.
package state

import (
	"database/sql"
	"errors"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/sapie/sapie/server/migrations"
)

const SteamIDBase uint64 = 76561197960265728

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
	db     *sql.DB
	mu     sync.Mutex
	nextID uint64
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
	if _, err := db.Exec(migrations.Schema); err != nil {
		db.Close()
		return nil, err
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
	return &Store{db: db, nextID: next}, nil
}

func (s *Store) Close() error { return s.db.Close() }

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

func IsNotFound(err error) bool { return errors.Is(err, sql.ErrNoRows) }
