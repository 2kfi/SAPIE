package state

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sapie/sapie/server/migrations"
)

func TestMigration002FreshAndLegacy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migrations.Files[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO accounts (steam_id, username, password_hash, persona_name)
		VALUES (?, 'alice', 'hash', 'Alice')`, SteamIDBase+1); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		store, err := Open(path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		for _, table := range []string{"friend_requests", "blocks"} {
			var name string
			if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
				t.Fatalf("open %d: table %s: %v", i, table, err)
			}
		}
		var indexes int
		if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name IN
			('idx_friend_requests_from', 'idx_friend_requests_to', 'idx_blocks_blocked')`).Scan(&indexes); err != nil || indexes != 3 {
			t.Fatalf("open %d: indexes = %d, want 3 (%v)", i, indexes, err)
		}
		account, err := store.AccountBySteamID(SteamIDBase + 1)
		if err != nil || account.PersonaName != "Alice" {
			t.Fatalf("open %d: legacy account lost: %+v %v", i, account, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func socialStore(t *testing.T) (*Store, uint64, uint64, uint64) {
	t.Helper()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	ids := make([]uint64, 3)
	for i, name := range []string{"alice", "bob", "carol"} {
		ids[i], err = store.CreateAccount(name, "hash-"+name, name)
		if err != nil {
			t.Fatal(err)
		}
	}
	return store, ids[0], ids[1], ids[2]
}

func TestFriendRequestLifecycle(t *testing.T) {
	store, alice, bob, carol := socialStore(t)

	req, err := store.SendFriendRequest(alice, bob, "hello")
	if err != nil || req.Status != "pending" || req.FromSteamID != alice || req.ToSteamID != bob {
		t.Fatalf("send = %+v, %v", req, err)
	}
	again, err := store.SendFriendRequest(alice, bob, "again")
	if err != nil || again.ID != req.ID {
		t.Fatalf("duplicate send = %+v, %v", again, err)
	}
	if _, err := store.SendFriendRequest(alice, alice, ""); !errors.Is(err, ErrSelfRequest) {
		t.Fatalf("self request = %v", err)
	}
	if err := store.BlockUser(carol, alice); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SendFriendRequest(alice, carol, ""); !errors.Is(err, ErrPairBlocked) {
		t.Fatalf("blocked outgoing = %v", err)
	}
	if _, err := store.SendFriendRequest(carol, alice, ""); !errors.Is(err, ErrPairBlocked) {
		t.Fatalf("blocked incoming = %v", err)
	}
	if err := store.UnblockUser(carol, alice); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptFriendRequest(req.ID, carol); !errors.Is(err, ErrNotRecipient) {
		t.Fatalf("accept by stranger = %v", err)
	}
	if err := store.AcceptFriendRequest(req.ID, bob); err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptFriendRequest(req.ID, bob); err != nil {
		t.Fatalf("repeat accept = %v", err)
	}
	aliceFriends, err := store.FriendsOf(alice)
	if err != nil || len(aliceFriends) != 1 || aliceFriends[0] != bob {
		t.Fatalf("alice friends = %v, %v", aliceFriends, err)
	}
	bobFriends, err := store.FriendsOf(bob)
	if err != nil || len(bobFriends) != 1 || bobFriends[0] != alice {
		t.Fatalf("bob friends = %v, %v", bobFriends, err)
	}
	if _, err := store.SendFriendRequest(alice, bob, ""); !errors.Is(err, ErrAlreadyFriends) {
		t.Fatalf("request to friend = %v", err)
	}
}

func TestFriendRequestDeclineCancelAndRemove(t *testing.T) {
	store, alice, bob, carol := socialStore(t)

	declined, err := store.SendFriendRequest(alice, carol, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.DeclineFriendRequest(declined.ID, alice); !errors.Is(err, ErrRequestInvalid) {
		t.Fatalf("decline by sender = %v", err)
	}
	if err := store.DeclineFriendRequest(declined.ID, carol); err != nil {
		t.Fatal(err)
	}
	if err := store.DeclineFriendRequest(declined.ID, carol); !errors.Is(err, ErrRequestInvalid) {
		t.Fatalf("repeat decline = %v", err)
	}
	reopened, err := store.SendFriendRequest(alice, carol, "again")
	if err != nil || reopened.ID != declined.ID || reopened.Status != "pending" || reopened.Message != "again" {
		t.Fatalf("reopen declined request = %+v, %v", reopened, err)
	}
	if err := store.CancelFriendRequest(reopened.ID, alice); err != nil {
		t.Fatal(err)
	}

	if err := store.AddFriend(alice, bob); err != nil {
		t.Fatal(err)
	}
	pending, err := store.SendFriendRequest(carol, bob, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CancelFriendRequest(pending.ID, bob); !errors.Is(err, ErrNotSender) {
		t.Fatalf("cancel by recipient = %v", err)
	}
	if err := store.RemoveFriend(alice, bob); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveFriend(alice, bob); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("repeat remove = %v", err)
	}
	requests, err := store.FriendRequestsOf(bob)
	if err != nil || len(requests.Incoming) != 1 || requests.Incoming[0].ID != pending.ID {
		t.Fatalf("remove changed unrelated invites: %+v, %v", requests, err)
	}
	if err := store.CancelFriendRequest(pending.ID, carol); err != nil {
		t.Fatal(err)
	}
	requests, err = store.FriendRequestsOf(bob)
	if err != nil || len(requests.Incoming) != 0 {
		t.Fatalf("cancel left incoming invites: %+v, %v", requests, err)
	}
}

func TestBlockHidesPresenceAndSearch(t *testing.T) {
	store, alice, bob, _ := socialStore(t)
	if err := store.AddFriend(alice, bob); err != nil {
		t.Fatal(err)
	}
	if err := store.BlockUser(alice, bob); err != nil {
		t.Fatal(err)
	}
	if err := store.BlockUser(alice, alice); !errors.Is(err, ErrSelfRequest) {
		t.Fatalf("self block = %v", err)
	}
	blocked, err := store.IsBlocked(bob, alice)
	if err != nil || !blocked {
		t.Fatalf("reverse block check = %v, %v", blocked, err)
	}
	friends, err := store.FriendsOf(alice)
	if err != nil || len(friends) != 0 {
		t.Fatalf("alice still sees blocked friend: %v, %v", friends, err)
	}
	presence, err := store.FriendsPresence(bob)
	if err != nil || len(presence) != 0 {
		t.Fatalf("bob still sees blocked friend: %v, %v", presence, err)
	}
	list, err := store.BlockedUsers(alice)
	if err != nil || len(list) != 1 || list[0].SteamID != bob {
		t.Fatalf("blocked list = %+v, %v", list, err)
	}
	found, err := store.SearchUsers(alice, "bo")
	if err != nil || len(found) != 1 || found[0].SteamID != bob {
		t.Fatalf("search = %+v, %v", found, err)
	}
	if _, err := store.SearchUsers(alice, "b"); !errors.Is(err, ErrSearchQuery) {
		t.Fatalf("short query = %v", err)
	}
	self, err := store.SearchUsers(alice, "ali")
	if err != nil || len(self) != 0 {
		t.Fatalf("search returned self: %+v, %v", self, err)
	}
	if err := store.UnblockUser(alice, bob); err != nil {
		t.Fatal(err)
	}
	friends, err = store.FriendsOf(alice)
	if err != nil || len(friends) != 1 {
		t.Fatalf("unblock did not restore friend: %v, %v", friends, err)
	}
}

func TestSessionRevokeAndCleanup(t *testing.T) {
	store, alice, bob, _ := socialStore(t)
	now := time.Now()
	for _, tc := range []struct {
		name   string
		owner  uint64
		expiry time.Time
	}{
		{"alice-live", alice, now.Add(time.Hour)},
		{"alice-expired", alice, now.Add(-time.Hour)},
		{"bob-live", bob, now.Add(time.Hour)},
	} {
		if err := store.CreateSession(tc.owner, "hash-"+tc.name, tc.expiry); err != nil {
			t.Fatal(err)
		}
	}
	sessions, err := store.SessionsOf(alice, "hash-alice-live")
	if err != nil || len(sessions) != 1 || !sessions[0].Current || len(sessions[0].ID) != sessionIDPrefixLen {
		t.Fatalf("sessions = %+v, %v", sessions, err)
	}
	if err := store.DeleteExpiredSessionsFor(alice, now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveSession("hash-alice-expired"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired session resolved: %v", err)
	}
	var expiredRows int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = 'hash-alice-expired'`).Scan(&expiredRows); err != nil || expiredRows != 0 {
		t.Fatalf("expired row not deleted: %d, %v", expiredRows, err)
	}
	id := sessions[0].ID
	if ok, err := store.RevokeSession(bob, id); err != nil || ok {
		t.Fatalf("cross-account revoke = %v, %v", ok, err)
	}
	if ok, err := store.RevokeSession(alice, id); err != nil || !ok {
		t.Fatalf("owner revoke = %v, %v", ok, err)
	}
	if _, err := store.ResolveSession("hash-alice-live"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("revoked token still resolves: %v", err)
	}
	if _, err := store.RevokeAllSessions(bob); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ResolveSession("hash-bob-live"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("logout-all left a session: %v", err)
	}
}
