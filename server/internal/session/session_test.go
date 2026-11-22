package session

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"io"
	"net"
	"path/filepath"
	"strings"
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

func TestGameLoginWithSessionToken(t *testing.T) {
	const validToken = "m16-valid-session-token"
	cases := []struct {
		name       string
		token      string
		expires    time.Duration
		wantOK     bool
		withStored bool
	}{
		{name: "valid token", token: validToken, expires: time.Hour, wantOK: true, withStored: true},
		{name: "wrong token", token: "m16-wrong-token", withStored: true},
		{name: "expired token", token: validToken, expires: -time.Hour, withStored: true},
		{name: "no token or password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			if tc.withStored {
				if err := store.CreateSession(id, auth.HashToken(validToken), time.Now().Add(tc.expires)); err != nil {
					t.Fatal(err)
				}
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
			login := &protocol.LoginRequest{Username: "alice"}
			if tc.name == "valid token" {
				// The token must bind identity even when the username is wrong.
				login.Username = "not-alice"
				login.Password = "wrong"
			}
			if tc.token != "" {
				login.SessionToken = tc.token
			}
			if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: login}}); err != nil {
				t.Fatal(err)
			}
			response := readLoginResponse(t, c)
			if tc.wantOK {
				if response.Status != protocol.LoginResponse_OK || response.SteamId != id ||
					string(response.PersonaName) != "Alice" || len(response.SessionToken) == 0 {
					t.Fatalf("bad token login response: %v", response)
				}
				if peer, ok := srv.PeerFor(id); !ok || peer == nil {
					t.Fatal("token-authenticated session not registered")
				}
			} else if response.Status != protocol.LoginResponse_BAD_CREDENTIALS {
				t.Fatalf("status = %v, want BAD_CREDENTIALS", response.Status)
			}
		})
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
	aliceID, err := store.CreateAccount("alice", hash, "Alice")
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

		valid, err := store.VerifyTicket(ticket, aliceID, 480, want)
		if err != nil || !valid {
			t.Fatalf("issued ticket did not verify: valid=%v err=%v", valid, err)
		}
		if valid, err := store.VerifyTicket(ticket, aliceID, 900, want); err != nil || valid {
			t.Fatalf("ticket verified for wrong appid: valid=%v err=%v", valid, err)
		}
		tampered := append([]byte(nil), ticket...)
		tampered[0] ^= 0xff
		if valid, err := store.VerifyTicket(tampered, aliceID, 480, want); err != nil || valid {
			t.Fatalf("tampered ticket verified: valid=%v err=%v", valid, err)
		}
	}
}

func TestHTTPOnGamePortGetsExplanation(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(store, auth.NewLimiter(nil)).Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if _, err := c.Write([]byte("GET / HTTP/1.1\r\nHost: localhost\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	reply, err := io.ReadAll(c)
	if err != nil {
		t.Fatal(err)
	}
	text := string(reply)
	if !strings.Contains(text, "400 Bad Request") || !strings.Contains(text, "8081") {
		t.Fatalf("bad HTTP refusal: %q", text)
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

func storeWithStats(t *testing.T) (*state.Store, uint64) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sapie.db")
	store, err := state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	id, err := store.CreateAccount("alice", "h", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, row := range []struct {
		appid uint32
		name  string
	}{{480, "kills"}, {480, "deaths"}, {900, "wins"}} {
		if _, err := db.Exec(`INSERT INTO stats (steam_id, appid, name, value) VALUES (?, ?, ?, ?)`,
			id, row.appid, row.name, []byte("1")); err != nil {
			t.Fatal(err)
		}
	}
	return store, id
}

func TestStatsGetNamesFilter(t *testing.T) {
	store, alice := storeWithStats(t)
	srv := New(store, nil)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	session := &Session{steamID: alice, conn: serverConn}
	request := func(appid uint32, names ...string) string {
		t.Helper()
		done := make(chan struct{})
		go func() {
			srv.handleStatsGet(session, &protocol.StatsGet{Appid: appid, Names: names})
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
		var got []string
		for _, stat := range response.StatsResponse.GetStats() {
			got = append(got, stat.GetName())
		}
		return strings.Join(got, ",")
	}
	if got, want := request(0), "deaths,kills,wins"; got != want {
		t.Fatalf("all stats = %q, want %q", got, want)
	}
	if got, want := request(0, "kills"), "kills"; got != want {
		t.Fatalf("named stats = %q, want %q", got, want)
	}
	if got, want := request(480, "kills", "wins"), "kills"; got != want {
		t.Fatalf("named stats with appid = %q, want %q", got, want)
	}
	if got := request(0, "missing"); got != "" {
		t.Fatalf("unknown name = %q, want empty", got)
	}
}

func TestHiddenUnlockNotPushedToFriends(t *testing.T) {
	store, alice, bob := testStore(t)
	srv := New(store, nil)
	observerConn, readerConn := net.Pipe()
	defer observerConn.Close()
	defer readerConn.Close()
	srv.peers[bob] = &Session{steamID: bob, conn: observerConn}
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

	unlocker := &Session{steamID: alice}
	srv.handleAchievementUnlock(unlocker, &protocol.AchievementUnlock{
		Appid: 480,
		Def:   &protocol.AchievementDef{Id: "secret_relic", DisplayName: "Secret", Hidden: true},
	})
	select {
	case frame := <-frames:
		t.Fatalf("hidden unlock pushed to friend: %+v", frame.Payload)
	case <-time.After(150 * time.Millisecond):
	}

	stats, err := store.StatsOf(alice)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats.Achievements) != 1 || stats.Achievements[0].ID != "secret_relic" || !stats.Achievements[0].Hidden {
		t.Fatalf("hidden unlock not stored canonically: %+v", stats.Achievements)
	}

	ownerServer, ownerClient := net.Pipe()
	defer ownerServer.Close()
	defer ownerClient.Close()
	ownerSession := &Session{steamID: alice, conn: ownerServer}
	go srv.handleStatsGet(ownerSession, &protocol.StatsGet{})
	ownerClient.SetReadDeadline(time.Now().Add(2 * time.Second))
	frame, err := protocol.ReadFrame(ownerClient)
	if err != nil {
		t.Fatal(err)
	}
	ownerView, ok := frame.Payload.(*protocol.Frame_StatsResponse)
	if !ok || len(ownerView.StatsResponse.GetAchievements()) != 1 ||
		ownerView.StatsResponse.GetAchievements()[0].GetName() != "secret_relic" {
		t.Fatalf("owner cannot see hidden unlock: %+v", frame.Payload)
	}

	srv.handleAchievementUnlock(unlocker, &protocol.AchievementUnlock{
		Appid: 480,
		Def:   &protocol.AchievementDef{Id: "public_win", DisplayName: "Public"},
	})
	select {
	case frame := <-frames:
		msg := steamMessage(frame)
		if msg == nil || msg.GetAchievementUnlock() == nil || msg.GetAchievementUnlock().GetDef().GetId() != "public_win" {
			t.Fatalf("bad public unlock push: %+v", frame.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("public unlock not pushed to friend")
	}
}

func frameCollector(conn net.Conn) <-chan *protocol.Frame {
	frames := make(chan *protocol.Frame, 16)
	go func() {
		for {
			frame, err := protocol.ReadFrame(conn)
			if err != nil {
				return
			}
			frames <- frame
		}
	}()
	return frames
}

func waitSteam(t *testing.T, frames <-chan *protocol.Frame, match func(*protocol.Common_Message) bool) *protocol.Common_Message {
	t.Helper()
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case frame, ok := <-frames:
			if !ok {
				t.Fatal("frame channel closed")
			}
			if msg := steamMessage(frame); msg != nil && match(msg) {
				return msg
			}
		case <-timeout.C:
			t.Fatal("timed out waiting for steam frame")
		}
	}
}

func lobbyMessage(id uint64, typ protocol.Lobby_Messages_Types, dest uint64) *protocol.Common_Message {
	return &protocol.Common_Message{
		DestId: dest,
		Messages: &protocol.Common_Message_LobbyMessages{LobbyMessages: &protocol.Lobby_Messages{
			Id: id, Type: typ,
		}},
	}
}

func TestSocketLobbyJoinAndLeave(t *testing.T) {
	store, alice, bob := testStore(t)
	carol, err := store.CreateAccount("carol", "h3", "Carol")
	if err != nil {
		t.Fatal(err)
	}
	lobbyID, err := store.CreateLobby(alice, 480, state.LobbyTypePublic, 3)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(store, nil)

	aliceServer, aliceClient := net.Pipe()
	defer aliceServer.Close()
	defer aliceClient.Close()
	aliceSession := &Session{steamID: alice, personaName: "Alice", appid: 480, conn: aliceServer}
	srv.peers[alice] = aliceSession
	aliceFrames := frameCollector(aliceClient)

	bobServer, bobClient := net.Pipe()
	defer bobServer.Close()
	defer bobClient.Close()
	bobSession := &Session{steamID: bob, personaName: "Bob", appid: 480, conn: bobServer}
	srv.peers[bob] = bobSession
	bobFrames := frameCollector(bobClient)

	srv.handleSteam(bobSession, lobbyMessage(lobbyID, protocol.Lobby_Messages_JOIN, alice))
	join := waitSteam(t, aliceFrames, func(msg *protocol.Common_Message) bool {
		return msg.GetLobbyMessages().GetType() == protocol.Lobby_Messages_JOIN
	})
	if join.GetSourceId() != bob || join.GetDestId() != alice {
		t.Fatalf("join frame = %+v", join)
	}
	lobby, err := store.Lobby(lobbyID)
	if err != nil || len(lobby.Members) != 2 || lobby.Members[1] != bob {
		t.Fatalf("lobby after socket join = %+v, %v", lobby, err)
	}

	// The owner's full snapshot must now reach the joiner through the router.
	srv.handleSteam(aliceSession, &protocol.Common_Message{
		Messages: &protocol.Common_Message_Lobby{Lobby: &protocol.Lobby{
			RoomId: lobbyID, Owner: alice, Appid: 480, MemberLimit: 3,
			Type: state.LobbyTypePublic, Joinable: true,
			Members: []*protocol.Lobby_Member{{Id: alice}, {Id: bob}},
		}},
	})
	snapshot := waitSteam(t, bobFrames, func(msg *protocol.Common_Message) bool { return msg.GetLobby() != nil })
	if snapshot.GetLobby().GetRoomId() != lobbyID || len(snapshot.GetLobby().GetMembers()) != 2 {
		t.Fatalf("routed lobby snapshot = %+v", snapshot.GetLobby())
	}

	srv.handleSteam(&Session{steamID: carol, appid: 900}, lobbyMessage(lobbyID, protocol.Lobby_Messages_JOIN, alice))
	lobby, err = store.Lobby(lobbyID)
	if err != nil || len(lobby.Members) != 2 {
		t.Fatalf("wrong-appid join changed roster: %+v, %v", lobby, err)
	}

	srv.handleSteam(bobSession, lobbyMessage(lobbyID, protocol.Lobby_Messages_LEAVE, alice))
	left := waitSteam(t, aliceFrames, func(msg *protocol.Common_Message) bool {
		return msg.GetLobbyMessages().GetType() == protocol.Lobby_Messages_LEAVE
	})
	if left.GetSourceId() != bob {
		t.Fatalf("leave frame = %+v", left)
	}
	lobby, err = store.Lobby(lobbyID)
	if err != nil || len(lobby.Members) != 1 || lobby.Members[0] != alice {
		t.Fatalf("lobby after socket leave = %+v, %v", lobby, err)
	}
}

func TestDisconnectUpdatesLobbyOwner(t *testing.T) {
	store, alice, bob := testStore(t)
	lobbyID, err := store.CreateLobby(alice, 480, state.LobbyTypePublic, 2)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(store, nil)

	aliceServer, aliceClient := net.Pipe()
	defer aliceServer.Close()
	defer aliceClient.Close()
	aliceSession := &Session{steamID: alice, personaName: "Alice", appid: 480, conn: aliceServer}
	srv.peers[alice] = aliceSession
	_ = frameCollector(aliceClient)

	bobServer, bobClient := net.Pipe()
	defer bobServer.Close()
	defer bobClient.Close()
	bobSession := &Session{steamID: bob, personaName: "Bob", appid: 480, conn: bobServer}
	srv.peers[bob] = bobSession
	bobFrames := frameCollector(bobClient)

	srv.handleSteam(bobSession, lobbyMessage(lobbyID, protocol.Lobby_Messages_JOIN, alice))
	srv.disconnect(aliceSession)
	changed := waitSteam(t, bobFrames, func(msg *protocol.Common_Message) bool {
		lm := msg.GetLobbyMessages()
		return lm != nil && lm.GetType() == protocol.Lobby_Messages_CHANGE_OWNER && lm.GetIdata() == bob
	})
	if changed.GetSourceId() != alice {
		t.Fatalf("owner change frame = %+v", changed)
	}
	lobby, err := store.Lobby(lobbyID)
	if err != nil || lobby.Owner != bob || len(lobby.Members) != 1 {
		t.Fatalf("lobby after owner disconnect = %+v, %v", lobby, err)
	}
}

func TestStampSourceHint(t *testing.T) {
	ipv4 := net.ParseIP("203.0.113.7")
	realIP := binary.BigEndian.Uint32(ipv4.To4())
	for _, tc := range []struct {
		name         string
		remote       net.Addr
		suppliedIP   uint32
		suppliedPort uint32
		wantIP       uint32
		wantPort     uint32
	}{
		{"client UDP port preserved", &net.TCPAddr{IP: ipv4, Port: 27015}, 111, 47584, realIP, 47584},
		{"zero port falls back to TCP", &net.TCPAddr{IP: ipv4, Port: 27015}, 111, 0, realIP, 27015},
		{"out-of-range port falls back to TCP", &net.TCPAddr{IP: ipv4, Port: 27015}, 111, 70000, realIP, 27015},
		{"max port preserved", &net.TCPAddr{IP: ipv4, Port: 27015}, 111, 65535, realIP, 65535},
		{"ipv6 gets no hint", &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 27015}, 111, 47584, 0, 0},
		{"non-tcp gets no hint", &net.UDPAddr{IP: ipv4, Port: 27015}, 111, 47584, 0, 0},
		{"nil gets no hint", nil, 111, 47584, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := &protocol.Common_Message{SourceIp: tc.suppliedIP, SourcePort: tc.suppliedPort}
			stampSourceHint(tc.remote, msg)
			if msg.SourceIp != tc.wantIP || msg.SourcePort != tc.wantPort {
				t.Fatalf("hint = ip %d port %d, want ip %d port %d (client supplied ip %d port %d)",
					msg.SourceIp, msg.SourcePort, tc.wantIP, tc.wantPort, tc.suppliedIP, tc.suppliedPort)
			}
		})
	}
}

type addrConn struct {
	net.Conn
	addr net.Addr
}

func (c addrConn) RemoteAddr() net.Addr { return c.addr }

func TestHandleSteamStampsNetworkingSocketsSource(t *testing.T) {
	store, alice, bob := testStore(t)
	srv := New(store, nil)

	aliceServer, _ := net.Pipe()
	defer aliceServer.Close()
	remote := &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 27015}
	aliceSession := &Session{steamID: alice, conn: addrConn{Conn: aliceServer, addr: remote}}

	bobServer, bobClient := net.Pipe()
	defer bobServer.Close()
	defer bobClient.Close()
	srv.peers[bob] = &Session{steamID: bob, conn: bobServer}
	bobFrames := frameCollector(bobClient)

	const declaredPort = 47584
	srv.handleSteam(aliceSession, &protocol.Common_Message{
		DestId: bob,
		Messages: &protocol.Common_Message_NetworkingSockets{
			NetworkingSockets: &protocol.Networking_Sockets{Type: protocol.Networking_Sockets_CONNECTION_REQUEST},
		},
		SourceIp: 111, SourcePort: declaredPort,
	})

	msg := waitSteam(t, bobFrames, func(m *protocol.Common_Message) bool { return m.GetNetworkingSockets() != nil })
	wantIP := binary.BigEndian.Uint32(remote.IP.To4())
	if msg.SourceIp != wantIP || msg.SourcePort != declaredPort {
		t.Fatalf("relayed hint = ip %d port %d, want ip %d port %d",
			msg.SourceIp, msg.SourcePort, wantIP, declaredPort)
	}
	if msg.GetSourceId() != alice {
		t.Fatalf("relayed source_id = %d, want %d", msg.GetSourceId(), alice)
	}
}

func TestContentServerRequestGetsNeutralAck(t *testing.T) {
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
	go New(store, auth.NewLimiter(nil)).Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "pw"}}}); err != nil {
		t.Fatal(err)
	}
	readLoginResponse(t, c)

	req := &protocol.Frame{Payload: &protocol.Frame_ContentServerRequest{ContentServerRequest: &protocol.ContentServerRequest{Appid: 480}}}
	if err := protocol.WriteFrame(c, req); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := protocol.ReadFrame(c)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := frame.Payload.(*protocol.Frame_ContentServerResponse)
	if !ok {
		t.Fatalf("got %T, want ContentServerResponse", frame.Payload)
	}
	if !result.ContentServerResponse.GetOk() {
		t.Fatal("content server response ok = false, want true")
	}
}

func TestAccountLockoutAfterRepeatedBadPasswords(t *testing.T) {
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

	login := func(password string) *protocol.LoginResponse {
		t.Helper()
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		req := &protocol.LoginRequest{Username: "alice", Password: password}
		if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
			t.Fatal(err)
		}
		return readLoginResponse(t, c)
	}

	for i := 0; i < auth.MaxFailures; i++ {
		if got := login("wrong").GetStatus(); got != protocol.LoginResponse_BAD_CREDENTIALS {
			t.Fatalf("failure %d status = %v, want BAD_CREDENTIALS", i+1, got)
		}
	}
	// Clear the IP counter so the next refusal can only come from the account lockout.
	srv.limiter.Reset("127.0.0.1")
	allowed, err := srv.accountAllowed(id)
	if err != nil || allowed {
		t.Fatalf("accountAllowed = %v, %v; want false, nil", allowed, err)
	}
	if got := login("pw").GetStatus(); got != protocol.LoginResponse_RATE_LIMITED {
		t.Fatalf("locked correct-password login status = %v, want RATE_LIMITED", got)
	}
}

func TestTokenLoginWorksBelowAccountLockout(t *testing.T) {
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
	if err := store.CreateSession(id, auth.HashToken("m16-handoff-token"), time.Now().Add(time.Hour)); err != nil {
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

	for i := 0; i < auth.MaxFailures-1; i++ {
		if err := srv.limiter.RecordBucket(auth.AccountBucket(id)); err != nil {
			t.Fatal(err)
		}
	}
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	req := &protocol.LoginRequest{
		Username: "not-alice", Password: "wrong", SessionToken: "m16-handoff-token",
	}
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
		t.Fatal(err)
	}
	response := readLoginResponse(t, c)
	if response.GetStatus() != protocol.LoginResponse_OK || response.GetSteamId() != id ||
		len(response.GetSessionToken()) == 0 {
		t.Fatalf("token login below lockout: %+v", response)
	}
}

func TestWrongTokenRejectedByConstantTimeCompare(t *testing.T) {
	const valid = "m16-valid-token-of-same-length"
	wrong := valid[:len(valid)-1] + "X"
	if !auth.Equal(valid, valid) || auth.Equal(valid, wrong) {
		t.Fatal("constant-time token compare misbehaved")
	}

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
	if err := store.CreateSession(id, auth.HashToken(valid), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(store, auth.NewLimiter(nil)).Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	req := &protocol.LoginRequest{Username: "alice", SessionToken: wrong}
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
		t.Fatal(err)
	}
	if got := readLoginResponse(t, c).GetStatus(); got != protocol.LoginResponse_BAD_CREDENTIALS {
		t.Fatalf("wrong token status = %v, want BAD_CREDENTIALS", got)
	}
}

func TestOversizedFrameOnGamePortRejected(t *testing.T) {
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
	go New(store, auth.NewLimiter(nil)).Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	head := make([]byte, 4)
	binary.BigEndian.PutUint32(head, protocol.MaxFrameSize+1)
	if _, err := c.Write(append(head, []byte("overflow")...)); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := protocol.ReadFrame(c); err == nil {
		t.Fatal("oversized frame was accepted")
	}

	// The rejected frame must not take down the listener.
	c2, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	req := &protocol.LoginRequest{Username: "alice", Password: "pw"}
	if err := protocol.WriteFrame(c2, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
		t.Fatal(err)
	}
	if got := readLoginResponse(t, c2).GetStatus(); got != protocol.LoginResponse_OK {
		t.Fatalf("login after oversized frame = %v, want OK", got)
	}
}

func TestIdleAuthenticatedConnectionClosed(t *testing.T) {
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
	srv.idleTimeout = 150 * time.Millisecond
	go srv.Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	req := &protocol.LoginRequest{Username: "alice", Password: "pw"}
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
		t.Fatal(err)
	}
	readLoginResponse(t, c)

	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, err := protocol.ReadFrame(c); err == nil {
		t.Fatal("idle authenticated connection stayed open")
	}
}

func TestIPLockoutUsesRateLimitedStatus(t *testing.T) {
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

	login := func(username, password string) *protocol.LoginResponse {
		t.Helper()
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		req := &protocol.LoginRequest{Username: username, Password: password}
		if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
			t.Fatal(err)
		}
		return readLoginResponse(t, c)
	}
	for i := 0; i < auth.MaxFailures; i++ {
		if got := login("nobody", "wrong").GetStatus(); got != protocol.LoginResponse_BAD_CREDENTIALS {
			t.Fatalf("failure %d = %v, want BAD_CREDENTIALS", i+1, got)
		}
	}
	if got := login("alice", "pw").GetStatus(); got != protocol.LoginResponse_RATE_LIMITED {
		t.Fatalf("IP-locked login = %v, want RATE_LIMITED", got)
	}
}

func TestGameLoginKeepsPresentedTokenValid(t *testing.T) {
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
	if err := store.CreateSession(id, auth.HashToken("presented-token"), time.Now().Add(time.Hour)); err != nil {
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

	login := func(token string) *protocol.LoginResponse {
		t.Helper()
		c, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		req := &protocol.LoginRequest{SessionToken: token}
		if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: req}}); err != nil {
			t.Fatal(err)
		}
		return readLoginResponse(t, c)
	}

	first := login("presented-token")
	if first.GetStatus() != protocol.LoginResponse_OK || len(first.GetSessionToken()) == 0 {
		t.Fatalf("token login = %v, token length %d", first.GetStatus(), len(first.GetSessionToken()))
	}
	second := login("presented-token")
	if second.GetStatus() != protocol.LoginResponse_OK {
		t.Fatalf("second token login = %v, want OK; 0.4 DLL and keychain tokens must stay valid", second.GetStatus())
	}
	if resolved, err := store.ResolveSession(auth.HashToken("presented-token")); err != nil || resolved != id {
		t.Fatalf("presented token after game login = %d, %v", resolved, err)
	}
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
