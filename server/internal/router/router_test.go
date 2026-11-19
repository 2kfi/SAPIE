package router

import (
	"testing"

	"github.com/sapie/sapie/server/internal/protocol"
)

type fakePeer struct{ frames []*protocol.Frame }

func (p *fakePeer) Send(frame *protocol.Frame) error {
	p.frames = append(p.frames, frame)
	return nil
}

type fakeLookup map[uint64]*fakePeer

func (l fakeLookup) PeerFor(id uint64) (Peer, bool) {
	peer, ok := l[id]
	return peer, ok
}

type fakeResolver struct {
	friends map[uint64][]uint64
	lobbies map[uint64][]uint64
}

func (r fakeResolver) FriendsOf(id uint64) ([]uint64, error) { return r.friends[id], nil }

func (r fakeResolver) LobbyMembers(id uint64) ([]uint64, error) { return r.lobbies[id], nil }

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

func TestRouteFanout(t *testing.T) {
	peer2, peer3 := &fakePeer{}, &fakePeer{}
	lookup := fakeLookup{2: peer2, 3: peer3}
	resolver := fakeResolver{
		friends: map[uint64][]uint64{1: {2, 3, 4}},
		lobbies: map[uint64][]uint64{7: {2, 4}},
	}
	r := NewWithResolver(lookup, resolver)

	friendMsg := &protocol.Common_Message{
		SourceId: 999,
		Messages: &protocol.Common_Message_Friend{Friend: &protocol.Friend{Id: 1}},
	}
	r.Route(1, friendMsg)
	if len(peer2.frames) != 1 || len(peer3.frames) != 1 {
		t.Fatalf("presence fanout = %d/%d, want 1/1", len(peer2.frames), len(peer3.frames))
	}
	if got := delivered(t, peer2.frames[0]).GetSourceId(); got != 1 {
		t.Fatalf("delivered source_id = %d, want 1", got)
	}

	r.Route(1, &protocol.Common_Message{
		DestId:   3,
		Messages: &protocol.Common_Message_FriendMessages{FriendMessages: &protocol.Friend_Messages{}},
	})
	if len(peer2.frames) != 1 || len(peer3.frames) != 2 {
		t.Fatalf("targeted invite = %d/%d, want 1/2", len(peer2.frames), len(peer3.frames))
	}

	r.Route(1, &protocol.Common_Message{
		Messages: &protocol.Common_Message_Lobby{Lobby: &protocol.Lobby{RoomId: 7}},
	})
	r.Route(1, &protocol.Common_Message{
		Messages: &protocol.Common_Message_LobbyMessages{LobbyMessages: &protocol.Lobby_Messages{Id: 7}},
	})
	if len(peer2.frames) != 3 || len(peer3.frames) != 2 {
		t.Fatalf("lobby fanout = %d/%d, want 3/2", len(peer2.frames), len(peer3.frames))
	}
	if delivered(t, peer2.frames[1]).GetLobby().GetRoomId() != 7 ||
		delivered(t, peer2.frames[2]).GetLobbyMessages().GetId() != 7 {
		t.Fatal("lobby frames delivered in unexpected order")
	}
}

func delivered(t *testing.T, frame *protocol.Frame) *protocol.Common_Message {
	t.Helper()
	steam, ok := frame.Payload.(*protocol.Frame_Steam)
	if !ok {
		t.Fatalf("got %T, want Frame_Steam", frame.Payload)
	}
	return steam.Steam
}
