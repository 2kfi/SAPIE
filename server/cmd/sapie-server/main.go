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
)

func main() {
	listen := flag.String("listen", ":47584", "TCP game-protocol listen address")
	adminListen := flag.String("admin-listen", "127.0.0.1:8080", "admin HTTP listen address")
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
		log.Printf("generated admin token: %s", token)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	sessions := session.New(store, auth.NewLimiter(nil))
	admin := api.New(store, token)
	errs := make(chan error, 2)
	go func() { errs <- sessions.Serve(ctx, gameLn) }()
	go func() { errs <- admin.Serve(ctx, adminLn) }()
	log.Printf("sapie game protocol on %s, admin API on %s", gameLn.Addr(), adminLn.Addr())

	select {
	case err := <-errs:
		if err != nil {
			log.Fatal(err)
		}
	case <-ctx.Done():
	}
}
