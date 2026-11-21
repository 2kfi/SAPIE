package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

func TestBlockedJoinReturnsForbidden(t *testing.T) {
	srv, handler, ids, tokens, _ := newSocialAPI(t)
	created := socialRequest(handler, http.MethodPost, "/api/v1/lobbies", tokens["alice"], "",
		`{"appid":480,"type":2,"member_limit":4}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create lobby = %d: %s", created.Code, created.Body.String())
	}
	var body struct {
		LobbyID uint64 `json:"lobby_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil || body.LobbyID == 0 {
		t.Fatalf("create body = %s, %v", created.Body.String(), err)
	}
	if err := srv.store.BlockUser(ids["bob"], ids["alice"]); err != nil {
		t.Fatal(err)
	}
	join := "/api/v1/lobbies/" + strconv.FormatUint(body.LobbyID, 10) + "/join"
	if rec := socialRequest(handler, http.MethodPost, join, tokens["bob"], "", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("blocked join = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLobbyPushSkipsBlockedMembers(t *testing.T) {
	srv, handler, ids, tokens, notifier := newSocialAPI(t)
	created := socialRequest(handler, http.MethodPost, "/api/v1/lobbies", tokens["alice"], "",
		`{"appid":480,"type":2,"member_limit":4}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create lobby = %d: %s", created.Code, created.Body.String())
	}
	var body struct {
		LobbyID uint64 `json:"lobby_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &body); err != nil || body.LobbyID == 0 {
		t.Fatalf("create body = %s, %v", created.Body.String(), err)
	}
	join := "/api/v1/lobbies/" + strconv.FormatUint(body.LobbyID, 10) + "/join"
	for _, name := range []string{"bob", "carol"} {
		if rec := socialRequest(handler, http.MethodPost, join, tokens[name], "", ""); rec.Code != http.StatusOK {
			t.Fatalf("%s join = %d: %s", name, rec.Code, rec.Body.String())
		}
	}
	if err := srv.store.BlockUser(ids["alice"], ids["carol"]); err != nil {
		t.Fatal(err)
	}
	bobBefore := len(notifier.delivered[ids["bob"]])
	carolBefore := len(notifier.delivered[ids["carol"]])

	if rec := socialRequest(handler, http.MethodPost, join, tokens["alice"], "", ""); rec.Code != http.StatusOK {
		t.Fatalf("alice join = %d: %s", rec.Code, rec.Body.String())
	}
	if got := len(notifier.delivered[ids["bob"]]); got != bobBefore+1 {
		t.Fatalf("unblocked member frames = %d, want %d", got, bobBefore+1)
	}
	if got := len(notifier.delivered[ids["carol"]]); got != carolBefore {
		t.Fatalf("blocked member received a frame: %d, want %d", got, carolBefore)
	}
}
