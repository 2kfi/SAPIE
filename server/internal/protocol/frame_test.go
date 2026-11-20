package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	tests := []*Frame{
		{Payload: &Frame_Login{Login: &LoginRequest{Username: "alice", Password: "secret", Appid: 480}}},
		{Payload: &Frame_Ping{Ping: true}},
		{Payload: &Frame_ContentServerRequest{ContentServerRequest: &ContentServerRequest{Appid: 480}}},
		{Payload: &Frame_ContentServerResponse{ContentServerResponse: &ContentServerResponse{Ok: true}}},
		{Payload: &Frame_Steam{Steam: &Common_Message{SourceId: 999, DestId: 7, Messages: &Common_Message_Announce{Announce: &Announce{Type: Announce_PING}}}}},
	}
	for _, want := range tests {
		var buf bytes.Buffer
		if err := WriteFrame(&buf, want); err != nil {
			t.Fatal(err)
		}
		if n := binary.BigEndian.Uint32(buf.Bytes()[:4]); int(n) != buf.Len()-4 {
			t.Fatalf("header length %d, body %d", n, buf.Len()-4)
		}
		got, err := ReadFrame(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(mustMarshal(t, want), mustMarshal(t, got)) {
			t.Fatalf("round trip mismatch: %v != %v", want, got)
		}
	}
}

func TestReadFrameRejectsOversize(t *testing.T) {
	var buf bytes.Buffer
	header := make([]byte, 4)
	binary.BigEndian.PutUint32(header, MaxFrameSize+1)
	buf.Write(header)
	if _, err := ReadFrame(&buf); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want ErrFrameTooLarge", err)
	}
}

func TestWriteFrameRejectsOversize(t *testing.T) {
	frame := &Frame{Payload: &Frame_Login{Login: &LoginRequest{Password: string(bytes.Repeat([]byte("x"), MaxFrameSize))}}}
	var buf bytes.Buffer
	if err := WriteFrame(&buf, frame); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want ErrFrameTooLarge", err)
	}
}

func mustMarshal(t *testing.T, f *Frame) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := WriteFrame(buf, f); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
