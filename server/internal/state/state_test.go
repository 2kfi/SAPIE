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
