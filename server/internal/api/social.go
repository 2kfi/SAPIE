package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/state"
)

const (
	socialLimit       = 30
	socialWindow      = time.Hour
	maxRequestMessage = 280
)

func (s *Server) socialAllowed(steamID uint64) (bool, error) {
	bucket := auth.SocialBucket(steamID)
	allowed, err := s.limiter.AllowBucket(bucket, socialLimit, socialWindow)
	if err != nil || !allowed {
		return allowed, err
	}
	return true, s.limiter.RecordBucket(bucket)
}

func steamIDParam(w http.ResponseWriter, r *http.Request, name string) (uint64, bool) {
	id, err := strconv.ParseUint(r.PathValue(name), 10, 64)
	if err != nil {
		http.Error(w, "invalid steam id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func notFound(w http.ResponseWriter) {
	http.Error(w, "not found", http.StatusNotFound)
}

func idemKey(r *http.Request, steamID uint64) string {
	return userIdeKey(steamID, r.Header.Get("Idempotency-Key"))
}

func (s *Server) listFriendRequests(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	requests, err := s.store.FriendRequestsOf(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, requests)
}

func (s *Server) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	allowed, err := s.socialAllowed(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	var req struct {
		Username string `json:"username"`
		Message  string `json:"message"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Username == "" || len(req.Username) > 64 || len(req.Message) > maxRequestMessage {
		http.Error(w, "invalid username or message", http.StatusBadRequest)
		return
	}
	s.idempotent(idemKey(r, steamID), w, func(w http.ResponseWriter) cachedResponse {
		return s.sendFriendRequestNow(w, steamID, req.Username, strings.TrimSpace(req.Message))
	})
}

func (s *Server) sendFriendRequestNow(w http.ResponseWriter, from uint64, username, message string) cachedResponse {
	account, err := s.store.AccountByUsername(username)
	if err != nil {
		notFound(w) // unknown, self, blocked and existing friends are indistinguishable
		return cachedResponse{}
	}
	request, err := s.store.SendFriendRequest(from, account.SteamID, message)
	switch {
	case err == nil:
	case errors.Is(err, state.ErrRequestExists):
		http.Error(w, "request already exists", http.StatusConflict)
		return cachedResponse{}
	case errors.Is(err, state.ErrSelfRequest), errors.Is(err, state.ErrPairBlocked),
		errors.Is(err, state.ErrAlreadyFriends), errors.Is(err, sql.ErrNoRows):
		notFound(w)
		return cachedResponse{}
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return cachedResponse{}
	}
	body := struct {
		ID       int64  `json:"id"`
		Status   string `json:"status"`
		CreateAt int64  `json:"created_at"`
	}{request.ID, request.Status, request.CreatedAt}
	return cachedResponse{status: http.StatusCreated, body: writeJSON(w, http.StatusCreated, body)}
}

func (s *Server) requestID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid request id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func requestError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, state.ErrNotRecipient),
		errors.Is(err, state.ErrNotSender), errors.Is(err, state.ErrRequestInvalid):
		notFound(w)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	return true
}

func (s *Server) acceptFriendRequest(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	id, ok := s.requestID(w, r)
	if !ok {
		return
	}
	s.idempotent(idemKey(r, steamID), w, func(w http.ResponseWriter) cachedResponse {
		if err := s.store.AcceptFriendRequest(id, steamID); requestError(w, err) {
			return cachedResponse{}
		}
		request, err := s.store.FriendRequestByID(id, steamID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return cachedResponse{}
		}
		s.pushFriendSnapshot(steamID, request.FromSteamID)
		body := struct {
			Status string `json:"status"`
		}{"accepted"}
		return cachedResponse{status: http.StatusOK, body: writeJSON(w, http.StatusOK, body)}
	})
}

func (s *Server) declineFriendRequest(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	id, ok := s.requestID(w, r)
	if !ok {
		return
	}
	s.idempotent(idemKey(r, steamID), w, func(w http.ResponseWriter) cachedResponse {
		if err := s.store.DeclineFriendRequest(id, steamID); requestError(w, err) {
			return cachedResponse{}
		}
		body := struct {
			Status string `json:"status"`
		}{"declined"}
		return cachedResponse{status: http.StatusOK, body: writeJSON(w, http.StatusOK, body)}
	})
}

// deleteFriendRoute resolves the two DELETE shapes that Go's ServeMux cannot
// register separately: /friends/requests/{id} and /friends/{steam_id}/block.
func (s *Server) deleteFriendRoute(w http.ResponseWriter, r *http.Request) {
	a, b := r.PathValue("a"), r.PathValue("b")
	switch {
	case a == "requests":
		s.cancelFriendRequest(w, r, b)
	case b == "block":
		s.unblockFriend(w, r, a)
	default:
		notFound(w)
	}
}

func (s *Server) cancelFriendRequest(w http.ResponseWriter, r *http.Request, rawID string) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid request id", http.StatusBadRequest)
		return
	}
	if err := s.store.CancelFriendRequest(id, steamID); requestError(w, err) {
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) removeFriend(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	target, ok := steamIDParam(w, r, "steam_id")
	if !ok {
		return
	}
	if err := s.store.RemoveFriend(steamID, target); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			notFound(w)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.pushFriendSnapshot(steamID, target)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) blockFriend(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	target, ok := steamIDParam(w, r, "steam_id")
	if !ok {
		return
	}
	allowed, err := s.socialAllowed(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	s.idempotent(idemKey(r, steamID), w, func(w http.ResponseWriter) cachedResponse {
		if err := s.store.BlockUser(steamID, target); err != nil {
			if errors.Is(err, state.ErrSelfRequest) || errors.Is(err, sql.ErrNoRows) {
				notFound(w)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return cachedResponse{}
		}
		s.pushFriendSnapshot(steamID, target)
		w.WriteHeader(http.StatusNoContent)
		return cachedResponse{status: http.StatusNoContent}
	})
}

func (s *Server) unblockFriend(w http.ResponseWriter, r *http.Request, rawID string) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	target, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		http.Error(w, "invalid steam id", http.StatusBadRequest)
		return
	}
	if err := s.store.UnblockUser(steamID, target); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.pushFriendSnapshot(steamID, target)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listBlocked(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	users, err := s.store.BlockedUsers(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"blocked": users})
}

func (s *Server) searchUsers(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	allowed, err := s.socialAllowed(steamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	users, err := s.store.SearchUsers(steamID, r.URL.Query().Get("q"))
	if errors.Is(err, state.ErrSearchQuery) {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}
