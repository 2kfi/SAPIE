// Package api is the loopback admin and user HTTP API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/state"
)

const (
	sessionTokenLifetime = 24 * time.Hour
	maxIdemKeys          = 256
)

type cachedResponse struct {
	status int
	body   []byte
}

type idemCall struct {
	done     chan struct{}
	response cachedResponse
}

// Notifier delivers game frames and reports live presence to the API.
type Notifier interface {
	Deliver(uint64, *protocol.Frame)
	IsOnline(uint64) bool
}

type Server struct {
	store         *state.Store
	token         string
	limiter       *auth.Limiter
	notifier      Notifier
	allowRegister bool

	mu   sync.Mutex
	idem map[string]*idemCall
}

func New(store *state.Store, token string, limiter *auth.Limiter, notifier Notifier) *Server {
	return &Server{
		store: store, token: token, limiter: limiter,
		notifier: notifier, idem: make(map[string]*idemCall),
	}
}

func (s *Server) SetAllowRegister(enabled bool) { s.allowRegister = enabled }

func (s *Server) idemStart(key string) (*idemCall, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if call, ok := s.idem[key]; ok {
		return call, false
	}
	call := &idemCall{done: make(chan struct{})}
	s.idem[key] = call
	for len(s.idem) > maxIdemKeys {
		freed := false
		for k, other := range s.idem {
			select {
			case <-other.done:
				delete(s.idem, k)
				freed = true
			default:
			}
			if freed {
				break
			}
		}
		if !freed {
			break // all entries are in flight; they cannot be replayed yet
		}
	}
	return call, true
}

func (s *Server) idemFinish(key string, call *idemCall, resp cachedResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call.response = resp
	close(call.done)
	if resp.status == 0 {
		delete(s.idem, key)
	}
}

// idempotent caches only successful (status-bearing) responses. A failed call
// can be retried with the same key.
func (s *Server) idempotent(key string, w http.ResponseWriter, fn func(http.ResponseWriter) cachedResponse) {
	if key == "" {
		fn(w)
		return
	}
	for {
		call, leader := s.idemStart(key)
		if !leader {
			<-call.done
			if call.response.status != 0 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(call.response.status)
				w.Write(call.response.body)
				return
			}
			continue
		}
		s.idemFinish(key, call, fn(w))
		return
	}
}

func userIdeKey(steamID uint64, key string) string {
	return strconv.FormatUint(steamID, 10) + ":" + key
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"allow_register": s.allowRegister})
	})
	mux.HandleFunc("POST /api/v1/register", s.register)
	mux.HandleFunc("POST /api/v1/login", s.login)
	mux.HandleFunc("POST /api/v1/password", s.changePassword)
	mux.HandleFunc("GET /api/v1/friends", s.listFriends)
	mux.HandleFunc("POST /api/v1/friends/{steam_id}/invite", s.inviteFriend)
	mux.HandleFunc("GET /api/v1/friends/requests", s.listFriendRequests)
	mux.HandleFunc("POST /api/v1/friends/requests", s.sendFriendRequest)
	mux.HandleFunc("POST /api/v1/friends/requests/{id}/accept", s.acceptFriendRequest)
	mux.HandleFunc("POST /api/v1/friends/requests/{id}/decline", s.declineFriendRequest)
	mux.HandleFunc("DELETE /api/v1/friends/{a}/{b}", s.deleteFriendRoute)
	mux.HandleFunc("DELETE /api/v1/friends/{steam_id}", s.removeFriend)
	mux.HandleFunc("POST /api/v1/friends/{steam_id}/block", s.blockFriend)
	mux.HandleFunc("GET /api/v1/friends/blocked", s.listBlocked)
	mux.HandleFunc("POST /api/v1/tickets/verify", s.verifyTicket)
	mux.HandleFunc("GET /api/v1/lobbies", s.listLobbies)
	mux.HandleFunc("POST /api/v1/lobbies", s.createLobby)
	mux.HandleFunc("POST /api/v1/lobbies/{lobby_id}/join", s.joinLobby)
	mux.HandleFunc("POST /api/v1/lobbies/{lobby_id}/invite", s.lobbyInvite)
	mux.HandleFunc("GET /api/v1/users", s.listUsers)
	mux.HandleFunc("GET /api/v1/users/search", s.searchUsers)
	mux.HandleFunc("GET /api/v1/users/{steam_id}/stats", s.userStats)
	mux.HandleFunc("POST /api/v1/logout", s.logout)
	mux.HandleFunc("POST /api/v1/logout-all", s.logoutAll)
	mux.HandleFunc("GET /api/v1/sessions", s.listSessions)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", s.revokeSession)
	mux.HandleFunc("GET /api/v1/accounts", s.listAccounts)
	mux.HandleFunc("POST /api/v1/accounts", s.createAccount)
	mux.HandleFunc("DELETE /api/v1/accounts/{id}", s.deleteAccount)
	return withCORS(mux)
}

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	httpServer := &http.Server{Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		httpServer.Shutdown(context.Background())
	}()
	err := httpServer.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		return nil
	}
	return err
}

func (s *Server) isAdmin(r *http.Request) bool {
	token, ok := s.bearer(r)
	return ok && s.token != "" && auth.Equal(token, s.token)
}

func (s *Server) authorized(w http.ResponseWriter, r *http.Request) bool {
	// An empty configured token disables admin access rather than opening it up.
	if !s.isAdmin(r) {
		s.unauthorized(w)
		return false
	}
	return true
}

func (s *Server) bearer(r *http.Request) (string, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return token, ok && token != ""
}

func (s *Server) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func (s *Server) authorizeUser(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	token, ok := s.bearer(r)
	if !ok {
		s.unauthorized(w)
		return 0, false
	}
	steamID, err := s.store.ResolveSession(auth.HashToken(token))
	if err != nil {
		s.unauthorized(w)
		return 0, false
	}
	return steamID, true
}

func (s *Server) authorizeLobbies(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	token, ok := s.bearer(r)
	if ok && s.token != "" && auth.Equal(token, s.token) {
		return 0, true
	}
	return s.authorizeUser(w, r)
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.allowRegister {
		http.Error(w, "registration is disabled", http.StatusForbidden)
		return
	}
	s.createAccountNow(w, r)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.limiter == nil {
		http.Error(w, "login unavailable", http.StatusServiceUnavailable)
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	if !s.limiter.Allow(ip) {
		s.unauthorized(w)
		return
	}
	var req struct {
		Username     string `json:"username"`
		Password     string `json:"password"`
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	now := time.Now()
	token, err := auth.NewToken()
	if err != nil {
		http.Error(w, "cannot issue token", http.StatusInternalServerError)
		return
	}
	var account *state.AuthAccount
	if req.SessionToken != "" {
		steamID, _, err := s.store.RotateSession(auth.HashToken(req.SessionToken), auth.HashToken(token), now.Add(sessionTokenLifetime))
		if err != nil {
			s.limiter.RecordFailure(ip)
			s.unauthorized(w)
			return
		}
		stored, err := s.store.AccountBySteamID(steamID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		account = &state.AuthAccount{Account: stored}
	} else {
		account, err = s.store.AccountByUsername(req.Username)
		if err != nil || !auth.VerifyPassword(account.PasswordHash, req.Password) {
			s.limiter.RecordFailure(ip)
			s.unauthorized(w)
			return
		}
		if err := s.store.DeleteExpiredSessionsFor(account.SteamID, now); err != nil {
			http.Error(w, "cannot clean sessions", http.StatusInternalServerError)
			return
		}
		if err := s.store.CreateSession(account.SteamID, auth.HashToken(token), now.Add(sessionTokenLifetime)); err != nil {
			http.Error(w, "cannot store session", http.StatusInternalServerError)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_token": token,
		"steam_id":      account.SteamID,
		"persona_name":  account.PersonaName,
	})
}

func (s *Server) listFriends(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	friends, err := s.store.FriendsPresence(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for i := range friends {
		if s.notifier != nil && s.notifier.IsOnline(friends[i].SteamID) {
			friends[i].PersonaState = 1
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"friends": friends})
}

func (s *Server) inviteFriend(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	target, err := strconv.ParseUint(r.PathValue("steam_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid steam id", http.StatusBadRequest)
		return
	}
	var req struct {
		ConnectStr string `json:"connect_str"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	friends, err := s.store.FriendsOf(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	friend := false
	for _, id := range friends {
		if id == target {
			friend = true
			break
		}
	}
	if !friend {
		http.Error(w, "friend not found", http.StatusNotFound)
		return
	}
	if err := s.store.SetConnectStr(steamID, req.ConnectStr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.notifier != nil {
		s.notifier.Deliver(target, &protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
			SourceId: steamID,
			DestId:   target,
			Messages: &protocol.Common_Message_FriendMessages{FriendMessages: &protocol.Friend_Messages{
				Type:       protocol.Friend_Messages_GAME_INVITE,
				InviteData: &protocol.Friend_Messages_ConnectStr{ConnectStr: []byte(req.ConnectStr)},
			}},
		}}})
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) verifyTicket(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	var req struct {
		Ticket  []byte `json:"ticket"`
		SteamID uint64 `json:"steam_id"`
		Appid   uint32 `json:"appid"`
		Number  uint32 `json:"number"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	valid, err := s.store.VerifyTicket(req.Ticket, req.SteamID, req.Appid, req.Number)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": valid})
}

func (s *Server) listLobbies(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.authorizeLobbies(w, r); !ok {
		return
	}
	lobbies, err := s.store.Lobbies()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lobbies": lobbies})
}

func (s *Server) createLobby(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Appid       uint32 `json:"appid"`
		Type        uint32 `json:"type"`
		MemberLimit uint32 `json:"member_limit"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.MemberLimit == 0 {
		http.Error(w, "member_limit must be positive", http.StatusBadRequest)
		return
	}
	lobbyID, err := s.store.CreateLobby(steamID, req.Appid, req.Type, req.MemberLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.pushLobby(lobbyID, steamID)
	writeJSON(w, http.StatusCreated, map[string]any{"lobby_id": lobbyID})
}

func (s *Server) joinLobby(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	lobbyID, err := strconv.ParseUint(r.PathValue("lobby_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid lobby id", http.StatusBadRequest)
		return
	}
	var req struct {
		Invite string `json:"invite"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Invite != "" {
		invited, err := s.resolveInvite(req.Invite, time.Now())
		switch {
		case errors.Is(err, errInviteLink):
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		case errors.Is(err, errInviteExpired):
			http.Error(w, err.Error(), http.StatusGone)
			return
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		case invited != lobbyID:
			http.Error(w, "invite link is for a different lobby", http.StatusBadRequest)
			return
		}
	}
	presence, err := s.store.PresenceOf(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.JoinLobby(lobbyID, steamID, presence.Appid); err != nil {
		switch {
		case state.IsNotFound(err):
			http.Error(w, "lobby not found", http.StatusNotFound)
		case errors.Is(err, state.ErrLobbyUnavailable):
			http.Error(w, "lobby unavailable", http.StatusConflict)
		case errors.Is(err, state.ErrPairBlocked):
			http.Error(w, "cannot join this lobby", http.StatusForbidden)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	s.pushLobby(lobbyID, steamID)
	writeJSON(w, http.StatusOK, map[string]any{"lobby_id": lobbyID})
}

func (s *Server) pushLobby(lobbyID, from uint64) {
	if s.notifier == nil {
		return
	}
	lobby, err := s.store.Lobby(lobbyID)
	if err != nil {
		return
	}
	members := make([]*protocol.Lobby_Member, 0, len(lobby.Members))
	for _, id := range lobby.Members {
		members = append(members, &protocol.Lobby_Member{Id: id})
	}
	frame := &protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
		Messages: &protocol.Common_Message_Lobby{Lobby: &protocol.Lobby{
			RoomId:      lobby.LobbyID,
			Owner:       lobby.Owner,
			Appid:       lobby.Appid,
			MemberLimit: lobby.MemberLimit,
			Type:        lobby.Type,
			Joinable:    lobby.Joinable,
			Members:     members,
		}},
	}}}
	for _, id := range lobby.Members {
		blocked, err := s.store.IsBlocked(from, id)
		if err != nil {
			log.Printf("lobby push block check %d -> %d: %v", from, id, err)
			continue
		}
		if blocked {
			continue
		}
		s.notifier.Deliver(id, frame)
	}
}

// pushFriendSnapshot sends the current friend list as one Friend frame per
// friend. The wire contract has no snapshot message, so a reload is a sequence
// of existing Friend pushes.
func (s *Server) pushFriendSnapshot(ids ...uint64) {
	if s.notifier == nil {
		return
	}
	for _, id := range ids {
		friends, err := s.store.FriendsPresence(id)
		if err != nil {
			log.Printf("snapshot for %d: %v", id, err)
			continue
		}
		for _, friend := range friends {
			presenceState := []byte("0")
			if s.notifier.IsOnline(friend.SteamID) {
				presenceState = []byte("1")
			}
			friend.RichPresence[state.PersonaStateKey] = presenceState
			s.notifier.Deliver(id, &protocol.Frame{Payload: &protocol.Frame_Steam{Steam: &protocol.Common_Message{
				SourceId: friend.SteamID,
				Messages: &protocol.Common_Message_Friend{Friend: &protocol.Friend{
					Id:           friend.SteamID,
					Name:         []byte(friend.PersonaName),
					Appid:        friend.Appid,
					LobbyId:      friend.LobbyID,
					RichPresence: friend.RichPresence,
				}},
			}}})
		}
	}
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	accounts, err := s.store.ListAccounts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	users := make([]state.Presence, 0, len(accounts))
	for _, account := range accounts {
		presence, err := s.store.PresenceOf(account.SteamID)
		if err != nil {
			continue
		}
		if s.notifier != nil && s.notifier.IsOnline(account.SteamID) {
			presence.PersonaState = 1
		}
		users = append(users, presence)
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) userStats(w http.ResponseWriter, r *http.Request) {
	steamID, err := strconv.ParseUint(r.PathValue("steam_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid steam id", http.StatusBadRequest)
		return
	}
	if !s.isAdmin(r) {
		caller, ok := s.authorizeUser(w, r)
		if !ok {
			return
		}
		if caller != steamID {
			s.unauthorized(w)
			return
		}
	}
	stats, err := s.store.StatsOf(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) listAccounts(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	accounts, err := s.store.ListAccounts()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	s.idempotent(r.Header.Get("Idempotency-Key"), w, func(w http.ResponseWriter) cachedResponse {
		return s.createAccountNow(w, r)
	})
}

func (s *Server) createAccountNow(w http.ResponseWriter, r *http.Request) cachedResponse {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		PersonaName string `json:"persona_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return cachedResponse{}
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return cachedResponse{}
	}
	if req.PersonaName == "" {
		req.PersonaName = req.Username
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "cannot hash password", http.StatusInternalServerError)
		return cachedResponse{}
	}
	if _, err := s.store.CreateAccount(req.Username, hash, req.PersonaName); err != nil {
		http.Error(w, "account creation failed", http.StatusConflict)
		return cachedResponse{}
	}
	account, err := s.store.AccountByUsername(req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return cachedResponse{}
	}
	return cachedResponse{status: http.StatusCreated, body: writeJSON(w, http.StatusCreated, account.Account)}
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(w, r) {
		return
	}
	steamID, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid steam id", http.StatusBadRequest)
		return
	}
	if err := s.store.DeleteAccount(steamID); err != nil {
		if state.IsNotFound(err) {
			http.Error(w, "account not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := allowedOrigin(r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			w.Header().Set("Access-Control-Max-Age", "600")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1":
		return u.Scheme + "://" + u.Host
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, value any) []byte {
	body, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
	return body
}
