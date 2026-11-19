// Command harness starts SAPIE in-process and proves two accounts can log in.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/sapie/sapie/server/internal/api"
	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/session"
	"github.com/sapie/sapie/server/internal/state"
)

const adminToken = "harness-admin-token"

func main() {
	if err := run(); err != nil {
		log.Fatalf("FAIL %v", err)
	}
	fmt.Println("PASS harness")
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := state.Open(":memory:")
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	gameLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen game: %w", err)
	}
	adminLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("listen admin: %w", err)
	}
	sessions := session.New(store, auth.NewLimiter(nil))
	errs := make(chan error, 2)
	go func() { errs <- sessions.Serve(ctx, gameLn) }()
	go func() { errs <- api.New(store, adminToken).Serve(ctx, adminLn) }()
	defer func() {
		cancel()
		for i := 0; i < 2; i++ {
			<-errs
		}
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	adminURL := "http://" + adminLn.Addr().String()
	alice, err := createAccount(client, adminURL, "harness-alice", "alice-pw", "Alice")
	if err != nil {
		return err
	}
	bob, err := createAccount(client, adminURL, "harness-bob", "bob-pw", "Bob")
	if err != nil {
		return err
	}

	aliceConn, aliceLogin, err := login(gameLn.Addr().String(), "harness-alice", "alice-pw")
	if err != nil {
		return err
	}
	defer aliceConn.Close()
	fmt.Printf("PASS login alice steam_id=%d\n", aliceLogin.SteamId)

	bobConn, bobLogin, err := login(gameLn.Addr().String(), "harness-bob", "bob-pw")
	if err != nil {
		return err
	}
	defer bobConn.Close()
	fmt.Printf("PASS login bob steam_id=%d\n", bobLogin.SteamId)

	if aliceLogin.SteamId != alice.SteamID || bobLogin.SteamId != bob.SteamID {
		return fmt.Errorf("login IDs %d/%d differ from admin records %d/%d",
			aliceLogin.SteamId, bobLogin.SteamId, alice.SteamID, bob.SteamID)
	}
	fmt.Println("PASS login SteamIDs match admin records")
	if aliceLogin.SteamId == bobLogin.SteamId {
		return fmt.Errorf("duplicate SteamID %d", aliceLogin.SteamId)
	}
	if len(aliceLogin.SessionToken) == 0 || len(bobLogin.SessionToken) == 0 {
		return fmt.Errorf("missing session tokens")
	}
	fmt.Println("PASS distinct SteamIDs")

	ticket, err := requestTicket(aliceConn, 480)
	if err != nil {
		return err
	}
	if len(ticket.Ticket) == 0 || ticket.Number == 0 {
		return fmt.Errorf("bad ticket response: %v", ticket)
	}
	fmt.Printf("PASS ticket issued steam_id=%d appid=480 number=%d bytes=%d\n",
		aliceLogin.SteamId, ticket.Number, len(ticket.Ticket))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, adminURL+"/api/v1/accounts", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("list accounts status %d: %s", resp.StatusCode, body)
	}
	var accounts []state.Account
	if err := json.Unmarshal(body, &accounts); err != nil {
		return fmt.Errorf("decode accounts: %w", err)
	}
	if len(accounts) != 2 {
		return fmt.Errorf("got %d accounts, want 2", len(accounts))
	}
	seen := map[uint64]bool{alice.SteamID: true, bob.SteamID: true}
	for _, account := range accounts {
		if !seen[account.SteamID] {
			return fmt.Errorf("unexpected account %d in list", account.SteamID)
		}
	}
	if strings.Contains(string(body), "password") {
		return fmt.Errorf("admin API exposed password data: %s", body)
	}
	fmt.Println("PASS admin API lists both accounts")
	return nil
}

func createAccount(client *http.Client, base, username, password, persona string) (state.Account, error) {
	payload, err := json.Marshal(map[string]string{
		"username": username, "password": password, "persona_name": persona,
	})
	if err != nil {
		return state.Account{}, err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/accounts", bytes.NewReader(payload))
	if err != nil {
		return state.Account{}, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "create-"+username)
	resp, err := client.Do(req)
	if err != nil {
		return state.Account{}, fmt.Errorf("create %s: %w", username, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return state.Account{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		return state.Account{}, fmt.Errorf("create %s status %d: %s", username, resp.StatusCode, body)
	}
	var account state.Account
	if err := json.Unmarshal(body, &account); err != nil {
		return state.Account{}, err
	}
	return account, nil
}

func requestTicket(conn net.Conn, appid uint32) (*protocol.TicketResponse, error) {
	frame := &protocol.Frame{Payload: &protocol.Frame_TicketRequest{TicketRequest: &protocol.TicketRequest{
		Appid: appid, Data: []byte("harness-ticket-request"),
	}}}
	if err := protocol.WriteFrame(conn, frame); err != nil {
		return nil, fmt.Errorf("send ticket request: %w", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response, err := protocol.ReadFrame(conn)
	if err != nil {
		return nil, fmt.Errorf("read ticket response: %w", err)
	}
	result, ok := response.Payload.(*protocol.Frame_TicketResponse)
	if !ok {
		return nil, fmt.Errorf("got %T, want TicketResponse", response.Payload)
	}
	return result.TicketResponse, nil
}

func login(addr, username, password string) (net.Conn, *protocol.LoginResponse, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	frame := &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{
		Username: username, Password: password, Appid: 480, ClientVersion: "harness",
	}}}
	if err := protocol.WriteFrame(conn, frame); err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("send login: %w", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	response, err := protocol.ReadFrame(conn)
	if err != nil {
		conn.Close()
		return nil, nil, fmt.Errorf("read login response: %w", err)
	}
	result, ok := response.Payload.(*protocol.Frame_LoginResponse)
	if !ok {
		conn.Close()
		return nil, nil, fmt.Errorf("got %T, want LoginResponse", response.Payload)
	}
	if result.LoginResponse.Status != protocol.LoginResponse_OK {
		conn.Close()
		return nil, nil, fmt.Errorf("login %s status %v", username, result.LoginResponse.Status)
	}
	return conn, result.LoginResponse, nil
}
