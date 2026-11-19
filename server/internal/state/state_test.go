package state

import (
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAccounts(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ids := make([]uint64, 0, 2)
	for _, name := range []string{"alice", "bob"} {
		id, err := store.CreateAccount(name, "hashed-"+name, name)
		if err != nil {
			t.Fatal(err)
		}
		if id <= SteamIDBase {
			t.Fatalf("steam id %d not above base %d", id, SteamIDBase)
		}
		ids = append(ids, id)
	}
	if ids[0] == ids[1] {
		t.Fatalf("duplicate steam id %d", ids[0])
	}

	accounts, err := store.ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 {
		t.Fatalf("got %d accounts, want 2", len(accounts))
	}
	for _, a := range accounts {
		if strings.Contains(a.Username, "hashed") {
			t.Fatal("password hash leaked into public account")
		}
	}

	auth, err := store.AccountByUsername("alice")
	if err != nil {
		t.Fatal(err)
	}
	if auth.PasswordHash != "hashed-alice" || auth.SteamID != ids[0] {
		t.Fatalf("unexpected auth account: %+v", auth)
	}

	if err := store.CreateSession(ids[0], "token-hash", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAccount(ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteAccount(ids[0]); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("got %v, want sql.ErrNoRows", err)
	}
	var sessions int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions); err != nil {
		t.Fatal(err)
	}
	if sessions != 0 {
		t.Fatalf("session survived account deletion: %d", sessions)
	}
}

func TestAchievementStore(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	alice, err := store.CreateAccount("alice", "h1", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateAccount("bob", "h2", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	want := Achievement{
		Appid: 480, ID: "first_kill", DisplayName: "First Kill",
		Description: "Kill one opponent", Hidden: true, Icon: "first.png",
	}
	first, err := store.UnlockAchievement(alice, want)
	if err != nil {
		t.Fatal(err)
	}
	if first.UnlockTime <= 0 || !first.Achieved || first.DisplayName != want.DisplayName ||
		first.Description != want.Description || !first.Hidden || first.Icon != want.Icon {
		t.Fatalf("stored unlock = %+v", first)
	}

	if _, err := store.db.Exec(`UPDATE achievements SET unlock_time = 1000 WHERE steam_id = ?`, alice); err != nil {
		t.Fatal(err)
	}
	changed := want
	changed.DisplayName = "Renamed"
	changed.UnlockTime = 9999
	again, err := store.UnlockAchievement(alice, changed)
	if err != nil {
		t.Fatal(err)
	}
	if again.UnlockTime != 1000 || again.DisplayName != "Renamed" {
		t.Fatalf("repeat unlock = %+v", again)
	}

	stats, err := store.StatsOf(alice)
	if err != nil {
		t.Fatal(err)
	}
	if stats.SteamID != alice || len(stats.Achievements) != 1 ||
		stats.Achievements[0].UnlockTime != 1000 || stats.Achievements[0].ID != "first_kill" {
		t.Fatalf("stats = %+v", stats)
	}
	other, err := store.StatsOf(bob)
	if err != nil || len(other.Achievements) != 0 {
		t.Fatalf("bob stats = %+v, %v", other, err)
	}

	lobbyID, err := store.CreateLobby(alice, 480, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.JoinLobby(lobbyID, bob); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint64{alice, bob} {
		lobbies, err := store.LobbiesOf(id)
		if err != nil || len(lobbies) != 1 || lobbies[0] != lobbyID {
			t.Fatalf("lobbies of %d = %v, %v", id, lobbies, err)
		}
	}
}

func TestLegacyAchievementSchemaUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`CREATE TABLE accounts (steam_id INTEGER PRIMARY KEY, username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL, persona_name TEXT NOT NULL, created_at INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE achievements (steam_id INTEGER NOT NULL REFERENCES accounts(steam_id),
		appid INTEGER NOT NULL, name TEXT NOT NULL, achieved INTEGER NOT NULL DEFAULT 0,
		unlock_time INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (steam_id, appid, name));`)
	if err != nil {
		t.Fatal(err)
	}
	legacy.Close()

	for i := 0; i < 2; i++ {
		store, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		accountID := uint64(0)
		if i == 0 {
			accountID, err = store.CreateAccount("alice", "h", "Alice")
		} else {
			var account *AuthAccount
			account, err = store.AccountByUsername("alice")
			if err == nil {
				accountID = account.SteamID
			}
		}
		if err != nil {
			t.Fatalf("account on open %d: %v", i, err)
		}
		unlock, err := store.UnlockAchievement(accountID, Achievement{Appid: 480, ID: "first_kill"})
		if err != nil || unlock.UnlockTime <= 0 {
			t.Fatalf("unlock on legacy schema = %+v, %v", unlock, err)
		}
		store.Close()
	}
}

func TestSocialStore(t *testing.T) {
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	alice, err := store.CreateAccount("alice", "h1", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateAccount("bob", "h2", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	carol, err := store.CreateAccount("carol", "h3", "Carol")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddFriend(alice, bob); err != nil {
		t.Fatal(err)
	}
	friends, err := store.FriendsOf(alice)
	if err != nil || len(friends) != 1 || friends[0] != bob {
		t.Fatalf("friends of alice = %v, %v", friends, err)
	}
	friends, err = store.FriendsOf(bob)
	if err != nil || len(friends) != 1 || friends[0] != alice {
		t.Fatalf("friends of bob = %v, %v", friends, err)
	}

	err = store.UpsertPresence(Presence{
		SteamID: bob, PersonaName: "Bob", Appid: 480, LobbyID: 42,
		RichPresence: map[string][]byte{"map": []byte("de_dust2")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetConnectStr(bob, "connect=127.0.0.1:27015"); err != nil {
		t.Fatal(err)
	}
	roster, err := store.FriendsPresence(alice)
	if err != nil || len(roster) != 1 {
		t.Fatalf("roster = %v, %v", roster, err)
	}
	friend := roster[0]
	if friend.SteamID != bob || friend.PersonaName != "Bob" || friend.Appid != 480 ||
		friend.LobbyID != 42 || friend.ConnectStr != "connect=127.0.0.1:27015" ||
		string(friend.RichPresence["map"]) != "de_dust2" {
		t.Fatalf("unexpected friend presence: %+v", friend)
	}
	if _, ok := friend.RichPresence[ConnectStrKey]; ok {
		t.Fatal("reserved connect_str leaked into rich presence")
	}

	if err := store.CreateSession(alice, "hash-alice", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	id, err := store.ResolveSession("hash-alice")
	if err != nil || id != alice {
		t.Fatalf("resolve session = %d, %v", id, err)
	}
	if _, err := store.ResolveSession("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("resolve missing = %v", err)
	}

	lobbyID, err := store.CreateLobby(alice, 480, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.JoinLobby(lobbyID, bob); err != nil {
		t.Fatal(err)
	}
	lobby, err := store.Lobby(lobbyID)
	if err != nil || !lobby.Joinable || lobby.Owner != alice {
		t.Fatalf("lobby = %+v, %v", lobby, err)
	}
	if len(lobby.Members) != 2 || lobby.Members[0] != alice || lobby.Members[1] != bob {
		t.Fatalf("lobby members = %v", lobby.Members)
	}
	if err := store.JoinLobby(lobbyID, carol); !errors.Is(err, ErrLobbyUnavailable) {
		t.Fatalf("over-limit join = %v", err)
	}
	if err := store.JoinLobby(999, carol); !errors.Is(err, ErrLobbyNotFound) {
		t.Fatalf("missing lobby join = %v", err)
	}
	lobbies, err := store.Lobbies()
	if err != nil || len(lobbies) != 1 || len(lobbies[0].Members) != 2 {
		t.Fatalf("lobbies = %+v, %v", lobbies, err)
	}
}
