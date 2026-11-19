// Package router stamps authenticated identity on routed messages and fans out.
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

// Resolver supplies the recipient sets for presence and lobby messages.
type Resolver interface {
	FriendsOf(uint64) ([]uint64, error)
	LobbyMembers(uint64) ([]uint64, error)
}

type Router struct {
	lookup   Lookup
	resolver Resolver
}

func New(lookup Lookup) *Router { return &Router{lookup: lookup} }

func NewWithResolver(lookup Lookup, resolver Resolver) *Router {
	return &Router{lookup: lookup, resolver: resolver}
}

// Route is the single place source_id is assigned.
func (r *Router) Route(from uint64, msg *protocol.Common_Message) {
	if msg == nil {
		return
	}
	msg.SourceId = from
	for _, to := range r.recipients(from, msg) {
		if to == from {
			continue
		}
		r.Deliver(to, &protocol.Frame{Payload: &protocol.Frame_Steam{Steam: msg}})
	}
}

func (r *Router) recipients(from uint64, msg *protocol.Common_Message) []uint64 {
	if msg.DestId != 0 {
		return []uint64{msg.DestId}
	}
	if r.resolver == nil {
		return nil
	}
	switch msg.Messages.(type) {
	case *protocol.Common_Message_Friend, *protocol.Common_Message_FriendMessages:
		ids, err := r.resolver.FriendsOf(from)
		if err != nil {
			log.Printf("route from=%d friends: %v", from, err)
		}
		return ids
	case *protocol.Common_Message_Lobby:
		return r.lobbyRecipients(msg.GetLobby().GetRoomId())
	case *protocol.Common_Message_LobbyMessages:
		return r.lobbyRecipients(msg.GetLobbyMessages().GetId())
	}
	return nil
}

func (r *Router) lobbyRecipients(lobbyID uint64) []uint64 {
	ids, err := r.resolver.LobbyMembers(lobbyID)
	if err != nil {
		log.Printf("route lobby=%d members: %v", lobbyID, err)
	}
	return ids
}

func (r *Router) Deliver(to uint64, frame *protocol.Frame) {
	if r.lookup == nil {
		return
	}
	peer, ok := r.lookup.PeerFor(to)
	if !ok {
		log.Printf("route to=%d offline, dropped", to)
		return
	}
	if err := peer.Send(frame); err != nil {
		log.Printf("route to=%d: %v", to, err)
	}
}
