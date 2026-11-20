package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/state"
)

func TestIdempotencyConcurrentAndBounded(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	srv := New(store, "admin-token", auth.NewLimiter(nil), nil)
	handler := srv.Handler()
	post := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts",
			strings.NewReader(`{"username":"racer","password":"pw"}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	const racers = 8
	results := make([]*httptest.ResponseRecorder, racers)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = post("shared")
		}(i)
	}
	wg.Wait()
	for _, rec := range results {
		if rec.Code != http.StatusCreated || rec.Body.String() != results[0].Body.String() {
			t.Fatalf("concurrent replay = %d %s", rec.Code, rec.Body.String())
		}
	}
	accounts, err := store.ListAccounts()
	if err != nil || len(accounts) != 1 {
		t.Fatalf("created %d accounts, want 1 (%v)", len(accounts), err)
	}

	srv.mu.Lock()
	for i := 0; i < maxIdemKeys; i++ {
		done := make(chan struct{})
		close(done)
		srv.idem[fmt.Sprintf("seed-%d", i)] = &idemCall{done: done}
	}
	srv.mu.Unlock()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts",
		strings.NewReader(`{"username":"bounded","password":"pw"}`))
	req.Header.Set("Authorization", "Bearer admin-token")
	req.Header.Set("Idempotency-Key", "fresh")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("bounded create = %d: %s", rec.Code, rec.Body.String())
	}
	srv.mu.Lock()
	size := len(srv.idem)
	srv.mu.Unlock()
	if size > maxIdemKeys {
		t.Fatalf("idempotency cache grew to %d entries", size)
	}
}

func TestVerifyTicketEndpoint(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id, err := store.CreateAccount("alice", "h", "Alice")
	if err != nil {
		t.Fatal(err)
	}
	nonce := []byte(strings.Repeat("n", 32))
	if err := store.IssueTicket(id, 480, 1, nonce, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	ticket := base64.StdEncoding.EncodeToString(state.TicketHash(id, 480, nonce))
	handler := New(store, "admin-token", auth.NewLimiter(nil), nil).Handler()
	post := func(body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/verify", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	validBody := fmt.Sprintf(`{"ticket":"%s","steam_id":%d,"appid":480,"number":1}`, ticket, id)
	if rec := post(validBody, "admin-token"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"valid":true`) {
		t.Fatalf("valid ticket = %d %s", rec.Code, rec.Body.String())
	}
	if rec := post(validBody, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated verify = %d", rec.Code)
	}
	wrongNumber := fmt.Sprintf(`{"ticket":"%s","steam_id":%d,"appid":480,"number":2}`, ticket, id)
	if rec := post(wrongNumber, "admin-token"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"valid":false`) {
		t.Fatalf("wrong number = %d %s", rec.Code, rec.Body.String())
	}
}

func TestAdminAPI(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, "admin-token", auth.NewLimiter(nil), nil).Handler()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rec.Code)
	}

	body := `{"username":"alice","password":"pw","persona_name":"Alice"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized create = %d", rec.Code)
	}

	post := func(key string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Idempotency-Key", key)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	first := post("create-alice-2")
	second := post("create-alice-2")
	if first.Code != http.StatusCreated || second.Code != http.StatusCreated || first.Body.String() != second.Body.String() {
		t.Fatalf("idempotent create: %d %s / %d %s", first.Code, first.Body.String(), second.Code, second.Body.String())
	}
	if strings.Contains(first.Body.String(), "password") {
		t.Fatalf("password leaked in create response: %s", first.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	var accounts []state.Account
	if err := json.Unmarshal(rec.Body.Bytes(), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 1 || accounts[0].PersonaName != "Alice" {
		t.Fatalf("unexpected accounts: %+v", accounts)
	}
	if strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("password leaked in list: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/v1/accounts/"+strconv.FormatUint(accounts[0].SteamID, 10), nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPublicConfigAndRegistration(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	srv := New(store, "admin-token", auth.NewLimiter(nil), nil)
	handler := srv.Handler()

	config := func() bool {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("config = %d: %s", rec.Code, rec.Body.String())
		}
		var body struct {
			AllowRegister bool `json:"allow_register"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.AllowRegister
	}
	if config() {
		t.Fatal("registration enabled by default")
	}
	body := `{"username":"racer","password":"pw"}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(body)))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("disabled register = %d: %s", rec.Code, rec.Body.String())
	}
	if accounts, err := store.ListAccounts(); err != nil || len(accounts) != 0 {
		t.Fatalf("disabled register created accounts: %d (%v)", len(accounts), err)
	}

	srv.SetAllowRegister(true)
	if !config() {
		t.Fatal("config did not report enabled registration")
	}
	register := func(payload string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/register", strings.NewReader(payload)))
		return rec
	}
	created := register(body)
	if created.Code != http.StatusCreated || strings.Contains(strings.ToLower(created.Body.String()), "password") {
		t.Fatalf("register = %d: %s", created.Code, created.Body.String())
	}
	var account state.Account
	if err := json.Unmarshal(created.Body.Bytes(), &account); err != nil {
		t.Fatal(err)
	}
	if account.Username != "racer" || account.PersonaName != "racer" {
		t.Fatalf("unexpected account: %+v", account)
	}
	if dup := register(body); dup.Code != http.StatusConflict {
		t.Fatalf("duplicate register = %d: %s", dup.Code, dup.Body.String())
	}
	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/v1/login", strings.NewReader(body)))
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), "session_token") {
		t.Fatalf("login = %d: %s", login.Code, login.Body.String())
	}
}

type fakeNotifier struct {
	delivered map[uint64][]*protocol.Frame
	online    map[uint64]bool
}

func (n *fakeNotifier) Deliver(id uint64, frame *protocol.Frame) {
	n.delivered[id] = append(n.delivered[id], frame)
}

func (n *fakeNotifier) IsOnline(id uint64) bool { return n.online[id] }

func request(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStatsEndpoint(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	alice, err := store.CreateAccount("alice", hash, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateAccount("bob", hash, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	unlock, err := store.UnlockAchievement(alice, state.Achievement{
		Appid: 480, ID: "first_kill", DisplayName: "First Kill", UnlockTime: 9999,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"alice-token", "bob-token"} {
		id := alice
		if token == "bob-token" {
			id = bob
		}
		if err := store.CreateSession(id, auth.HashToken(token), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(store, "admin-token", auth.NewLimiter(nil), nil).Handler()
	path := "/api/v1/users/" + strconv.FormatUint(alice, 10) + "/stats"

	for _, token := range []string{"admin-token", "alice-token"} {
		rec := request(handler, http.MethodGet, path, token, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("stats as %s = %d: %s", token, rec.Code, rec.Body.String())
		}
		var stats state.UserStats
		if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
			t.Fatal(err)
		}
		if stats.SteamID != alice || len(stats.Achievements) != 1 ||
			stats.Achievements[0].UnlockTime != unlock.UnlockTime ||
			stats.Achievements[0].DisplayName != "First Kill" {
			t.Fatalf("stats as %s = %s", token, rec.Body.String())
		}
	}
	for _, token := range []string{"bob-token", "wrong"} {
		if rec := request(handler, http.MethodGet, path, token, ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("stats as %s = %d", token, rec.Code)
		}
	}
	if rec := request(handler, http.MethodGet, path, "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("stats without token = %d", rec.Code)
	}
	if rec := request(handler, http.MethodGet, "/api/v1/users/nope/stats", "admin-token", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad steam id = %d", rec.Code)
	}
}

func TestUserEndpoints(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	alice, err := store.CreateAccount("alice", hash, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	bob, err := store.CreateAccount("bob", hash, "Bob")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddFriend(alice, bob); err != nil {
		t.Fatal(err)
	}
	notifier := &fakeNotifier{
		delivered: map[uint64][]*protocol.Frame{},
		online:    map[uint64]bool{alice: true, bob: true},
	}
	handler := New(store, "admin-token", auth.NewLimiter(nil), notifier).Handler()

	loginBody := `{"username":"alice","password":"pw"}`
	rec := request(handler, http.MethodPost, "/api/v1/login", "", loginBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", rec.Code, rec.Body.String())
	}
	var session struct {
		Token string `json:"session_token"`
		ID    uint64 `json:"steam_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &session); err != nil || session.Token == "" || session.ID != alice {
		t.Fatalf("login body = %s, %v", rec.Body.String(), err)
	}
	if rec := request(handler, http.MethodPost, "/api/v1/login", "", `{"username":"alice","password":"bad"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login = %d", rec.Code)
	}

	rec = request(handler, http.MethodGet, "/api/v1/friends", session.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("friends = %d", rec.Code)
	}
	var friends struct {
		Friends []state.Presence `json:"friends"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &friends); err != nil || len(friends.Friends) != 1 ||
		friends.Friends[0].SteamID != bob || friends.Friends[0].PersonaState != 1 {
		t.Fatalf("friends body = %s, %v", rec.Body.String(), err)
	}

	invite := `{"connect_str":"connect=127.0.0.1:27015"}`
	rec = request(handler, http.MethodPost, "/api/v1/friends/"+strconv.FormatUint(bob, 10)+"/invite", session.Token, invite)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("invite = %d: %s", rec.Code, rec.Body.String())
	}
	frames := notifier.delivered[bob]
	if len(frames) != 1 {
		t.Fatalf("delivered to bob = %d, want 1", len(frames))
	}
	steam, ok := frames[0].Payload.(*protocol.Frame_Steam)
	if !ok || steam.Steam.GetFriendMessages().GetType() != protocol.Friend_Messages_GAME_INVITE ||
		string(steam.Steam.GetFriendMessages().GetConnectStr()) != "connect=127.0.0.1:27015" {
		t.Fatalf("invite frame = %+v", frames[0].Payload)
	}

	rec = request(handler, http.MethodPost, "/api/v1/lobbies", session.Token, `{"appid":480,"type":0,"member_limit":2}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create lobby = %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		LobbyID uint64 `json:"lobby_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.LobbyID == 0 {
		t.Fatalf("create body = %s, %v", rec.Body.String(), err)
	}
	rec = request(handler, http.MethodPost, "/api/v1/lobbies/"+strconv.FormatUint(created.LobbyID, 10)+"/join", session.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("join lobby = %d: %s", rec.Code, rec.Body.String())
	}
	rec = request(handler, http.MethodGet, "/api/v1/lobbies", "admin-token", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin lobbies = %d", rec.Code)
	}
	var lobbies struct {
		Lobbies []state.Lobby `json:"lobbies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &lobbies); err != nil || len(lobbies.Lobbies) != 1 ||
		len(lobbies.Lobbies[0].Members) != 1 {
		t.Fatalf("lobbies body = %s, %v", rec.Body.String(), err)
	}

	rec = request(handler, http.MethodGet, "/api/v1/users", "admin-token", "")
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "password") {
		t.Fatalf("users = %d: %s", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/friends", nil)
	req.Header.Set("Origin", "http://localhost:1420")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:1420" {
		t.Fatalf("CORS origin = %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("CORS allowed hostile origin: %q", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
