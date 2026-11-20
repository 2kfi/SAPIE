// Command harness starts SAPIE in-process and proves login, friends and lobbies.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sapie/sapie/server/internal/api"
	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/session"
	"github.com/sapie/sapie/server/internal/state"
)

const adminToken = "harness-admin-token"

const connectString = "connect=127.0.0.1:27015"

func main() {
	if err := run(); err != nil {
		log.Fatalf("FAIL %v", err)
	}
	fmt.Println("PASS harness")
}

// gameClient drains the socket so pings and pushed presence never block a reply.
type gameClient struct {
	conn    net.Conn
	frames  chan *protocol.Frame
	stop    chan struct{}
	pending []*protocol.Frame
}

func newGameClient(conn net.Conn) *gameClient {
	c := &gameClient{conn: conn, frames: make(chan *protocol.Frame, 128), stop: make(chan struct{})}
	go c.read()
	return c
}

func (c *gameClient) read() {
	defer close(c.frames)
	for {
		frame, err := protocol.ReadFrame(c.conn)
		if err != nil {
			return
		}
		select {
		case c.frames <- frame:
		case <-c.stop:
			return
		}
	}
}

func (c *gameClient) next(match func(*protocol.Frame) bool) (*protocol.Frame, error) {
	for i, frame := range c.pending {
		if match(frame) {
			c.pending = append(c.pending[:i], c.pending[i+1:]...)
			return frame, nil
		}
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case frame, ok := <-c.frames:
			if !ok {
				return nil, fmt.Errorf("connection closed")
			}
			if match(frame) {
				return frame, nil
			}
			c.pending = append(c.pending, frame)
		case <-timer.C:
			return nil, fmt.Errorf("timed out waiting for frame")
		}
	}
}

func (c *gameClient) received(match func(*protocol.Frame) bool, wait time.Duration) bool {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case frame, ok := <-c.frames:
			if !ok {
				return false
			}
			if match(frame) {
				return true
			}
			c.pending = append(c.pending, frame)
		case <-timer.C:
			for _, frame := range c.pending {
				if match(frame) {
					return true
				}
			}
			return false
		}
	}
}

func (c *gameClient) send(frame *protocol.Frame) error { return protocol.WriteFrame(c.conn, frame) }

func waitLobby(store *state.Store, lobbyID uint64, members int) (state.Lobby, error) {
	deadline := time.Now().Add(3 * time.Second)
	for {
		lobby, err := store.Lobby(lobbyID)
		if err == nil && len(lobby.Members) == members {
			return lobby, nil
		}
		if time.Now().After(deadline) {
			if err == nil {
				err = fmt.Errorf("members = %d, want %d", len(lobby.Members), members)
			}
			return state.Lobby{}, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (c *gameClient) login(username, password string) (*protocol.LoginResponse, error) {
	frame := &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{
		Username: username, Password: password, Appid: 480, ClientVersion: "harness",
	}}}
	if err := c.send(frame); err != nil {
		return nil, fmt.Errorf("send login: %w", err)
	}
	response, err := c.next(func(f *protocol.Frame) bool {
		_, ok := f.Payload.(*protocol.Frame_LoginResponse)
		return ok
	})
	if err != nil {
		return nil, fmt.Errorf("read login response: %w", err)
	}
	result, ok := response.Payload.(*protocol.Frame_LoginResponse)
	if !ok {
		return nil, fmt.Errorf("got %T, want LoginResponse", response.Payload)
	}
	if result.LoginResponse.Status != protocol.LoginResponse_OK {
		return nil, fmt.Errorf("login %s status %v", username, result.LoginResponse.Status)
	}
	return result.LoginResponse, nil
}

func (c *gameClient) requestTicket(appid uint32) (*protocol.TicketResponse, error) {
	frame := &protocol.Frame{Payload: &protocol.Frame_TicketRequest{TicketRequest: &protocol.TicketRequest{
		Appid: appid, Data: []byte("harness-ticket-request"),
	}}}
	if err := c.send(frame); err != nil {
		return nil, fmt.Errorf("send ticket request: %w", err)
	}
	response, err := c.next(func(f *protocol.Frame) bool {
		_, ok := f.Payload.(*protocol.Frame_TicketResponse)
		return ok
	})
	if err != nil {
		return nil, fmt.Errorf("read ticket response: %w", err)
	}
	result, ok := response.Payload.(*protocol.Frame_TicketResponse)
	if !ok {
		return nil, fmt.Errorf("got %T, want TicketResponse", response.Payload)
	}
	return result.TicketResponse, nil
}

func (c *gameClient) requestStats(appid uint32, steamID uint64) (*protocol.StatsResponse, error) {
	frame := &protocol.Frame{Payload: &protocol.Frame_StatsGet{StatsGet: &protocol.StatsGet{
		Appid: appid, SteamId: steamID,
	}}}
	if err := c.send(frame); err != nil {
		return nil, fmt.Errorf("send stats get: %w", err)
	}
	response, err := c.next(func(f *protocol.Frame) bool {
		_, ok := f.Payload.(*protocol.Frame_StatsResponse)
		return ok
	})
	if err != nil {
		return nil, fmt.Errorf("read stats response: %w", err)
	}
	result, ok := response.Payload.(*protocol.Frame_StatsResponse)
	if !ok {
		return nil, fmt.Errorf("got %T, want StatsResponse", response.Payload)
	}
	return result.StatsResponse, nil
}

func expectAchievements(resp *protocol.StatsResponse, want ...string) error {
	got := map[string]bool{}
	for _, a := range resp.GetAchievements() {
		got[a.GetName()] = true
	}
	if len(got) != len(want) {
		return fmt.Errorf("achievements = %v, want %v", got, want)
	}
	for _, name := range want {
		if !got[name] {
			return fmt.Errorf("achievements = %v, want %v", got, want)
		}
	}
	return nil
}

func steamMessage(frame *protocol.Frame) *protocol.Common_Message {
	steam, ok := frame.Payload.(*protocol.Frame_Steam)
	if !ok {
		return nil
	}
	return steam.Steam
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := state.Open(":memory:")
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	gameLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen game: %w", err)
	}
	adminLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen admin: %w", err)
	}
	limiter := auth.NewLimiter(nil)
	sessions := session.New(store, limiter)
	errs := make(chan error, 2)
	go func() { errs <- sessions.Serve(ctx, gameLn) }()
	go func() { errs <- api.New(store, adminToken, limiter, sessions).Serve(ctx, adminLn) }()
	defer func() {
		cancel()
		for i := 0; i < 2; i++ {
			<-errs
		}
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	adminURL := "http://" + adminLn.Addr().String()
	alice, err := createAccount(client, adminURL, "harness-alice", "alice-pw", "Alice")
	if err != nil {
		return err
	}
	bob, err := createAccount(client, adminURL, "harness-bob", "bob-pw", "Bob")
	if err != nil {
		return err
	}
	if err := store.AddFriend(alice.SteamID, bob.SteamID); err != nil {
		return fmt.Errorf("seed friendship: %w", err)
	}

	aliceConn, err := net.Dial("tcp", gameLn.Addr().String())
	if err != nil {
		return fmt.Errorf("dial alice: %w", err)
	}
	defer aliceConn.Close()
	aliceClient := newGameClient(aliceConn)
	aliceLogin, err := aliceClient.login("harness-alice", "alice-pw")
	if err != nil {
		return err
	}
	fmt.Printf("PASS login alice steam_id=%d\n", aliceLogin.SteamId)

	bobConn, err := net.Dial("tcp", gameLn.Addr().String())
	if err != nil {
		return fmt.Errorf("dial bob: %w", err)
	}
	defer bobConn.Close()
	bobClient := newGameClient(bobConn)
	bobLogin, err := bobClient.login("harness-bob", "bob-pw")
	if err != nil {
		return err
	}
	fmt.Printf("PASS login bob steam_id=%d\n", bobLogin.SteamId)

	if aliceLogin.SteamId != alice.SteamID || bobLogin.SteamId != bob.SteamID {
		return fmt.Errorf("login IDs %d/%d differ from admin records %d/%d",
			aliceLogin.SteamId, bobLogin.SteamId, alice.SteamID, bob.SteamID)
	}
	fmt.Println("PASS login SteamIDs match admin records")
	if aliceLogin.SteamId == bobLogin.SteamId {
		return fmt.Errorf("duplicate SteamID %d", aliceLogin.SteamId)
	}
	if len(aliceLogin.SessionToken) == 0 || len(bobLogin.SessionToken) == 0 {
		return fmt.Errorf("missing session tokens")
	}
	fmt.Println("PASS distinct SteamIDs")

	ticket, err := aliceClient.requestTicket(480)
	if err != nil {
		return err
	}
	if len(ticket.Ticket) == 0 || ticket.Number == 0 {
		return fmt.Errorf("bad ticket response: %v", ticket)
	}
	fmt.Printf("PASS ticket issued steam_id=%d appid=480 number=%d bytes=%d\n",
		aliceLogin.SteamId, ticket.Number, len(ticket.Ticket))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adminURL+"/api/v1/accounts", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("list accounts status %d: %s", resp.StatusCode, body)
	}
	var accounts []state.Account
	if err := json.Unmarshal(body, &accounts); err != nil {
		return fmt.Errorf("decode accounts: %w", err)
	}
	if len(accounts) != 2 {
		return fmt.Errorf("got %d accounts, want 2", len(accounts))
	}
	seen := map[uint64]bool{alice.SteamID: true, bob.SteamID: true}
	for _, account := range accounts {
		if !seen[account.SteamID] {
			return fmt.Errorf("unexpected account %d in list", account.SteamID)
		}
	}
	if strings.Contains(string(body), "password") {
		return fmt.Errorf("admin API exposed password data: %s", body)
	}
	fmt.Println("PASS admin API lists both accounts")

	aliceToken := string(aliceLogin.SessionToken)
	bobToken := string(bobLogin.SessionToken)
	if err := checkFriends(client, adminURL, aliceToken, aliceLogin.SteamId, bob.SteamID, "Bob"); err != nil {
		return err
	}
	if err := checkFriends(client, adminURL, bobToken, bob.SteamID, alice.SteamID, "Alice"); err != nil {
		return err
	}
	fmt.Println("PASS M2 both friends see each other online")

	presence, err := aliceClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetFriend() != nil && msg.GetFriend().GetId() == bob.SteamID &&
			string(msg.GetFriend().GetRichPresence()[state.PersonaStateKey]) == "1"
	})
	if err != nil || steamMessage(presence).GetSourceId() != bob.SteamID {
		return fmt.Errorf("alice did not receive bob online presence: %v", err)
	}
	fmt.Println("PASS M2 bob presence pushed to alice over game socket")

	status, body, err := apiCall(client, http.MethodPost,
		fmt.Sprintf("%s/api/v1/friends/%d/invite", adminURL, bob.SteamID), aliceToken,
		map[string]string{"connect_str": connectString})
	if err != nil || status != http.StatusAccepted {
		return fmt.Errorf("HTTP invite = %d: %s (%v)", status, body, err)
	}
	invite, err := bobClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetFriendMessages() != nil &&
			msg.GetFriendMessages().GetType() == protocol.Friend_Messages_GAME_INVITE
	})
	if err != nil {
		return fmt.Errorf("bob did not receive invite: %w", err)
	}
	got := steamMessage(invite)
	if got.SourceId != alice.SteamID || string(got.GetFriendMessages().GetConnectStr()) != connectString {
		return fmt.Errorf("bad invite: source=%d connect=%q", got.SourceId, got.GetFriendMessages().GetConnectStr())
	}
	fmt.Printf("PASS M2 bob received game invite from alice connect_str=%s\n", connectString)

	if err := checkConnectString(client, adminURL, bobToken, alice.SteamID, connectString); err != nil {
		return err
	}
	fmt.Println("PASS M2 invite connect string recorded in alice presence")

	socketConnect := "connect=10.0.0.7:27016"
	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
		DestId: bob.SteamID,
		Messages: &protocol.Common_Message_FriendMessages{FriendMessages: &protocol.Friend_Messages{
			Type:       protocol.Friend_Messages_GAME_INVITE,
			InviteData: &protocol.Friend_Messages_ConnectStr{ConnectStr: []byte(socketConnect)},
		}},
	}}}); err != nil {
		return err
	}
	frame, err := bobClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetFriendMessages() != nil &&
			string(msg.GetFriendMessages().GetConnectStr()) == socketConnect
	})
	if err != nil || steamMessage(frame).GetSourceId() != alice.SteamID {
		return fmt.Errorf("socket invite not routed to bob: %v", err)
	}
	fmt.Println("PASS M2 socket-sent invite routed and source_id stamped")

	status, body, err = apiCall(client, http.MethodPost, adminURL+"/api/v1/lobbies", aliceToken,
		map[string]any{"appid": 480, "type": 2, "member_limit": 4})
	if err != nil || status != http.StatusCreated {
		return fmt.Errorf("create lobby = %d: %s (%v)", status, body, err)
	}
	var created struct {
		LobbyID uint64 `json:"lobby_id"`
	}
	if err := json.Unmarshal(body, &created); err != nil || created.LobbyID == 0 {
		return fmt.Errorf("bad lobby response: %s (%v)", body, err)
	}
	fmt.Printf("PASS M3 alice created lobby_id=%d\n", created.LobbyID)

	frame, err = aliceClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetLobby() != nil && msg.GetLobby().GetRoomId() == created.LobbyID
	})
	if err != nil || len(steamMessage(frame).GetLobby().GetMembers()) != 1 {
		return fmt.Errorf("alice lobby update = %v", err)
	}
	fmt.Println("PASS M3 lobby creation pushed to owner")

	status, body, err = apiCall(client, http.MethodPost,
		fmt.Sprintf("%s/api/v1/lobbies/%d/join", adminURL, created.LobbyID), bobToken, nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("join lobby = %d: %s (%v)", status, body, err)
	}
	for _, c := range []*gameClient{aliceClient, bobClient} {
		frame, err := c.next(func(f *protocol.Frame) bool {
			msg := steamMessage(f)
			return msg != nil && msg.GetLobby() != nil && msg.GetLobby().GetRoomId() == created.LobbyID &&
				len(msg.GetLobby().GetMembers()) == 2
		})
		if err != nil {
			return fmt.Errorf("lobby join update: %v", err)
		}
		members := steamMessage(frame).GetLobby().GetMembers()
		if members[0].GetId() != alice.SteamID || members[1].GetId() != bob.SteamID {
			return fmt.Errorf("unexpected lobby members: %+v", members)
		}
	}
	fmt.Println("PASS M3 both clients observe same lobby_id and member list")

	for _, caller := range []struct{ name, token string }{
		{"alice", aliceToken}, {"bob", bobToken}, {"admin", adminToken},
	} {
		status, body, err = apiCall(client, http.MethodGet, adminURL+"/api/v1/lobbies", caller.token, nil)
		if err != nil || status != http.StatusOK {
			return fmt.Errorf("list lobbies as %s = %d: %s (%v)", caller.name, status, body, err)
		}
		var listed struct {
			Lobbies []state.Lobby `json:"lobbies"`
		}
		if err := json.Unmarshal(body, &listed); err != nil || len(listed.Lobbies) != 1 ||
			listed.Lobbies[0].LobbyID != created.LobbyID || len(listed.Lobbies[0].Members) != 2 ||
			listed.Lobbies[0].Owner != alice.SteamID {
			return fmt.Errorf("lobby listing = %s (%v)", body, err)
		}
	}
	fmt.Println("PASS M3 user and admin GET /api/v1/lobbies show the lobby")

	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
		Messages: &protocol.Common_Message_LobbyMessages{LobbyMessages: &protocol.Lobby_Messages{
			Id: created.LobbyID, Type: protocol.Lobby_Messages_CHAT_MESSAGE, Bdata: []byte("gg"),
		}},
	}}}); err != nil {
		return err
	}
	frame, err = bobClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetLobbyMessages() != nil && msg.GetLobbyMessages().GetId() == created.LobbyID
	})
	if err != nil || steamMessage(frame).GetSourceId() != alice.SteamID {
		return fmt.Errorf("lobby chat not routed to bob: %v", err)
	}
	fmt.Println("PASS M3 socket-sent lobby message routed to members")

	const socketLobbyID uint64 = 76561198000000001
	aliceSnapshot := &protocol.Lobby{
		RoomId: socketLobbyID, Owner: alice.SteamID, Appid: 480,
		MemberLimit: 2, Type: 2, Joinable: true,
		Members: []*protocol.Lobby_Member{{Id: alice.SteamID}},
	}
	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
		Messages: &protocol.Common_Message_Lobby{Lobby: aliceSnapshot},
	}}}); err != nil {
		return err
	}
	if _, err := waitLobby(store, socketLobbyID, 1); err != nil {
		return fmt.Errorf("socket lobby snapshot: %w", err)
	}
	fmt.Printf("PASS M9 client-created lobby_id=%d mirrored from game socket\n", socketLobbyID)

	joinPacket := func() *protocol.Frame {
		return &protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
			DestId: alice.SteamID,
			Messages: &protocol.Common_Message_LobbyMessages{LobbyMessages: &protocol.Lobby_Messages{
				Id: socketLobbyID, Type: protocol.Lobby_Messages_JOIN,
			}},
		}}}
	}
	if err := bobClient.send(joinPacket()); err != nil {
		return err
	}
	if _, err := waitLobby(store, socketLobbyID, 2); err != nil {
		return fmt.Errorf("socket join: %w", err)
	}
	frame, err = aliceClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetLobbyMessages() != nil &&
			msg.GetLobbyMessages().GetId() == socketLobbyID &&
			msg.GetLobbyMessages().GetType() == protocol.Lobby_Messages_JOIN &&
			msg.GetSourceId() == bob.SteamID
	})
	if err != nil {
		return fmt.Errorf("JOIN not routed to owner: %w", err)
	}
	fmt.Println("PASS M9 join-friend JOIN recorded and routed to owner")

	aliceSnapshot.Members = append(aliceSnapshot.Members, &protocol.Lobby_Member{Id: bob.SteamID})
	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
		Messages: &protocol.Common_Message_Lobby{Lobby: aliceSnapshot},
	}}}); err != nil {
		return err
	}
	frame, err = bobClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetLobby() != nil && msg.GetLobby().GetRoomId() == socketLobbyID &&
			len(msg.GetLobby().GetMembers()) == 2
	})
	if err != nil || len(steamMessage(frame).GetLobby().GetMembers()) != 2 {
		return fmt.Errorf("roster not routed to joiner: %v", err)
	}
	fmt.Println("PASS M9 joiner receives lobby roster through server routing")

	if err := bobClient.send(joinPacket()); err != nil {
		return err
	}
	lobby, err := waitLobby(store, socketLobbyID, 2)
	if err != nil || lobby.Members[1] != bob.SteamID {
		return fmt.Errorf("duplicate join changed roster: %+v, %v", lobby, err)
	}
	fmt.Println("PASS M9 duplicate JOIN leaves roster unchanged")

	if err := bobClient.send(&protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
		DestId: alice.SteamID,
		Messages: &protocol.Common_Message_LobbyMessages{LobbyMessages: &protocol.Lobby_Messages{
			Id: socketLobbyID, Type: protocol.Lobby_Messages_LEAVE,
		}},
	}}}); err != nil {
		return err
	}
	if _, err := waitLobby(store, socketLobbyID, 1); err != nil {
		return fmt.Errorf("socket leave: %w", err)
	}
	frame, err = aliceClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetLobbyMessages() != nil &&
			msg.GetLobbyMessages().GetId() == socketLobbyID &&
			msg.GetLobbyMessages().GetType() == protocol.Lobby_Messages_LEAVE &&
			msg.GetSourceId() == bob.SteamID
	})
	if err != nil {
		return fmt.Errorf("LEAVE not routed to owner: %w", err)
	}
	fmt.Println("PASS M9 leave removes member and notifies owner")

	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_AchievementUnlock{
		AchievementUnlock: &protocol.AchievementUnlock{
			Appid: 480,
			Def: &protocol.AchievementDef{
				Id: "first_kill", DisplayName: "First Kill", Description: "Kill one opponent",
			},
		},
	}}); err != nil {
		return fmt.Errorf("send unlock: %w", err)
	}
	frame, err = bobClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetAchievementUnlock() != nil &&
			msg.GetAchievementUnlock().GetDef().GetId() == "first_kill"
	})
	if err != nil {
		return fmt.Errorf("bob did not receive unlock push: %w", err)
	}
	pushed := steamMessage(frame).GetAchievementUnlock()
	if pushed.GetSteamId() != alice.SteamID || steamMessage(frame).GetSourceId() != alice.SteamID ||
		pushed.GetUnlockTime() <= 0 {
		return fmt.Errorf("bad unlock push: %+v", pushed)
	}
	unlockTime := pushed.GetUnlockTime()
	fmt.Printf("PASS M4 bob received alice unlock push unlock_time=%d\n", unlockTime)

	persisted, err := store.StatsOf(alice.SteamID)
	if err != nil {
		return fmt.Errorf("read persisted stats: %w", err)
	}
	if len(persisted.Achievements) != 1 || persisted.Achievements[0].UnlockTime != unlockTime ||
		persisted.Achievements[0].ID != "first_kill" {
		return fmt.Errorf("persisted unlock differs from push: %+v", persisted.Achievements)
	}
	fmt.Printf("PASS M4 alice unlock persisted with canonical unlock_time=%d\n", unlockTime)

	status, body, err = apiCall(client, http.MethodGet,
		fmt.Sprintf("%s/api/v1/users/%d/stats", adminURL, alice.SteamID), adminToken, nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("admin stats = %d: %s (%v)", status, body, err)
	}
	var stats state.UserStats
	if err := json.Unmarshal(body, &stats); err != nil {
		return fmt.Errorf("decode admin stats: %w", err)
	}
	if stats.SteamID != alice.SteamID || len(stats.Achievements) != 1 ||
		stats.Achievements[0].ID != "first_kill" || stats.Achievements[0].UnlockTime != unlockTime {
		return fmt.Errorf("admin stats unlock_time differs from push: %s", body)
	}
	fmt.Printf("PASS M4 admin API shows same unlock_time=%d\n", unlockTime)

	hiddenDef := protocol.AchievementDef{Id: "hidden_relic", DisplayName: "Hidden Relic", Hidden: true}
	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_AchievementUnlock{AchievementUnlock: &protocol.AchievementUnlock{
		Appid: 480, Def: &hiddenDef,
	}}}); err != nil {
		return fmt.Errorf("send hidden unlock: %w", err)
	}
	if bobClient.received(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetAchievementUnlock() != nil &&
			msg.GetAchievementUnlock().GetDef().GetId() == "hidden_relic"
	}, 300*time.Millisecond) {
		return fmt.Errorf("hidden achievement leaked to friend via unlock push")
	}
	fmt.Println("PASS hidden achievement unlock not pushed to friends")

	publicDef := protocol.AchievementDef{Id: "other_game_win", DisplayName: "Other Game Win"}
	if err := aliceClient.send(&protocol.Frame{Payload: &protocol.Frame_AchievementUnlock{AchievementUnlock: &protocol.AchievementUnlock{
		Appid: 900, Def: &publicDef,
	}}}); err != nil {
		return fmt.Errorf("send visible unlock: %w", err)
	}
	frame, err = bobClient.next(func(f *protocol.Frame) bool {
		msg := steamMessage(f)
		return msg != nil && msg.GetAchievementUnlock() != nil &&
			msg.GetAchievementUnlock().GetDef().GetId() == "other_game_win"
	})
	if err != nil {
		return fmt.Errorf("visible unlock push: %w", err)
	}
	fmt.Println("PASS visible achievement unlock still pushed to friends")

	bobView, err := bobClient.requestStats(0, alice.SteamID)
	if err != nil {
		return err
	}
	if err := expectAchievements(bobView, "first_kill", "other_game_win"); err != nil {
		return fmt.Errorf("bob stats: %w", err)
	}
	fmt.Println("PASS MR1 hidden achievement omitted from other user's StatsGet")

	aliceView, err := aliceClient.requestStats(0, 0)
	if err != nil {
		return err
	}
	if err := expectAchievements(aliceView, "first_kill", "hidden_relic", "other_game_win"); err != nil {
		return fmt.Errorf("alice own stats: %w", err)
	}
	fmt.Println("PASS MR1 owner still sees hidden achievement")

	appView, err := bobClient.requestStats(900, alice.SteamID)
	if err != nil {
		return err
	}
	if err := expectAchievements(appView, "other_game_win"); err != nil {
		return fmt.Errorf("appid-filtered stats: %w", err)
	}
	fmt.Println("PASS MR1 StatsGet appid filter")

	status, body, err = apiCall(client, http.MethodGet,
		fmt.Sprintf("%s/api/v1/users/%d/stats", adminURL, alice.SteamID), adminToken, nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("admin stats after filters = %d: %s (%v)", status, body, err)
	}
	stats = state.UserStats{}
	if err := json.Unmarshal(body, &stats); err != nil {
		return fmt.Errorf("decode admin stats: %w", err)
	}
	adminNames := map[string]bool{}
	for _, a := range stats.Achievements {
		adminNames[a.ID] = true
	}
	if len(adminNames) != 3 || !adminNames["first_kill"] || !adminNames["hidden_relic"] || !adminNames["other_game_win"] {
		return fmt.Errorf("admin stats after filters = %s", body)
	}
	fmt.Println("PASS MR1 admin stats API still returns all apps and hidden achievements")
	return nil
}

func checkFriends(client *http.Client, base, token string, self, friend uint64, persona string) error {
	status, body, err := apiCall(client, http.MethodGet, base+"/api/v1/friends", token, nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("friends for %d = %d: %s (%v)", self, status, body, err)
	}
	var result struct {
		Friends []state.Presence `json:"friends"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	if len(result.Friends) != 1 {
		return fmt.Errorf("friends for %d = %s", self, body)
	}
	f := result.Friends[0]
	if f.SteamID != friend || f.PersonaName != persona || f.PersonaState != 1 {
		return fmt.Errorf("unexpected friend for %d: %+v", self, f)
	}
	return nil
}

func checkConnectString(client *http.Client, base, token string, friend uint64, want string) error {
	status, body, err := apiCall(client, http.MethodGet, base+"/api/v1/friends", token, nil)
	if err != nil || status != http.StatusOK {
		return fmt.Errorf("friends = %d: %s (%v)", status, body, err)
	}
	var result struct {
		Friends []state.Presence `json:"friends"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}
	for _, f := range result.Friends {
		if f.SteamID == friend && f.ConnectStr == want {
			return nil
		}
	}
	return fmt.Errorf("connect string %q not found for %d: %s", want, friend, body)
}

func apiCall(client *http.Client, method, url, token string, payload any) (int, []byte, error) {
	var reader io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, body, nil
}

func createAccount(client *http.Client, base, username, password, persona string) (state.Account, error) {
	payload, err := json.Marshal(map[string]string{
		"username": username, "password": password, "persona_name": persona,
	})
	if err != nil {
		return state.Account{}, err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/accounts", bytes.NewReader(payload))
	if err != nil {
		return state.Account{}, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "create-"+username)
	resp, err := client.Do(req)
	if err != nil {
		return state.Account{}, fmt.Errorf("create %s: %w", username, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return state.Account{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return state.Account{}, fmt.Errorf("create %s status %d: %s", username, resp.StatusCode, body)
	}
	var account state.Account
	if err := json.Unmarshal(body, &account); err != nil {
		return state.Account{}, err
	}
	return account, nil
}
