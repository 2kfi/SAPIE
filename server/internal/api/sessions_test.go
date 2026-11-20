package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/state"
)

func TestSessionRoutes(t *testing.T) {
	_, handler, _, tokens, _ := newSocialAPI(t)
	aliceToken := tokens["alice"]

	path := "/api/v1/sessions"
	list := socialRequest(handler, http.MethodGet, path, aliceToken, "", "")
	var body struct {
		Sessions []state.SessionInfo `json:"sessions"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil || len(body.Sessions) != 1 {
		t.Fatalf("sessions = %s, %v", list.Body.String(), err)
	}
	if body.Sessions[0].ID != state.SessionID(auth.HashToken(aliceToken)) || !body.Sessions[0].Current {
		t.Fatalf("session info = %+v", body.Sessions[0])
	}

	if other := socialRequest(handler, http.MethodDelete, path+"/"+body.Sessions[0].ID, tokens["bob"], "", ""); other.Code != http.StatusNotFound {
		t.Fatalf("cross-account revoke = %d: %s", other.Code, other.Body.String())
	}
	if rec := socialRequest(handler, http.MethodGet, path, aliceToken, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("session survived cross-account revoke: %d", rec.Code)
	}

	if rec := socialRequest(handler, http.MethodDelete, path+"/"+body.Sessions[0].ID, aliceToken, "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke one = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := socialRequest(handler, http.MethodGet, path, aliceToken, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token accepted = %d", rec.Code)
	}
}

func TestLogoutAndLogoutAll(t *testing.T) {
	for _, endpoint := range []string{"/api/v1/logout", "/api/v1/logout-all"} {
		t.Run(endpoint, func(t *testing.T) {
			_, handler, _, tokens, _ := newSocialAPI(t)
			rec := socialRequest(handler, http.MethodPost, endpoint, tokens["alice"], "", "")
			if rec.Code != http.StatusNoContent {
				t.Fatalf("%s = %d: %s", endpoint, rec.Code, rec.Body.String())
			}
			if replay := socialRequest(handler, http.MethodGet, "/api/v1/sessions", tokens["alice"], "", ""); replay.Code != http.StatusUnauthorized {
				t.Fatalf("%s did not invalidate token: %d", endpoint, replay.Code)
			}
			if other := socialRequest(handler, http.MethodGet, "/api/v1/sessions", tokens["bob"], "", ""); other.Code != http.StatusOK {
				t.Fatalf("%s hit another account: %d", endpoint, other.Code)
			}
		})
	}
}

func TestLoginRotatesTokenAndCleansExpired(t *testing.T) {
	srv, handler, ids, tokens, _ := newSocialAPI(t)
	expiredHash := auth.HashToken("expired-token")
	if err := srv.store.CreateSession(ids["alice"], expiredHash, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	login := socialRequest(handler, http.MethodPost, "/api/v1/login", "", "", `{"username":"alice","password":"pw"}`)
	if login.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", login.Code, login.Body.String())
	}
	var body struct {
		Token string `json:"session_token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &body); err != nil || body.Token == "" || body.Token == tokens["alice"] {
		t.Fatalf("login token = %s, %v", login.Body.String(), err)
	}
	if rec := socialRequest(handler, http.MethodGet, "/api/v1/sessions", body.Token, "", ""); rec.Code != http.StatusOK {
		t.Fatalf("new token rejected: %d", rec.Code)
	}
	sessions, err := srv.store.SessionsOf(ids["alice"], auth.HashToken(body.Token))
	if err != nil || len(sessions) != 2 {
		t.Fatalf("sessions after login = %+v, %v", sessions, err)
	}
	for _, s := range sessions {
		if s.ID == state.SessionID(expiredHash) {
			t.Fatal("expired session survived login cleanup")
		}
	}
	list := socialRequest(handler, http.MethodGet, "/api/v1/sessions", body.Token, "", "")
	if strings.Contains(list.Body.String(), body.Token) || strings.Contains(list.Body.String(), "password") {
		t.Fatalf("session list leaked secrets: %s", list.Body.String())
	}
}
