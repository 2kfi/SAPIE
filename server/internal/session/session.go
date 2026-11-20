// Package session owns the TCP accept loop and authenticated connections.
package session

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/router"
	"github.com/sapie/sapie/server/internal/state"
)

const (
	loginTimeout   = 10 * time.Second
	pingInterval   = 30 * time.Second
	readTimeout    = 90 * time.Second
	tokenLifetime  = 24 * time.Hour
	ticketLifetime = 10 * time.Minute
)

type Session struct {
	steamID      uint64
	personaName  string
	conn         net.Conn
	writeMu      sync.Mutex
	done         chan struct{}
	closeOnce    sync.Once
	ticketNumber uint32 // owned by the frame loop
}

func (c *Session) Send(frame *protocol.Frame) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return protocol.WriteFrame(c.conn, frame)
}

func (c *Session) stop() { c.closeOnce.Do(func() { close(c.done) }) }

type Server struct {
	store   *state.Store
	limiter *auth.Limiter
	router  *router.Router

	mu    sync.RWMutex
	peers map[uint64]*Session
}

func New(store *state.Store, limiter *auth.Limiter) *Server {
	s := &Server{store: store, limiter: limiter, peers: make(map[uint64]*Session)}
	s.router = router.NewWithResolver(s, store)
	return s
}

func (s *Server) Router() *router.Router { return s.router }

func (s *Server) Deliver(to uint64, frame *protocol.Frame) { s.router.Deliver(to, frame) }

func (s *Server) IsOnline(steamID uint64) bool {
	_, ok := s.PeerFor(steamID)
	return ok
}

func (s *Server) PeerFor(id uint64) (router.Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peer, ok := s.peers[id]
	return peer, ok
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		s.CloseAll()
		ln.Close()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go s.handle(conn)
	}
}

func (s *Server) CloseAll() {
	s.mu.RLock()
	sessions := make([]*Session, 0, len(s.peers))
	for _, peer := range s.peers {
		sessions = append(sessions, peer)
	}
	s.mu.RUnlock()
	for _, peer := range sessions {
		peer.conn.Close()
	}
}

func (s *Server) handle(conn net.Conn) {
	session := &Session{conn: conn, done: make(chan struct{})}
	defer conn.Close()
	defer session.stop()

	conn.SetReadDeadline(time.Now().Add(loginTimeout))
	var head [4]byte
	if _, err := io.ReadFull(conn, head[:]); err != nil {
		return
	}
	if looksLikeHTTP(head[:]) {
		rejectHTTP(conn)
		return
	}
	frame, err := protocol.ReadFrame(io.MultiReader(bytes.NewReader(head[:]), conn))
	if err != nil {
		return
	}
	login, ok := frame.Payload.(*protocol.Frame_Login)
	if !ok {
		return // first frame must be LoginRequest; anything else closes the connection
	}

	ip := hostFromAddr(conn.RemoteAddr())
	if !s.limiter.Allow(ip) {
		s.sendLoginResult(session, protocol.LoginResponse_BAD_CREDENTIALS, 0, nil, nil, nil)
		return
	}
	account, err := s.store.AccountByUsername(login.Login.GetUsername())
	if err != nil || !auth.VerifyPassword(account.PasswordHash, login.Login.GetPassword()) {
		s.limiter.RecordFailure(ip)
		s.sendLoginResult(session, protocol.LoginResponse_BAD_CREDENTIALS, 0, nil, nil, nil)
		return
	}

	token, err := auth.NewToken()
	if err != nil {
		log.Printf("session: token generation: %v", err)
		return
	}
	if err := s.store.CreateSession(account.SteamID, auth.HashToken(token), time.Now().Add(tokenLifetime)); err != nil {
		log.Printf("session: store session: %v", err)
		return
	}
	session.steamID = account.SteamID
	session.personaName = account.PersonaName
	s.register(session)
	defer s.disconnect(session)
	go s.heartbeat(session)

	s.sendLoginResult(session, protocol.LoginResponse_OK, account.SteamID,
		[]byte(account.PersonaName), []byte(token), s.friendSnapshots(account.SteamID))
	s.pushPresence(session, true)

	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		frame, err = protocol.ReadFrame(conn)
		if err != nil {
			return
		}
		switch payload := frame.Payload.(type) {
		case *protocol.Frame_Ping:
			_ = session.Send(&protocol.Frame{Payload: &protocol.Frame_Pong{Pong: true}})
		case *protocol.Frame_Steam:
			s.handleSteam(session, payload.Steam)
		case *protocol.Frame_TicketRequest:
			var nonce [32]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				log.Printf("session %d: ticket nonce: %v", session.steamID, err)
				continue
			}
			session.ticketNumber++
			appid := payload.TicketRequest.GetAppid()
			ticket := state.TicketHash(session.steamID, appid, nonce[:])
			if err := s.store.IssueTicket(session.steamID, appid, session.ticketNumber, nonce[:], time.Now().Add(ticketLifetime)); err != nil {
				log.Printf("session %d: store ticket: %v", session.steamID, err)
				continue
			}
			response := &protocol.TicketResponse{Ticket: ticket, Number: session.ticketNumber}
			if err := session.Send(&protocol.Frame{Payload: &protocol.Frame_TicketResponse{TicketResponse: response}}); err != nil {
				log.Printf("session %d: ticket response: %v", session.steamID, err)
			}
		case *protocol.Frame_StatsGet:
			s.handleStatsGet(session, payload.StatsGet)
		case *protocol.Frame_AchievementUnlock:
			s.handleAchievementUnlock(session, payload.AchievementUnlock)
		default:
			log.Printf("session %d: %T ignored (no-op)", session.steamID, frame.Payload)
		}
	}
}

func (s *Server) friendSnapshots(steamID uint64) []*protocol.FriendSnapshot {
	friends, err := s.store.FriendsPresence(steamID)
	if err != nil {
		log.Printf("session %d: friends: %v", steamID, err)
		return nil
	}
	snapshots := make([]*protocol.FriendSnapshot, 0, len(friends))
	for _, friend := range friends {
		personaState := []byte("0")
		if s.IsOnline(friend.SteamID) {
			personaState = []byte("1")
		}
		friend.RichPresence[state.PersonaStateKey] = personaState
		snapshots = append(snapshots, &protocol.FriendSnapshot{
			SteamId:      friend.SteamID,
			PersonaName:  []byte(friend.PersonaName),
			Appid:        friend.Appid,
			LobbyId:      friend.LobbyID,
			RichPresence: friend.RichPresence,
		})
	}
	return snapshots
}

func (s *Server) pushPresence(session *Session, online bool) {
	presence, err := s.store.PresenceOf(session.steamID)
	if err != nil {
		log.Printf("session %d: presence: %v", session.steamID, err)
		return
	}
	personaState := []byte("0")
	if online {
		personaState = []byte("1")
	}
	presence.RichPresence[state.PersonaStateKey] = personaState
	s.router.Route(session.steamID, &protocol.Common_Message{
		Messages: &protocol.Common_Message_Friend{Friend: &protocol.Friend{
			Id:           session.steamID,
			Name:         []byte(session.personaName),
			RichPresence: presence.RichPresence,
			Appid:        presence.Appid,
			LobbyId:      presence.LobbyID,
		}},
	})
}

func (s *Server) handleSteam(session *Session, msg *protocol.Common_Message) {
	if msg == nil {
		return
	}
	switch payload := msg.Messages.(type) {
	case *protocol.Common_Message_Friend:
		friend := payload.Friend
		if friend == nil {
			return
		}
		friend.Id = session.steamID
		if err := s.store.UpdatePresence(session.steamID, string(friend.Name), friend.Appid, friend.LobbyId, friend.RichPresence); err != nil {
			log.Printf("session %d: store presence: %v", session.steamID, err)
			return
		}
	case *protocol.Common_Message_FriendMessages:
		if invite := payload.FriendMessages; invite != nil &&
			invite.GetType() == protocol.Friend_Messages_GAME_INVITE && len(invite.GetConnectStr()) > 0 {
			if err := s.store.SetConnectStr(session.steamID, string(invite.GetConnectStr())); err != nil {
				log.Printf("session %d: store connect string: %v", session.steamID, err)
			}
		}
	}
	s.router.Route(session.steamID, msg)
}

func (s *Server) handleStatsGet(session *Session, req *protocol.StatsGet) {
	if req == nil {
		return
	}
	steamID := req.GetSteamId()
	if steamID == 0 {
		steamID = session.steamID
	}
	stats, err := s.store.StatsOf(steamID)
	if err != nil {
		log.Printf("session %d: stats for %d: %v", session.steamID, steamID, err)
		return
	}
	appid := req.GetAppid()
	owner := steamID == session.steamID
	names := map[string]bool{}
	for _, name := range req.GetNames() {
		names[name] = true
	}
	response := &protocol.StatsResponse{SteamId: steamID}
	for _, stat := range stats.Stats {
		if appid != 0 && stat.Appid != appid {
			continue
		}
		if len(names) > 0 && !names[stat.Name] {
			continue
		}
		response.Stats = append(response.Stats, &protocol.StatValue{Name: stat.Name, Value: stat.Value})
	}
	for _, a := range stats.Achievements {
		if appid != 0 && a.Appid != appid {
			continue
		}
		// Hidden definitions are private to the owner.
		if a.Hidden && !owner {
			continue
		}
		response.Achievements = append(response.Achievements, &protocol.AchievementState{
			Name: a.ID, Achieved: a.Achieved,
			UnlockTime: uint32(a.UnlockTime), UnlockTimeUnix: a.UnlockTime,
		})
	}
	if err := session.Send(&protocol.Frame{Payload: &protocol.Frame_StatsResponse{StatsResponse: response}}); err != nil {
		log.Printf("session %d: stats response: %v", session.steamID, err)
	}
}

func (s *Server) handleAchievementUnlock(session *Session, unlock *protocol.AchievementUnlock) {
	if unlock == nil || unlock.GetDef().GetId() == "" {
		return
	}
	def := unlock.GetDef()
	stored, err := s.store.UnlockAchievement(session.steamID, state.Achievement{
		Appid: unlock.GetAppid(), ID: def.GetId(), DisplayName: def.GetDisplayName(),
		Description: def.GetDescription(), Hidden: def.GetHidden(), Icon: def.GetIcon(),
	})
	if err != nil {
		log.Printf("session %d: unlock achievement: %v", session.steamID, err)
		return
	}
	// Hidden definitions stay canonical in storage but are never pushed to
	// friends or lobby mates; the owner and admin still see them on query.
	if stored.Hidden {
		return
	}
	push := &protocol.Common_Message{
		SourceId: session.steamID,
		Messages: &protocol.Common_Message_AchievementUnlock{AchievementUnlock: &protocol.AchievementUnlock{
			SteamId: session.steamID, Appid: stored.Appid, UnlockTime: stored.UnlockTime,
			Def: &protocol.AchievementDef{
				Id: stored.ID, DisplayName: stored.DisplayName, Description: stored.Description,
				Hidden: stored.Hidden, Icon: stored.Icon,
			},
		}},
	}
	ids := map[uint64]struct{}{}
	friends, err := s.store.FriendsOf(session.steamID)
	if err != nil {
		log.Printf("session %d: unlock friends: %v", session.steamID, err)
	}
	for _, id := range friends {
		ids[id] = struct{}{}
	}
	lobbies, err := s.store.LobbiesOf(session.steamID)
	if err != nil {
		log.Printf("session %d: unlock lobbies: %v", session.steamID, err)
	}
	for _, lobbyID := range lobbies {
		members, err := s.store.LobbyMembers(lobbyID)
		if err != nil {
			log.Printf("session %d: unlock lobby %d members: %v", session.steamID, lobbyID, err)
			continue
		}
		for _, id := range members {
			ids[id] = struct{}{}
		}
	}
	for id := range ids {
		if id == session.steamID {
			continue
		}
		push.DestId = id
		s.router.Route(session.steamID, push)
	}
}

func (s *Server) sendLoginResult(session *Session, status protocol.LoginResponse_Status, id uint64, persona, token []byte, friends []*protocol.FriendSnapshot) {
	response := &protocol.LoginResponse{
		Status:       status,
		SteamId:      id,
		PersonaName:  persona,
		SessionToken: token,
		Friends:      friends,
	}
	if err := session.Send(&protocol.Frame{Payload: &protocol.Frame_LoginResponse{LoginResponse: response}}); err != nil {
		log.Printf("session: login response: %v", err)
	}
}

func (s *Server) heartbeat(session *Session) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-session.done:
			return
		case <-ticker.C:
			if err := session.Send(&protocol.Frame{Payload: &protocol.Frame_Ping{Ping: true}}); err != nil {
				return
			}
		}
	}
}

func (s *Server) register(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if old, ok := s.peers[session.steamID]; ok {
		old.conn.Close()
	}
	s.peers[session.steamID] = session
}

func (s *Server) unregister(session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.peers[session.steamID] == session {
		delete(s.peers, session.steamID)
	}
}

func (s *Server) disconnect(session *Session) {
	s.unregister(session)
	if peer, ok := s.PeerFor(session.steamID); ok && peer != session {
		return // a newer session already pushed its own presence
	}
	s.pushPresence(session, false)
}

func looksLikeHTTP(head []byte) bool {
	return bytes.HasPrefix(head, []byte("GET ")) || bytes.HasPrefix(head, []byte("POST")) || bytes.HasPrefix(head, []byte("HEAD"))
}

func rejectHTTP(conn net.Conn) {
	body := "This port is SAPIE's binary game protocol, not HTTP. Open the web UI on port 8081 instead (see --ui-listen).\n"
	fmt.Fprintf(conn, "HTTP/1.1 400 Bad Request\r\nContent-Type: text/plain; charset=utf-8\r\nConnection: close\r\nContent-Length: %d\r\n\r\n%s", len(body), body)
	// Draining the unread request avoids a TCP reset that would discard the reply.
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	io.Copy(io.Discard, conn)
}

func hostFromAddr(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
