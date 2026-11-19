// Package session owns the TCP accept loop and authenticated connections.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
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
	loginTimeout  = 10 * time.Second
	pingInterval  = 30 * time.Second
	readTimeout   = 90 * time.Second
	tokenLifetime = 24 * time.Hour
)

type Session struct {
	steamID      uint64
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
	s.router = router.New(s)
	return s
}

func (s *Server) Router() *router.Router { return s.router }

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
	frame, err := protocol.ReadFrame(conn)
	if err != nil {
		return
	}
	login, ok := frame.Payload.(*protocol.Frame_Login)
	if !ok {
		return // first frame must be LoginRequest; anything else closes the connection
	}

	ip := hostFromAddr(conn.RemoteAddr())
	if !s.limiter.Allow(ip) {
		s.sendLoginResult(session, protocol.LoginResponse_BAD_CREDENTIALS, 0, nil, nil)
		return
	}
	account, err := s.store.AccountByUsername(login.Login.GetUsername())
	if err != nil || !auth.VerifyPassword(account.PasswordHash, login.Login.GetPassword()) {
		s.limiter.RecordFailure(ip)
		s.sendLoginResult(session, protocol.LoginResponse_BAD_CREDENTIALS, 0, nil, nil)
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
	s.register(session)
	defer s.unregister(session)
	go s.heartbeat(session)

	s.sendLoginResult(session, protocol.LoginResponse_OK, account.SteamID,
		[]byte(account.PersonaName), []byte(token))

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
			s.router.Route(session.steamID, payload.Steam)
		case *protocol.Frame_TicketRequest:
			ticket, err := newTicket(session.steamID, payload.TicketRequest.GetAppid())
			if err != nil {
				log.Printf("session %d: ticket generation: %v", session.steamID, err)
				continue
			}
			session.ticketNumber++
			response := &protocol.TicketResponse{Ticket: ticket, Number: session.ticketNumber}
			if err := session.Send(&protocol.Frame{Payload: &protocol.Frame_TicketResponse{TicketResponse: response}}); err != nil {
				log.Printf("session %d: ticket response: %v", session.steamID, err)
			}
		default:
			log.Printf("session %d: %T ignored (M0 no-op)", session.steamID, frame.Payload)
		}
	}
}

// newTicket derives a 32-byte opaque ticket from a random nonce and the identity
// it was issued for. No record is kept, so binding cannot be verified yet.
func newTicket(steamID uint64, appid uint32) ([]byte, error) {
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	var input [44]byte
	binary.BigEndian.PutUint64(input[:8], steamID)
	binary.BigEndian.PutUint32(input[8:12], appid)
	copy(input[12:], nonce[:])
	sum := sha256.Sum256(input[:])
	return sum[:], nil
}

func (s *Server) sendLoginResult(session *Session, status protocol.LoginResponse_Status, id uint64, persona, token []byte) {
	response := &protocol.LoginResponse{
		Status:       status,
		SteamId:      id,
		PersonaName:  persona,
		SessionToken: token,
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

func hostFromAddr(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
