package ui

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>SPA</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	handler := Handler(dir)
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
	if rec.Code != http.StatusNotFound || strings.Contains(rec.Body.String(), "SPA") {
		t.Fatalf("API path served by SPA: %d %q", rec.Code, rec.Body.String())
	}

	missing := Handler(filepath.Join(dir, "absent"))
	rec = httptest.NewRecorder()
	missing.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusServiceUnavailable ||
		!strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") ||
		!strings.Contains(rec.Body.String(), "missing") {
		t.Fatalf("missing directory = %d %q (%q)", rec.Code, rec.Body.String(), rec.Header().Get("Content-Type"))
	}
}
