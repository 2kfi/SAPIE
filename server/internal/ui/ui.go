// Package ui serves the built SPA over HTTP.
package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" || strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		index := filepath.Join(dir, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, fmt.Sprintf("SAPIE web UI is unavailable: %s is missing. Build web/dist or pass --ui-dir.", index), http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && !strings.Contains(name, "..") {
			if info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name))); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		http.ServeFile(w, r, index) // hash routing: every unknown path is the SPA entry point
	})
}

func Serve(ctx context.Context, ln net.Listener, dir string) error {
	server := &http.Server{Handler: Handler(dir), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()
	err := server.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		return nil
	}
	return err
}
