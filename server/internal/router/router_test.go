package router

import (
	"testing"

	"github.com/sapie/sapie/server/internal/protocol"
)

func TestRouteOverwritesSourceID(t *testing.T) {
	r := New(nil)
	msg := &protocol.Common_Message{
		SourceId: 12345, // client-supplied identity must be discarded
		DestId:   678,
		Messages: &protocol.Common_Message_Announce{Announce: &protocol.Announce{}},
	}
	r.Route(999, msg)
	if msg.SourceId != 999 {
		t.Fatalf("source_id = %d, want 999", msg.SourceId)
	}
}
