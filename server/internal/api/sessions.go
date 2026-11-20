package api

import (
	"net/http"

	"github.com/sapie/sapie/server/internal/auth"
)

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	token, ok := s.bearer(r)
	if !ok {
		s.unauthorized(w)
		return
	}
	if _, ok := s.authorizeUser(w, r); !ok {
		return
	}
	if err := s.store.RevokeSessionByHash(auth.HashToken(token)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) logoutAll(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	if _, err := s.store.RevokeAllSessions(steamID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	token, _ := s.bearer(r)
	sessions, err := s.store.SessionsOf(steamID, auth.HashToken(token))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (s *Server) revokeSession(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	revoked, err := s.store.RevokeSession(steamID, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !revoked {
		notFound(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
