// Package api is the loopback admin HTTP API.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/state"
)

type cachedResponse struct {
	status int
	body   []byte
}

type Server struct {
	store *state.Store
	token string

	mu   sync.Mutex
	idem map[string]cachedResponse
}

func New(store *state.Store, token string) *Server {
	return &Server{store: store, token: token, idem: make(map[string]cachedResponse)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/accounts", s.listAccounts)
	mux.HandleFunc("POST /api/v1/accounts", s.createAccount)
	mux.HandleFunc("DELETE /api/v1/accounts/{id}", s.deleteAccount)
	return mux
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

func (s *Server) authorized(w http.ResponseWriter, r *http.Request) bool {
	header := r.Header.Get("Authorization")
	token, ok := strings.CutPrefix(header, "Bearer ")
	// An empty configured token disables admin access rather than opening it up.
	if !ok || s.token == "" || !auth.Equal(token, s.token) {
		w.Header().Set("WWW-Authenticate", "Bearer")
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
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
	key := r.Header.Get("Idempotency-Key")
	if key != "" {
		s.mu.Lock()
		if cached, ok := s.idem[key]; ok {
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(cached.status)
			w.Write(cached.body)
			return
		}
		defer s.mu.Unlock()
	}

	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		PersonaName string `json:"persona_name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if req.PersonaName == "" {
		req.PersonaName = req.Username
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "cannot hash password", http.StatusInternalServerError)
		return
	}
	if _, err := s.store.CreateAccount(req.Username, hash, req.PersonaName); err != nil {
		http.Error(w, "account creation failed", http.StatusConflict)
		return
	}
	account, err := s.store.AccountByUsername(req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body := writeJSON(w, http.StatusCreated, account.Account)
	if key != "" {
		s.idem[key] = cachedResponse{status: http.StatusCreated, body: body}
	}
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
