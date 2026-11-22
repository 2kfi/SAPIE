package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sapie/sapie/server/internal/state"
)

func newInviteLobby(t *testing.T, srv *Server, ids map[string]uint64, joinable bool) uint64 {
	t.Helper()
	lobbyID, err := srv.store.CreateLobby(ids["alice"], 480, state.LobbyTypePublic, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !joinable {
		if err := srv.store.UpsertLobby(ids["alice"], state.Lobby{
			LobbyID: lobbyID, Owner: ids["alice"], Appid: 480,
			Type: state.LobbyTypePublic, MemberLimit: 4,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return lobbyID
}

func inviteResponse(body string) map[string]any {
	var out map[string]any
	_ = json.Unmarshal([]byte(body), &out)
	return out
}

func TestLobbyInviteGeneration(t *testing.T) {
	srv, handler, ids, tokens, _ := newSocialAPI(t)
	lobbyID := newInviteLobby(t, srv, ids, true)
	privateID := newInviteLobby(t, srv, ids, false)

	for _, tc := range []struct {
		name, token string
		lobbyID     uint64
		want        int
	}{
		{"member", tokens["alice"], lobbyID, http.StatusOK},
		{"non member", tokens["carol"], lobbyID, http.StatusNotFound},
		{"missing lobby", tokens["alice"], lobbyID + 100, http.StatusNotFound},
		{"not joinable", tokens["alice"], privateID, http.StatusConflict},
		{"unauthenticated", "", lobbyID, http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/api/v1/lobbies/" + strconv.FormatUint(tc.lobbyID, 10) + "/invite"
			rec := socialRequest(handler, http.MethodPost, path, tc.token, "", "")
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want != http.StatusOK {
				return
			}
			body := inviteResponse(rec.Body.String())
			link, _ := body["url"].(string)
			expires, _ := body["expires_at"].(float64)
			gotID, err := srv.resolveInvite(link, time.Unix(int64(expires)-60, 0))
			if err != nil || gotID != lobbyID {
				t.Fatalf("generated link = %q, resolved to %d, %v", link, gotID, err)
			}
			if !strings.HasPrefix(link, fmt.Sprintf("sapie://join/%d?", lobbyID)) {
				t.Fatalf("unexpected link shape: %q", link)
			}
		})
	}
}

func TestLobbyInviteResolution(t *testing.T) {
	srv, handler, ids, tokens, _ := newSocialAPI(t)
	lobbyID := newInviteLobby(t, srv, ids, true)
	valid := srv.inviteLink(lobbyID, time.Now().Add(time.Hour).Unix())

	badSigURL, _ := url.Parse(valid)
	query := badSigURL.Query()
	query.Set("sig", "tampered")
	badSigURL.RawQuery = query.Encode()

	otherLobby := srv.inviteLink(lobbyID+100, time.Now().Add(time.Hour).Unix())
	expired := srv.inviteLink(lobbyID, time.Now().Add(-time.Minute).Unix())

	for _, tc := range []struct {
		name, invite, wantBody string
		want                   int
	}{
		{"valid", valid, "", http.StatusOK},
		{"tampered signature", badSigURL.String(), "invalid invite link", http.StatusBadRequest},
		{"expired", expired, "invite link expired", http.StatusGone},
		{"other lobby", otherLobby, "different lobby", http.StatusBadRequest},
		{"wrong scheme", strings.Replace(valid, "sapie://", "https://", 1), "invalid invite link", http.StatusBadRequest},
		{"not a link", "sapie://join/not-a-number", "invalid invite link", http.StatusBadRequest},
		{"no invite", "", "", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/api/v1/lobbies/" + strconv.FormatUint(lobbyID, 10) + "/join"
			body := `{}`
			if tc.invite != "" {
				encoded, err := json.Marshal(tc.invite)
				if err != nil {
					t.Fatal(err)
				}
				body = `{"invite":` + string(encoded) + `}`
			}
			rec := socialRequest(handler, http.MethodPost, path, tokens["bob"], "", body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tc.wantBody)
			}
		})
	}
	lobby, err := srv.store.Lobby(lobbyID)
	if err != nil {
		t.Fatal(err)
	}
	joined := false
	for _, id := range lobby.Members {
		if id == ids["bob"] {
			joined = true
		}
	}
	if !joined {
		t.Fatalf("bob did not join: %+v", lobby.Members)
	}
}

func TestLobbyInviteRespectsBlockList(t *testing.T) {
	srv, handler, ids, tokens, _ := newSocialAPI(t)
	lobbyID := newInviteLobby(t, srv, ids, true)
	if err := srv.store.BlockUser(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}
	link := srv.inviteLink(lobbyID, time.Now().Add(time.Hour).Unix())
	encoded, err := json.Marshal(link)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/lobbies/" + strconv.FormatUint(lobbyID, 10) + "/join"
	rec := socialRequest(handler, http.MethodPost, path, tokens["bob"], "", `{"invite":`+string(encoded)+`}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "cannot join this lobby") {
		t.Fatalf("blocked join = %d %s", rec.Code, rec.Body.String())
	}
}
