package session

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sapie/sapie/server/internal/auth"
	"github.com/sapie/sapie/server/internal/protocol"
	"github.com/sapie/sapie/server/internal/state"
)

func TestLoginHandshake(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.CreateAccount("alice", hash, "Alice")
	if err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := New(store, auth.NewLimiter(nil))
	go srv.Serve(ctx, ln)

	// A non-login first frame closes the connection.
	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Ping{Ping: true}}); err != nil {
		t.Fatal(err)
	}
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := protocol.ReadFrame(c); err == io.EOF {
		// expected
	} else if err != nil {
		t.Fatalf("got %v, want EOF", err)
	} else {
		t.Fatal("server accepted a non-login first frame")
	}
	c.Close()

	// Bad credentials, then a successful login.
	c, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	login := &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "wrong"}}}
	if err := protocol.WriteFrame(c, login); err != nil {
		t.Fatal(err)
	}
	response := readLoginResponse(t, c)
	if response.Status != protocol.LoginResponse_BAD_CREDENTIALS {
		t.Fatalf("status = %v, want BAD_CREDENTIALS", response.Status)
	}

	c.Close()

	c, err = net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := protocol.WriteFrame(c, &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "pw"}}}); err != nil {
		t.Fatal(err)
	}
	response = readLoginResponse(t, c)
	if response.Status != protocol.LoginResponse_OK || response.SteamId != id || len(response.SessionToken) == 0 {
		t.Fatalf("bad login response: %v", response)
	}
	if string(response.PersonaName) != "Alice" {
		t.Fatalf("persona = %q", response.PersonaName)
	}
	if peer, ok := srv.PeerFor(id); !ok || peer == nil {
		t.Fatal("authenticated session not registered")
	}
}

func TestTicketRequest(t *testing.T) {
	store, err := state.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hash, err := auth.HashPassword("pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateAccount("alice", hash, "Alice"); err != nil {
		t.Fatal(err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := New(store, auth.NewLimiter(nil))
	go srv.Serve(ctx, ln)

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	login := &protocol.Frame{Payload: &protocol.Frame_Login{Login: &protocol.LoginRequest{Username: "alice", Password: "pw"}}}
	if err := protocol.WriteFrame(c, login); err != nil {
		t.Fatal(err)
	}
	readLoginResponse(t, c)

	var previous []byte
	for want := uint32(1); want <= 2; want++ {
		req := &protocol.Frame{Payload: &protocol.Frame_TicketRequest{TicketRequest: &protocol.TicketRequest{Appid: 480}}}
		if err := protocol.WriteFrame(c, req); err != nil {
			t.Fatal(err)
		}
		c.SetReadDeadline(time.Now().Add(5 * time.Second))
		frame, err := protocol.ReadFrame(c)
		if err != nil {
			t.Fatal(err)
		}
		result, ok := frame.Payload.(*protocol.Frame_TicketResponse)
		if !ok {
			t.Fatalf("got %T, want TicketResponse", frame.Payload)
		}
		ticket := result.TicketResponse.GetTicket()
		if len(ticket) != 32 {
			t.Fatalf("ticket length = %d, want 32", len(ticket))
		}
		if got := result.TicketResponse.GetNumber(); got != want {
			t.Fatalf("ticket number = %d, want %d", got, want)
		}
		if bytes.Equal(ticket, previous) {
			t.Fatal("repeated ticket bytes")
		}
		previous = ticket
	}
}

func readLoginResponse(t *testing.T, c net.Conn) *protocol.LoginResponse {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := protocol.ReadFrame(c)
	if err != nil {
		t.Fatal(err)
	}
	response, ok := frame.Payload.(*protocol.Frame_LoginResponse)
	if !ok {
		t.Fatalf("got %T, want LoginResponse", frame.Payload)
	}
	return response.LoginResponse
}
