// Package ui serves the built SPA over HTTP.
package ui

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Handler(dir, adminAddr string) http.Handler {
	target, _ := url.Parse(adminTarget(adminAddr))
	proxy := httputil.NewSingleHostReverseProxy(target)
	api := http.NewServeMux()
	for _, route := range []string{
		"POST /api/v1/login",
		"POST /api/v1/register",
		"GET /api/v1/config",
		"GET /api/v1/friends",
		"POST /api/v1/friends/{id}/invite",
		"GET /api/v1/lobbies",
		"POST /api/v1/lobbies",
		"POST /api/v1/lobbies/{id}/join",
		"GET /api/v1/users/{steam_id}/stats",
	} {
		api.HandleFunc(route, proxy.ServeHTTP)
	}
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			http.NotFound(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
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

func adminTarget(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func Serve(ctx context.Context, ln net.Listener, dir, adminAddr string) error {
	server := &http.Server{Handler: Handler(dir, adminAddr), ReadHeaderTimeout: 5 * time.Second}
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
