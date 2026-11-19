package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sapie/sapie/server/internal/state"
)

func TestAdminAPI(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	handler := New(store, "admin-token").Handler()

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
