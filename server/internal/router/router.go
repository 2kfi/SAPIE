// Package router stamps authenticated identity on routed messages.
package router

import (
	"log"

	"github.com/sapie/sapie/server/internal/protocol"
)

// Peer is a live connection that can receive frames.
type Peer interface {
	Send(*protocol.Frame) error
}

// Lookup resolves a SteamID to a live session.
type Lookup interface {
	PeerFor(uint64) (Peer, bool)
}

type Router struct {
	lookup Lookup
}

func New(lookup Lookup) *Router { return &Router{lookup: lookup} }

// Route is the single place source_id is assigned. M0 logs and drops; later
// milestones fan out to friends/lobby members via Deliver.
func (r *Router) Route(from uint64, msg *protocol.Common_Message) {
	if msg == nil {
		return
	}
	msg.SourceId = from
	log.Printf("route from=%d dest=%d type=%T (M0: drop)", from, msg.DestId, msg.Messages)
}

func (r *Router) Deliver(to uint64, frame *protocol.Frame) {
	peer, ok := r.lookup.PeerFor(to)
	if !ok {
		log.Printf("route to=%d offline (M0: drop)", to)
		return
	}
	if err := peer.Send(frame); err != nil {
		log.Printf("route to=%d: %v", to, err)
	}
}
