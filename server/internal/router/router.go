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
	LobbiesOf(uint64) ([]uint64, error)
	LobbyMembers(uint64) ([]uint64, error)
	IsBlocked(uint64, uint64) (bool, error)
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
		if r.blocked(from, msg.DestId) {
			log.Printf("route from=%d to=%d dropped: blocked", from, msg.DestId)
			return nil
		}
		if isP2P(msg) && !r.p2pAllowed(from, msg.DestId) {
			log.Printf("route p2p from=%d to=%d dropped: not friend or lobby mate", from, msg.DestId)
			return nil
		}
		if isInvite(msg) && !r.friendAllowed(from, msg.DestId) {
			log.Printf("route invite from=%d to=%d dropped: not friend", from, msg.DestId)
			return nil
		}
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
		allowed := make([]uint64, 0, len(ids))
		for _, id := range ids {
			if !r.blocked(from, id) {
				allowed = append(allowed, id)
			}
		}
		return allowed
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

// p2pAllowed reports whether to is a friend of from or shares a lobby with them.
func (r *Router) p2pAllowed(from, to uint64) bool {
	if r.resolver == nil {
		return false
	}
	friends, err := r.resolver.FriendsOf(from)
	if err != nil {
		log.Printf("route p2p from=%d friends: %v", from, err)
	}
	if contains(friends, to) {
		return true
	}
	lobbies, err := r.resolver.LobbiesOf(from)
	if err != nil {
		log.Printf("route p2p from=%d lobbies: %v", from, err)
	}
	for _, lobbyID := range lobbies {
		members, err := r.resolver.LobbyMembers(lobbyID)
		if err != nil {
			log.Printf("route p2p lobby=%d members: %v", lobbyID, err)
			continue
		}
		if contains(members, to) {
			return true
		}
	}
	return false
}

func (r *Router) blocked(a, b uint64) bool {
	if r.resolver == nil {
		return false
	}
	blocked, err := r.resolver.IsBlocked(a, b)
	if err != nil {
		log.Printf("route block check from=%d to=%d: %v", a, b, err)
	}
	return blocked
}

func (r *Router) friendAllowed(from, to uint64) bool {
	if r.resolver == nil {
		return false
	}
	friends, err := r.resolver.FriendsOf(from)
	if err != nil {
		log.Printf("route invite from=%d friends: %v", from, err)
	}
	return contains(friends, to)
}

func contains(ids []uint64, want uint64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func isP2P(msg *protocol.Common_Message) bool {
	switch msg.Messages.(type) {
	case *protocol.Common_Message_NetworkingSockets, *protocol.Common_Message_NetworkingMessages:
		return true
	}
	return false
}

func isInvite(msg *protocol.Common_Message) bool {
	_, ok := msg.Messages.(*protocol.Common_Message_FriendMessages)
	return ok
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
