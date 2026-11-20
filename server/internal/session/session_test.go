package session

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/state"
)

func TestLoginHandshake(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateAccount("alice", hash, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := New(store, auth.NewLimiter(nil))
	go srv.Serve(ctx, ln)

	// A non-login first frame closes the connection.
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Ping{Ping: true}}); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := protocol.ReadFrame(c); err == io.EOF {
		// expected
	} else if err != nil {
		t.Fatalf("got %v, want EOF", err)
	} else {
		t.Fatal("server accepted a non-login first frame")
	}
	c.Close()

	// Bad credentials, then a successful login.
	c, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	login := &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "wrong"}}}
	if err := protocol.WriteFrame(c, login); err != nil {
		t.Fatal(err)
	}
	response := readLoginResponse(t, c)
	if response.Status != protocol.LoginResponse_BAD_CREDENTIALS {
		t.Fatalf("status = %v, want BAD_CREDENTIALS", response.Status)
	}

	c.Close()

	c, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "pw"}}}); err != nil {
		t.Fatal(err)
	}
	response = readLoginResponse(t, c)
	if response.Status != protocol.LoginResponse_OK || response.SteamId != id || len(response.SessionToken) == 0 {
		t.Fatalf("bad login response: %v", response)
	}
	if string(response.PersonaName) != "Alice" {
		t.Fatalf("persona = %q", response.PersonaName)
	}
	if peer, ok := srv.PeerFor(id); !ok || peer == nil {
		t.Fatal("authenticated session not registered")
	}
}

func TestTicketRequest(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAccount("alice", hash, "Alice"); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := New(store, auth.NewLimiter(nil))
	go srv.Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	login := &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "pw"}}}
	if err := protocol.WriteFrame(c, login); err != nil {
		t.Fatal(err)
	}
	readLoginResponse(t, c)

	var previous []byte
	for want := uint32(1); want <= 2; want++ {
		req := &protocol.Frame{Payload: &protocol.Frame_TicketRequest{TicketRequest: &protocol.TicketRequest{Appid: 480}}}
		if err := protocol.WriteFrame(c, req); err != nil {
			t.Fatal(err)
		}
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		frame, err := protocol.ReadFrame(c)
		if err != nil {
			t.Fatal(err)
		}
		result, ok := frame.Payload.(*protocol.Frame_TicketResponse)
		if !ok {
			t.Fatalf("got %T, want TicketResponse", frame.Payload)
		}
		ticket := result.TicketResponse.GetTicket()
		if len(ticket) != 32 {
			t.Fatalf("ticket length = %d, want 32", len(ticket))
		}
		if got := result.TicketResponse.GetNumber(); got != want {
			t.Fatalf("ticket number = %d, want %d", got, want)
		}
		if bytes.Equal(ticket, previous) {
			t.Fatal("repeated ticket bytes")
		}
		previous = ticket
	}
}

func testStore(t *testing.T) (*state.Store, uint64, uint64) {
	t.Helper()
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	alice, err := store.CreateAccount("alice", "h1", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateAccount("bob", "h2", "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddFriend(alice, bob); err != nil {
		t.Fatal(err)
	}
	return store, alice, bob
}

func TestLoginSnapshotsPersonaState(t *testing.T) {
	store, alice, bob := testStore(t)
	carol, err := store.CreateAccount("carol", "h3", "Carol")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddFriend(alice, carol); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPresence(state.Presence{SteamID: bob, RichPresence: map[string][]byte{"map": []byte("de_dust2")}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetConnectStr(bob, "connect=127.0.0.1:27015"); err != nil {
		t.Fatal(err)
	}
	srv := New(store, nil)
	srv.peers[bob] = &Session{steamID: bob, personaName: "Bob"}

	snapshots := srv.friendSnapshots(alice)
	if len(snapshots) != 2 {
		t.Fatalf("got %d snapshots: %+v", len(snapshots), snapshots)
	}
	byID := map[uint64]*protocol.FriendSnapshot{}
	for _, snapshot := range snapshots {
		byID[snapshot.GetSteamId()] = snapshot
	}
	bobSnapshot, carolSnapshot := byID[bob], byID[carol]
	if bobSnapshot == nil || carolSnapshot == nil {
		t.Fatalf("missing snapshots: %+v", byID)
	}
	if got := string(bobSnapshot.RichPresence[state.PersonaStateKey]); got != "1" {
		t.Fatalf("online persona state = %q, want 1", got)
	}
	if got := string(carolSnapshot.RichPresence[state.PersonaStateKey]); got != "0" {
		t.Fatalf("offline persona state = %q, want 0", got)
	}
	if got := string(bobSnapshot.RichPresence["map"]); got != "de_dust2" {
		t.Fatalf("rich presence lost: %q", got)
	}
	if _, ok := bobSnapshot.RichPresence[state.ConnectStrKey]; ok {
		t.Fatal("connect_str leaked into login snapshot")
	}
}

func TestDisconnectSkipsStaleOfflinePush(t *testing.T) {
	store, alice, bob := testStore(t)
	srv := New(store, nil)
	observerConn, readerConn := net.Pipe()
	defer observerConn.Close()
	defer readerConn.Close()
	observer := &Session{steamID: bob, conn: observerConn}
	srv.peers[bob] = observer
	frames := make(chan *protocol.Frame, 4)
	go func() {
		for {
			frame, err := protocol.ReadFrame(readerConn)
			if err != nil {
				return
			}
			frames <- frame
		}
	}()

	old := &Session{steamID: alice, personaName: "Alice"}
	replacement := &Session{steamID: alice, personaName: "Alice"}
	srv.peers[alice] = replacement
	srv.disconnect(old)
	select {
	case frame := <-frames:
		t.Fatalf("stale offline push after replacement: %+v", frame.Payload)
	default:
	}

	srv.peers[alice] = old
	srv.disconnect(old)
	select {
	case frame := <-frames:
		friend := steamMessage(frame)
		if friend == nil || friend.GetFriend() == nil || friend.GetSourceId() != alice ||
			string(friend.GetFriend().GetRichPresence()[state.PersonaStateKey]) != "0" {
			t.Fatalf("bad offline push: %+v", frame.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("real disconnect did not push offline presence")
	}
}

func steamMessage(frame *protocol.Frame) *protocol.Common_Message {
	steam, ok := frame.Payload.(*protocol.Frame_Steam)
	if !ok {
		return nil
	}
	return steam.Steam
}

func TestStatsGetPrivacyAndAppid(t *testing.T) {
	store, alice, bob := testStore(t)
	for _, a := range []state.Achievement{
		{Appid: 480, ID: "public_480"},
		{Appid: 480, ID: "secret_480", Hidden: true},
		{Appid: 900, ID: "public_900"},
	} {
		if _, err := store.UnlockAchievement(alice, a); err != nil {
			t.Fatal(err)
		}
	}

	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	srv := New(store, nil)
	session := &Session{steamID: bob, conn: serverConn}
	request := func(appid uint32, target uint64) *protocol.StatsResponse {
		t.Helper()
		done := make(chan struct{})
		go func() {
			srv.handleStatsGet(session, &protocol.StatsGet{Appid: appid, SteamId: target})
			close(done)
		}()
		clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		frame, err := protocol.ReadFrame(clientConn)
		if err != nil {
			t.Fatal(err)
		}
		<-done
		response, ok := frame.Payload.(*protocol.Frame_StatsResponse)
		if !ok {
			t.Fatalf("got %T, want StatsResponse", frame.Payload)
		}
		return response.StatsResponse
	}
	names := func(resp *protocol.StatsResponse) map[string]bool {
		got := map[string]bool{}
		for _, a := range resp.GetAchievements() {
			got[a.GetName()] = true
		}
		return got
	}
	want := func(resp *protocol.StatsResponse, expected ...string) {
		t.Helper()
		got := names(resp)
		if len(got) != len(expected) {
			t.Fatalf("achievements = %v, want %v", got, expected)
		}
		for _, name := range expected {
			if !got[name] {
				t.Fatalf("achievements = %v, want %v", got, expected)
			}
		}
	}

	want(request(0, alice), "public_480", "public_900")
	want(request(900, alice), "public_900")
	session.steamID = alice
	want(request(0, 0), "public_480", "secret_480", "public_900")
	want(request(480, 0), "public_480", "secret_480")
}

func readLoginResponse(t *testing.T, c net.Conn) *protocol.LoginResponse {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := protocol.ReadFrame(c)
	if err != nil {
		t.Fatal(err)
	}
	response, ok := frame.Payload.(*protocol.Frame_LoginResponse)
	if !ok {
		t.Fatalf("got %T, want LoginResponse", frame.Payload)
	}
	return response.LoginResponse
}
