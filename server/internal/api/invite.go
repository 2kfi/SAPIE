package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/sapie/sapie/server/internal/state"
)

const inviteLifetime = time.Hour

var (
	errInviteLink    = errors.New("invalid invite link")
	errInviteExpired = errors.New("invite link expired")
)

// inviteLink signs the lobby and expiry so the link cannot be pointed at
// another lobby or extended. The admin token is the server-side signing secret.
func (s *Server) inviteLink(lobbyID uint64, expires int64) string {
	query := url.Values{}
	query.Set("exp", strconv.FormatInt(expires, 10))
	query.Set("sig", s.inviteSignature(lobbyID, expires))
	return (&url.URL{
		Scheme:   "sapie",
		Host:     "join",
		Path:     "/" + strconv.FormatUint(lobbyID, 10),
		RawQuery: query.Encode(),
	}).String()
}

func (s *Server) inviteSignature(lobbyID uint64, expires int64) string {
	mac := hmac.New(sha256.New, []byte(s.token))
	fmt.Fprintf(mac, "%d:%d", lobbyID, expires)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// resolveInvite returns the lobby the link was issued for. An empty signing
// secret is rejected so a misconfigured server cannot issue forgeable links.
func (s *Server) resolveInvite(raw string, now time.Time) (uint64, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "sapie" || u.Host != "join" || u.User != nil || u.Fragment != "" {
		return 0, errInviteLink
	}
	rawID := strings.TrimPrefix(u.Path, "/")
	if !strings.HasPrefix(u.Path, "/") || rawID == "" || strings.Contains(rawID, "/") {
		return 0, errInviteLink
	}
	lobbyID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil {
		return 0, errInviteLink
	}
	expires, err := strconv.ParseInt(u.Query().Get("exp"), 10, 64)
	if err != nil {
		return 0, errInviteLink
	}
	signature := u.Query().Get("sig")
	if s.token == "" || subtle.ConstantTimeCompare([]byte(signature), []byte(s.inviteSignature(lobbyID, expires))) != 1 {
		return 0, errInviteLink
	}
	if now.Unix() >= expires {
		return 0, errInviteExpired
	}
	return lobbyID, nil
}

func (s *Server) lobbyInvite(w http.ResponseWriter, r *http.Request) {
	steamID, ok := s.authorizeUser(w, r)
	if !ok {
		return
	}
	if s.token == "" {
		http.Error(w, "invites unavailable", http.StatusServiceUnavailable)
		return
	}
	lobbyID, err := strconv.ParseUint(r.PathValue("lobby_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid lobby id", http.StatusBadRequest)
		return
	}
	lobby, err := s.store.Lobby(lobbyID)
	if err != nil {
		if state.IsNotFound(err) {
			http.Error(w, "lobby not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	member := false
	for _, id := range lobby.Members {
		if id == steamID {
			member = true
			break
		}
	}
	if !member {
		http.Error(w, "lobby not found", http.StatusNotFound)
		return
	}
	if !lobby.Joinable {
		http.Error(w, "lobby is not joinable", http.StatusConflict)
		return
	}
	expires := time.Now().Add(inviteLifetime).Unix()
	writeJSON(w, http.StatusOK, map[string]any{
		"url":        s.inviteLink(lobbyID, expires),
		"expires_at": expires,
	})
}
