package api

import (
	"encoding/json"
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

// changePassword verifies the old password, replaces the hash and revokes
// every session except the one that made the change.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	token, _ := s.bearer(r)
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.NewPassword == "" {
		http.Error(w, "new_password is required", http.StatusBadRequest)
		return
	}
	hash, err := s.store.PasswordHashOf(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !auth.VerifyPassword(hash, req.OldPassword) {
		s.unauthorized(w)
		return
	}
	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "cannot hash password", http.StatusInternalServerError)
		return
	}
	if err := s.store.ChangePassword(steamID, auth.HashToken(token), newHash); err != nil {
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
