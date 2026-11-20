package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/sapie/sapie/server/internal/api"
	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/state"
)

func TestHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>SPA</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := Handler(dir, "127.0.0.1:1")
	for path, want := range map[string]string{
		"/":           "SPA",
		"/#/friends":  "SPA",
		"/some/route": "SPA",
		"/app.js":     "console.log(1)",
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("GET %s = %d %q, want %q", path, rec.Code, rec.Body.String(), want)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/config", nil))
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "SPA") {
		t.Fatalf("API path served by SPA: %d %q", rec.Code, rec.Body.String())
	}

	missing := Handler(filepath.Join(dir, "absent"), "127.0.0.1:1")
	rec = httptest.NewRecorder()
	missing.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") ||
		!strings.Contains(rec.Body.String(), "missing") {
		t.Fatalf("missing directory = %d %q (%q)", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
}

func TestAdminTarget(t *testing.T) {
	for in, want := range map[string]string{
		"127.0.0.1:8080": "http://127.0.0.1:8080",
		":8080":          "http://127.0.0.1:8080",
		"0.0.0.0:8080":   "http://127.0.0.1:8080",
		"[::]:8080":      "http://127.0.0.1:8080",
		"[::1]:8080":     "http://[::1]:8080",
	} {
		if got := adminTarget(in); got != want {
			t.Fatalf("adminTarget(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProxyAllowsUserRoutesOnly(t *testing.T) {
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
	admin := httptest.NewServer(api.New(store, "admin-token", auth.NewLimiter(nil), nil).Handler())
	defer admin.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>SPA</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	ui := httptest.NewServer(Handler(dir, strings.TrimPrefix(admin.URL, "http://")))
	defer ui.Close()
	call := func(method, path, token, body string) (int, string) {
		req, err := http.NewRequest(method, ui.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, string(data)
	}

	status, body := call(http.MethodPost, "/api/v1/login", "", `{"username":"alice","password":"pw"}`)
	if status != http.StatusOK {
		t.Fatalf("login = %d: %s", status, body)
	}
	var login struct {
		Token string `json:"session_token"`
	}
	if err := json.Unmarshal([]byte(body), &login); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/v1/config",
		"/api/v1/friends",
		"/api/v1/lobbies",
		"/api/v1/users/" + strconv.FormatUint(alice, 10) + "/stats",
	} {
		status, _ = call(http.MethodGet, path, login.Token, "")
		if status != http.StatusOK {
			t.Fatalf("user route %s = %d", path, status)
		}
	}
	status, body = call(http.MethodPost, "/api/v1/friends/"+strconv.FormatUint(bob, 10)+"/invite", login.Token, `{"connect_str":"connect=127.0.0.1:27015"}`)
	if status != http.StatusAccepted {
		t.Fatalf("invite = %d: %s", status, body)
	}
	status, body = call(http.MethodPost, "/api/v1/lobbies", login.Token, `{"appid":480,"type":0,"member_limit":2}`)
	if status != http.StatusCreated {
		t.Fatalf("create lobby = %d: %s", status, body)
	}
	status, body = call(http.MethodPost, "/api/v1/lobbies/1/join", login.Token, "")
	if status != http.StatusOK {
		t.Fatalf("join lobby = %d: %s", status, body)
	}
	for _, path := range []string{
		"/api/v1/users",
		"/api/v1/accounts",
		"/api/v1/accounts/" + strconv.FormatUint(alice, 10),
		"/api/v1/unknown",
	} {
		status, _ = call(http.MethodGet, path, "admin-token", "")
		if status != http.StatusNotFound {
			t.Fatalf("admin route %s = %d, want 404", path, status)
		}
	}
	for _, tc := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/accounts"},
		{http.MethodDelete, "/api/v1/accounts/" + strconv.FormatUint(alice, 10)},
	} {
		status, _ = call(tc.method, tc.path, "admin-token", `{}`)
		if status != http.StatusNotFound {
			t.Fatalf("admin route %s %s = %d, want 404", tc.method, tc.path, status)
		}
	}
	status, _ = call(http.MethodGet, "/api/v1/users/"+strconv.FormatUint(bob, 10)+"/stats", login.Token, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("other user's stats = %d, want 401", status)
	}
	status, body = call(http.MethodGet, "/", "", "")
	if status != http.StatusOK || !strings.Contains(body, "SPA") {
		t.Fatalf("SPA after proxy = %d %q", status, body)
	}
}
