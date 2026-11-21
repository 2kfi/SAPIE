package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sapie/sapie/server/internal/api"
	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/session"
	"github.com/sapie/sapie/server/internal/state"
	"github.com/sapie/sapie/server/internal/ui"
)

func main() {
	port := flag.Int("port", 47584, "TCP game-protocol port")
	listen := flag.String("listen", "", "TCP game-protocol listen address (overrides --port)")
	adminListen := flag.String("admin-listen", "127.0.0.1:8080", "admin HTTP listen address")
	uiListen := flag.String("ui-listen", "0.0.0.0:8081", "web UI HTTP listen address")
	uiDir := flag.String("ui-dir", "./web/dist", "built web UI directory")
	allowRegister := flag.Bool("allow-register", false, "allow public account registration")
	dbPath := flag.String("db", "sapie.db", "SQLite database path")
	adminToken := flag.String("admin-token", "", "Bearer token for the admin API")
	certFile := flag.String("tls-cert", "", "TLS certificate (requires --tls-key)")
	keyFile := flag.String("tls-key", "", "TLS private key (requires --tls-cert)")
	flag.Parse()

	if (*certFile == "") != (*keyFile == "") {
		log.Fatal("--tls-cert and --tls-key must be supplied together")
	}
	store, err := state.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	token := *adminToken
	if token == "" {
		token, err = store.AdminToken()
		if err != nil {
			log.Fatal(err)
		}
	}
	if token == "" {
		token, err = auth.NewToken()
		if err != nil {
			log.Fatal(err)
		}
		if err := store.SetAdminToken(token); err != nil {
			log.Fatal(err)
		}
		log.Printf("generated admin token %s (saved in the database; override with --admin-token)", token)
	}

	listenAddr := *listen
	if listenAddr == "" {
		listenAddr = fmt.Sprintf(":%d", *port)
	}
	gameLn, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	if *certFile != "" {
		pair, err := tls.LoadX509KeyPair(*certFile, *keyFile)
		if err != nil {
			log.Fatal(err)
		}
		gameLn = tls.NewListener(gameLn, &tls.Config{
			Certificates: []tls.Certificate{pair},
			MinVersion:   tls.VersionTLS12,
		})
	}
	adminLn, err := net.Listen("tcp", *adminListen)
	if err != nil {
		log.Fatal(err)
	}
	uiLn, err := net.Listen("tcp", *uiListen)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	limiter := auth.NewPersistentLimiter(store, nil)
	sessions := session.New(store, limiter)
	admin := api.New(store, token, limiter, sessions)
	admin.SetAllowRegister(*allowRegister)
	cleanup := func() {
		if _, err := store.DeleteExpiredSessions(); err != nil {
			log.Printf("session cleanup: %v", err)
		}
		// One hour covers the longest limiter window (social requests).
		before := time.Now().Add(-time.Hour).Unix()
		if _, err := store.DeleteExpiredRates(before); err != nil {
			log.Printf("rate cleanup: %v", err)
		}
	}
	cleanup()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
	errs := make(chan error, 3)
	go func() { errs <- sessions.Serve(ctx, gameLn) }()
	go func() { errs <- admin.Serve(ctx, adminLn) }()
	go func() { errs <- ui.Serve(ctx, uiLn, *uiDir, *adminListen) }()
	log.Printf("sapie game protocol on %s, admin API on %s, web UI on %s", gameLn.Addr(), adminLn.Addr(), uiLn.Addr())

	select {
	case err := <-errs:
		if err != nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
	}
}
