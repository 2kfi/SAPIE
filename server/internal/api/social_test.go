package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/state"
)

func newSocialAPI(t *testing.T) (*Server, http.Handler, map[string]uint64, map[string]string, *fakeNotifier) {
	t.Helper()
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]uint64{}
	tokens := map[string]string{}
	for _, name := range []string{"alice", "bob", "carol"} {
		ids[name], err = store.CreateAccount(name, hash, strings.ToUpper(name[:1])+name[1:])
		if err != nil {
			t.Fatal(err)
		}
		tokens[name] = name + "-token"
		if err := store.CreateSession(ids[name], auth.HashToken(tokens[name]), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	notifier := &fakeNotifier{delivered: map[uint64][]*protocol.Frame{}, online: map[uint64]bool{}}
	srv := New(store, "admin-token", auth.NewLimiter(nil), notifier)
	return srv, srv.Handler(), ids, tokens, notifier
}

func socialRequest(h http.Handler, method, path, token, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSocialRoutesRequireAuth(t *testing.T) {
	_, handler, ids, _, _ := newSocialAPI(t)
	id := strconv.FormatUint(ids["bob"], 10)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/friends/requests"},
		{http.MethodPost, "/api/v1/friends/requests"},
		{http.MethodPost, "/api/v1/friends/requests/1/accept"},
		{http.MethodPost, "/api/v1/friends/requests/1/decline"},
		{http.MethodDelete, "/api/v1/friends/requests/1"},
		{http.MethodDelete, "/api/v1/friends/" + id},
		{http.MethodPost, "/api/v1/friends/" + id + "/block"},
		{http.MethodDelete, "/api/v1/friends/" + id + "/block"},
		{http.MethodGet, "/api/v1/friends/blocked"},
		{http.MethodGet, "/api/v1/users/search?q=bo"},
		{http.MethodPost, "/api/v1/logout"},
		{http.MethodPost, "/api/v1/logout-all"},
		{http.MethodGet, "/api/v1/sessions"},
		{http.MethodDelete, "/api/v1/sessions/abcdefabcdef"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			if rec := socialRequest(handler, tc.method, tc.path, "", "", "{}"); rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestFriendRequestHTTPLifecycle(t *testing.T) {
	_, handler, ids, tokens, notifier := newSocialAPI(t)
	alice, bob, _ := ids["alice"], ids["bob"], ids["carol"]

	sendBody := `{"username":"bob","message":"play?"}`
	first := socialRequest(handler, http.MethodPost, "/api/v1/friends/requests", tokens["alice"], "send-1", sendBody)
	second := socialRequest(handler, http.MethodPost, "/api/v1/friends/requests", tokens["alice"], "send-1", sendBody)
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated || first.Body.String() != second.Body.String() {
		t.Fatalf("idempotent send: %d %s / %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
	var sent struct {
		ID       int64  `json:"id"`
		Status   string `json:"status"`
		CreateAt int64  `json:"created_at"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &sent); err != nil || sent.ID == 0 || sent.Status != "pending" || sent.CreateAt == 0 {
		t.Fatalf("send body = %s, %v", first.Body.String(), err)
	}

	list := socialRequest(handler, http.MethodGet, "/api/v1/friends/requests", tokens["bob"], "", "")
	var bobRequests state.FriendRequests
	if err := json.Unmarshal(list.Body.Bytes(), &bobRequests); err != nil || len(bobRequests.Incoming) != 1 ||
		bobRequests.Incoming[0].PersonaName != "Alice" {
		t.Fatalf("bob requests = %s, %v", list.Body.String(), err)
	}
	list = socialRequest(handler, http.MethodGet, "/api/v1/friends/requests", tokens["alice"], "", "")
	var aliceRequests state.FriendRequests
	if err := json.Unmarshal(list.Body.Bytes(), &aliceRequests); err != nil || len(aliceRequests.Outgoing) != 1 {
		t.Fatalf("alice requests = %s, %v", list.Body.String(), err)
	}

	acceptPath := "/api/v1/friends/requests/" + strconv.FormatInt(sent.ID, 10) + "/accept"
	if rec := socialRequest(handler, http.MethodPost, acceptPath, tokens["carol"], "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("accept by stranger = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := socialRequest(handler, http.MethodPost, acceptPath, tokens["bob"], "accept-1", ""); rec.Code != http.StatusOK {
		t.Fatalf("accept = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := socialRequest(handler, http.MethodPost, acceptPath, tokens["bob"], "accept-1", ""); rec.Code != http.StatusOK {
		t.Fatalf("replayed accept = %d: %s", rec.Code, rec.Body.String())
	}
	friends := socialRequest(handler, http.MethodGet, "/api/v1/friends", tokens["alice"], "", "")
	if !strings.Contains(friends.Body.String(), strconv.FormatUint(bob, 10)) {
		t.Fatalf("friend list = %s", friends.Body.String())
	}
	for _, id := range []uint64{alice, bob} {
		frames := notifier.delivered[id]
		if len(frames) == 0 {
			t.Fatalf("no friend snapshot pushed to %d", id)
		}
		steam, ok := frames[len(frames)-1].Payload.(*protocol.Frame_Steam)
		if !ok || steam.Steam.GetFriend().GetId() == 0 {
			t.Fatalf("snapshot frame to %d = %+v", id, frames[len(frames)-1].Payload)
		}
	}

	// The same key in another account's namespace cannot replay Alice's send.
	shared := socialRequest(handler, http.MethodPost, "/api/v1/friends/requests", tokens["carol"], "send-1", `{"username":"bob"}`)
	if shared.Code != http.StatusCreated || shared.Body.String() == first.Body.String() {
		t.Fatalf("cross-user idempotency leak = %d %s", shared.Code, shared.Body.String())
	}
}

func TestFriendRequestHTTPRejections(t *testing.T) {
	_, handler, ids, tokens, _ := newSocialAPI(t)
	bobID := strconv.FormatUint(ids["bob"], 10)
	for _, tc := range []struct {
		name, token, path, body string
		want                    int
	}{
		{"self", tokens["alice"], "/api/v1/friends/requests", `{"username":"alice"}`, http.StatusNotFound},
		{"unknown", tokens["alice"], "/api/v1/friends/requests", `{"username":"nobody"}`, http.StatusNotFound},
		{"bad json", tokens["alice"], "/api/v1/friends/requests", `{`, http.StatusBadRequest},
		{"long message", tokens["alice"], "/api/v1/friends/requests", `{"username":"bob","message":"` + strings.Repeat("x", 281) + `"}`, http.StatusBadRequest},
		{"bad request id", tokens["alice"], "/api/v1/friends/requests/nope/accept", "", http.StatusBadRequest},
		{"missing request", tokens["alice"], "/api/v1/friends/requests/999/accept", "", http.StatusNotFound},
		{"decline missing", tokens["alice"], "/api/v1/friends/requests/999/decline", "", http.StatusNotFound},
		{"cancel missing", tokens["alice"], "/api/v1/friends/requests/999", "", http.StatusNotFound},
		{"bad steam id", tokens["alice"], "/api/v1/friends/nope", "", http.StatusBadRequest},
		{"bad block id", tokens["alice"], "/api/v1/friends/nope/block", "", http.StatusBadRequest},
		{"self block", tokens["alice"], "/api/v1/friends/" + strconv.FormatUint(ids["alice"], 10) + "/block", "", http.StatusNotFound},
		{"remove nonfriend", tokens["alice"], "/api/v1/friends/" + bobID, "", http.StatusNotFound},
		{"short search", tokens["alice"], "/api/v1/users/search?q=b", "", http.StatusBadRequest},
		{"missing search", tokens["alice"], "/api/v1/users/search", "", http.StatusBadRequest},
		{"nonnumeric block id", tokens["alice"], "/api/v1/friends/requests/block", "", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := socialRequest(handler, methodFor(tc.path), tc.path, tc.token, "", tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func methodFor(path string) string {
	switch {
	case strings.HasSuffix(path, "/accept"), strings.HasSuffix(path, "/decline"), strings.HasSuffix(path, "/block"):
		return http.MethodPost
	case path == "/api/v1/friends/requests" || path == "/api/v1/users/search?q=b" || path == "/api/v1/users/search":
		if strings.HasPrefix(path, "/api/v1/users/search") {
			return http.MethodGet
		}
		return http.MethodPost
	default:
		return http.MethodDelete
	}
}

func TestBlockRemoveAndSearchHTTP(t *testing.T) {
	srv, handler, ids, tokens, notifier := newSocialAPI(t)
	alice, bob := ids["alice"], ids["bob"]
	if err := srv.store.AddFriend(alice, bob); err != nil {
		t.Fatal(err)
	}

	if rec := socialRequest(handler, http.MethodPost, "/api/v1/friends/"+strconv.FormatUint(bob, 10)+"/block", tokens["alice"], "block-1", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("block = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := socialRequest(handler, http.MethodPost, "/api/v1/friends/requests", tokens["alice"], "", `{"username":"bob"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("blocked request = %d: %s", rec.Code, rec.Body.String())
	}
	blocked := socialRequest(handler, http.MethodGet, "/api/v1/friends/blocked", tokens["alice"], "", "")
	if !strings.Contains(blocked.Body.String(), strconv.FormatUint(bob, 10)) {
		t.Fatalf("blocked list = %s", blocked.Body.String())
	}
	if rec := socialRequest(handler, http.MethodDelete, "/api/v1/friends/"+strconv.FormatUint(bob, 10)+"/block", tokens["alice"], "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unblock = %d: %s", rec.Code, rec.Body.String())
	}
	if len(notifier.delivered[bob]) == 0 {
		t.Fatal("unblock did not push a friend snapshot")
	}
	search := socialRequest(handler, http.MethodGet, "/api/v1/users/search?q=bo", tokens["alice"], "", "")
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), `"persona_name":"Bob"`) {
		t.Fatalf("search = %d: %s", search.Code, search.Body.String())
	}
	if strings.Contains(strings.ToLower(search.Body.String()), "password") || strings.Contains(search.Body.String(), "hash") {
		t.Fatalf("search leaked account fields: %s", search.Body.String())
	}

	if rec := socialRequest(handler, http.MethodDelete, "/api/v1/friends/"+strconv.FormatUint(bob, 10), tokens["alice"], "", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("remove = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := socialRequest(handler, http.MethodGet, "/api/v1/friends", tokens["alice"], "", ""); strings.Contains(rec.Body.String(), strconv.FormatUint(bob, 10)) {
		t.Fatalf("friend survived removal: %s", rec.Body.String())
	}
}

func TestSocialRateLimit(t *testing.T) {
	srv, handler, ids, tokens, _ := newSocialAPI(t)
	now := time.Now()
	srv.social.now = func() time.Time { return now }
	for i := 0; i < socialLimit; i++ {
		if rec := socialRequest(handler, http.MethodGet, "/api/v1/users/search?q=bob", tokens["alice"], "", ""); rec.Code != http.StatusOK {
			t.Fatalf("call %d = %d", i, rec.Code)
		}
	}
	for _, path := range []string{"/api/v1/users/search?q=bob", "/api/v1/friends/requests"} {
		method := http.MethodGet
		body := ""
		if strings.HasPrefix(path, "/api/v1/friends/requests") {
			method, body = http.MethodPost, fmt.Sprintf(`{"username":"%s"}`, "bob")
		}
		if rec := socialRequest(handler, method, path, tokens["alice"], "", body); rec.Code != http.StatusTooManyRequests {
			t.Fatalf("rate limited %s = %d: %s", path, rec.Code, rec.Body.String())
		}
	}
	if rec := socialRequest(handler, http.MethodGet, "/api/v1/users/search?q=bob", tokens["bob"], "", ""); rec.Code != http.StatusOK {
		t.Fatalf("other account rate limited = %d", rec.Code)
	}
	_ = ids
}
