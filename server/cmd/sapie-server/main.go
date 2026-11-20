package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/sapie/sapie/server/internal/api"
	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/session"
	"github.com/sapie/sapie/server/internal/state"
	"github.com/sapie/sapie/server/internal/ui"
)

func main() {
	listen := flag.String("listen", ":47584", "TCP game-protocol listen address")
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
		token, err = auth.NewToken()
		if err != nil {
			log.Fatal(err)
		}
		log.Print("generated admin token (value withheld); pass --admin-token to use a known value")
	}

	gameLn, err := net.Listen("tcp", *listen)
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
	limiter := auth.NewLimiter(nil)
	sessions := session.New(store, limiter)
	admin := api.New(store, token, limiter, sessions)
	admin.SetAllowRegister(*allowRegister)
	errs := make(chan error, 3)
	go func() { errs <- sessions.Serve(ctx, gameLn) }()
	go func() { errs <- admin.Serve(ctx, adminLn) }()
	go func() { errs <- ui.Serve(ctx, uiLn, *uiDir) }()
	log.Printf("sapie game protocol on %s, admin API on %s, web UI on %s", gameLn.Addr(), adminLn.Addr(), uiLn.Addr())

	select {
	case err := <-errs:
		if err != nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
	}
}
