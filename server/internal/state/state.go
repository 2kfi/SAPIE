// Package state owns the SQLite store and server-assigned SteamIDs.
package state

import (
	"database/sql"
	"encoding/json"
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

func (s *Store) ResolveSession(tokenHash string) (uint64, error) {
	var steamID uint64
	err := s.db.QueryRow(
		`SELECT steam_id FROM sessions WHERE token_hash = ? AND expires_at > CAST(strftime('%s', 'now') AS INTEGER)`,
		tokenHash,
	).Scan(&steamID)
	return steamID, err
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
}

var (
	ErrLobbyNotFound    = errors.New("lobby not found")
	ErrLobbyUnavailable = errors.New("lobby full or not joinable")
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
	rows, err := s.db.Query(`SELECT friend_id FROM friendships WHERE steam_id = ? ORDER BY friend_id`, steamID)
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
		WHERE f.steam_id = ? ORDER BY f.friend_id`, steamID)
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
	p, err := s.PresenceOf(steamID)
	if err != nil {
		return err
	}
	p.ConnectStr = connect
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
	var id uint64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(lobby_id), 0) + 1 FROM lobbies`).Scan(&id); err != nil {
		return 0, err
	}
	if _, err := s.db.Exec(`INSERT INTO lobbies (lobby_id, owner_id, appid, type, member_limit) VALUES (?, ?, ?, ?, ?)`,
		id, owner, appid, lobbyType, memberLimit); err != nil {
		return 0, err
	}
	_, err := s.db.Exec(`INSERT INTO lobby_members (lobby_id, steam_id) VALUES (?, ?)`, id, owner)
	return id, err
}

func (s *Store) JoinLobby(lobbyID, steamID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var joinable int
	var limit uint32
	err := s.db.QueryRow(`SELECT joinable, member_limit FROM lobbies WHERE lobby_id = ?`, lobbyID).Scan(&joinable, &limit)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLobbyNotFound
	}
	if err != nil {
		return err
	}
	if joinable == 0 {
		return ErrLobbyUnavailable
	}
	var members uint32
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM lobby_members WHERE lobby_id = ?`, lobbyID).Scan(&members); err != nil {
		return err
	}
	if members >= limit {
		return ErrLobbyUnavailable
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO lobby_members (lobby_id, steam_id) VALUES (?, ?)`, lobbyID, steamID)
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
	rows, err := s.db.Query(`SELECT name, value FROM stats WHERE steam_id = ? ORDER BY name`, steamID)
	if err != nil {
		return UserStats{}, err
	}
	for rows.Next() {
		var stat Stat
		var value []byte
		if err := rows.Scan(&stat.Name, &value); err != nil {
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
