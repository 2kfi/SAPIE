package state

import (
	"database/sql"
	"errors"
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
