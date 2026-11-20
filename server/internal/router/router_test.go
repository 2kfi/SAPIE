package router

import (
	"bytes"
	"log"
	"strings"
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
	friends     map[uint64][]uint64
	userLobbies map[uint64][]uint64
	lobbies     map[uint64][]uint64
}

func (r fakeResolver) FriendsOf(id uint64) ([]uint64, error) { return r.friends[id], nil }

func (r fakeResolver) LobbiesOf(id uint64) ([]uint64, error) { return r.userLobbies[id], nil }

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

func p2pResolver() fakeResolver {
	return fakeResolver{
		friends:     map[uint64][]uint64{1: {2}},
		userLobbies: map[uint64][]uint64{1: {7}},
		lobbies:     map[uint64][]uint64{7: {1, 3}},
	}
}

func socketMsg(dest uint64, typ protocol.Networking_Sockets_Types) *protocol.Common_Message {
	return &protocol.Common_Message{
		DestId: dest,
		Messages: &protocol.Common_Message_NetworkingSockets{
			NetworkingSockets: &protocol.Networking_Sockets{Type: typ},
		},
	}
}

func messagesMsg(dest uint64, typ protocol.Networking_Messages_Types) *protocol.Common_Message {
	return &protocol.Common_Message{
		DestId: dest,
		Messages: &protocol.Common_Message_NetworkingMessages{
			NetworkingMessages: &protocol.Networking_Messages{Type: typ},
		},
	}
}

func captureLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(orig)
	fn()
	return buf.String()
}

func TestRouteP2PAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name string
		dest uint64
		want int
	}{
		{"friend", 2, 1},
		{"lobby mate", 3, 1},
		{"stranger", 4, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := &fakePeer{}
			r := NewWithResolver(fakeLookup{tc.dest: peer}, p2pResolver())
			r.Route(1, socketMsg(tc.dest, protocol.Networking_Sockets_CONNECTION_REQUEST))
			if got := len(peer.frames); got != tc.want {
				t.Fatalf("delivered = %d, want %d", got, tc.want)
			}
			if tc.want > 0 && delivered(t, peer.frames[0]).GetSourceId() != 1 {
				t.Fatal("routed P2P frame lost source_id stamp")
			}
		})
	}
}

func TestRouteP2PUnauthorizedLoggedOnce(t *testing.T) {
	r := NewWithResolver(fakeLookup{4: &fakePeer{}}, p2pResolver())
	out := captureLogs(t, func() {
		r.Route(1, socketMsg(4, protocol.Networking_Sockets_CONNECTION_REQUEST))
	})
	if n := strings.Count(out, "route p2p from=1 to=4 dropped"); n != 1 {
		t.Fatalf("drop log count = %d, want 1: %q", n, out)
	}
}

func TestRouteP2PDataNotRelayed(t *testing.T) {
	peer := &fakePeer{}
	r := NewWithResolver(fakeLookup{2: peer}, p2pResolver())

	out := captureLogs(t, func() {
		r.Route(1, socketMsg(2, protocol.Networking_Sockets_DATA))
		r.Route(1, messagesMsg(2, protocol.Networking_Messages_DATA))
	})
	if len(peer.frames) != 0 {
		t.Fatalf("DATA payloads relayed: %d frames", len(peer.frames))
	}
	if n := strings.Count(out, "p2p DATA payload dropped"); n != 2 {
		t.Fatalf("DATA drop log count = %d, want 2: %q", n, out)
	}

	// Direct P2P signaling (non-DATA) must still pass through the server.
	r.Route(1, socketMsg(2, protocol.Networking_Sockets_CONNECTION_ACCEPTED))
	r.Route(1, messagesMsg(2, protocol.Networking_Messages_CONNECTION_NEW))
	if len(peer.frames) != 2 {
		t.Fatalf("signaling delivered = %d, want 2", len(peer.frames))
	}
}

func TestRouteNonP2PDestUnaffected(t *testing.T) {
	peer := &fakePeer{}
	r := NewWithResolver(fakeLookup{4: peer}, p2pResolver())
	r.Route(1, &protocol.Common_Message{
		DestId:   4,
		Messages: &protocol.Common_Message_Announce{Announce: &protocol.Announce{}},
	})
	if len(peer.frames) != 1 {
		t.Fatalf("non-p2p targeted frame delivered = %d, want 1", len(peer.frames))
	}
}
