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
	blocked     map[[2]uint64]bool
}

func (r fakeResolver) FriendsOf(id uint64) ([]uint64, error) { return r.friends[id], nil }

func (r fakeResolver) LobbiesOf(id uint64) ([]uint64, error) { return r.userLobbies[id], nil }

func (r fakeResolver) LobbyMembers(id uint64) ([]uint64, error) { return r.lobbies[id], nil }

func (r fakeResolver) IsBlocked(a, b uint64) (bool, error) {
	return r.blocked[[2]uint64{a, b}] || r.blocked[[2]uint64{b, a}], nil
}

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

func TestRouteP2PDataRelayedToDestOnly(t *testing.T) {
	dest, lobbyMate, stranger := &fakePeer{}, &fakePeer{}, &fakePeer{}
	r := NewWithResolver(fakeLookup{2: dest, 3: lobbyMate, 4: stranger}, p2pResolver())

	r.Route(1, socketMsg(2, protocol.Networking_Sockets_DATA))
	r.Route(1, messagesMsg(2, protocol.Networking_Messages_DATA))
	if len(dest.frames) != 2 {
		t.Fatalf("DATA to dest relayed = %d, want 2", len(dest.frames))
	}
	if len(lobbyMate.frames) != 0 || len(stranger.frames) != 0 {
		t.Fatalf("DATA broadcast: lobby mate=%d stranger=%d, want 0/0",
			len(lobbyMate.frames), len(stranger.frames))
	}
	for _, frame := range dest.frames {
		if got := delivered(t, frame).GetSourceId(); got != 1 {
			t.Fatalf("relayed source_id = %d, want 1", got)
		}
	}

	// Direct P2P signaling (non-DATA) still passes through the server.
	r.Route(1, socketMsg(2, protocol.Networking_Sockets_CONNECTION_ACCEPTED))
	r.Route(1, messagesMsg(2, protocol.Networking_Messages_CONNECTION_NEW))
	if len(dest.frames) != 4 || len(lobbyMate.frames) != 0 {
		t.Fatalf("signaling relayed = %d, broadcast = %d, want 4/0",
			len(dest.frames), len(lobbyMate.frames))
	}
}

func TestRouteP2PDataNoDestNotBroadcast(t *testing.T) {
	friend, lobbyMate := &fakePeer{}, &fakePeer{}
	r := NewWithResolver(fakeLookup{2: friend, 3: lobbyMate}, p2pResolver())

	r.Route(1, socketMsg(0, protocol.Networking_Sockets_DATA))
	r.Route(1, messagesMsg(0, protocol.Networking_Messages_DATA))
	if len(friend.frames) != 0 || len(lobbyMate.frames) != 0 {
		t.Fatalf("untargeted DATA broadcast: friend=%d lobby mate=%d, want 0/0",
			len(friend.frames), len(lobbyMate.frames))
	}
}

func TestRouteP2PDataUnauthorizedDropped(t *testing.T) {
	stranger := &fakePeer{}
	r := NewWithResolver(fakeLookup{4: stranger}, p2pResolver())

	out := captureLogs(t, func() {
		r.Route(1, socketMsg(4, protocol.Networking_Sockets_DATA))
		r.Route(1, messagesMsg(4, protocol.Networking_Messages_DATA))
	})
	if len(stranger.frames) != 0 {
		t.Fatalf("unauthorized DATA relayed: %d frames", len(stranger.frames))
	}
	if n := strings.Count(out, "route p2p from=1 to=4 dropped"); n != 2 {
		t.Fatalf("drop log count = %d, want 2: %q", n, out)
	}
}

func TestRouteP2PDataToSelfDropped(t *testing.T) {
	self := &fakePeer{}
	r := NewWithResolver(fakeLookup{1: self}, p2pResolver())
	r.Route(1, socketMsg(1, protocol.Networking_Sockets_DATA))
	r.Route(1, messagesMsg(1, protocol.Networking_Messages_DATA))
	if len(self.frames) != 0 {
		t.Fatalf("self DATA relayed: %d frames", len(self.frames))
	}
}

func TestRouteP2PDataOfflineDestLogged(t *testing.T) {
	r := NewWithResolver(fakeLookup{}, p2pResolver()) // dest 2 authorized but not live
	out := captureLogs(t, func() {
		r.Route(1, socketMsg(2, protocol.Networking_Sockets_DATA))
	})
	if n := strings.Count(out, "route to=2 offline, dropped"); n != 1 {
		t.Fatalf("offline log count = %d, want 1: %q", n, out)
	}
}

func inviteMsg(dest uint64) *protocol.Common_Message {
	return &protocol.Common_Message{
		DestId: dest,
		Messages: &protocol.Common_Message_FriendMessages{FriendMessages: &protocol.Friend_Messages{
			Type: protocol.Friend_Messages_GAME_INVITE,
		}},
	}
}

func TestRouteInviteAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name    string
		dest    uint64
		blocked bool
		want    int
	}{
		{"friend", 2, false, 1},
		{"stranger", 4, false, 0},
		{"blocked friend", 2, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := &fakePeer{}
			resolver := p2pResolver()
			if tc.blocked {
				resolver.blocked = map[[2]uint64]bool{{1, 2}: true}
			}
			r := NewWithResolver(fakeLookup{tc.dest: peer}, resolver)
			r.Route(1, inviteMsg(tc.dest))
			if got := len(peer.frames); got != tc.want {
				t.Fatalf("delivered = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRouteBlockedPeerCannotUseLobbyP2P(t *testing.T) {
	peer := &fakePeer{}
	resolver := p2pResolver() // 3 is a lobby mate of 1
	resolver.blocked = map[[2]uint64]bool{{3, 1}: true}
	r := NewWithResolver(fakeLookup{3: peer}, resolver)
	r.Route(1, socketMsg(3, protocol.Networking_Sockets_CONNECTION_REQUEST))
	if len(peer.frames) != 0 {
		t.Fatalf("blocked lobby mate received P2P: %d frames", len(peer.frames))
	}
}

func TestRouteInviteBroadcastSkipsBlocked(t *testing.T) {
	friend, blocked := &fakePeer{}, &fakePeer{}
	resolver := fakeResolver{
		friends: map[uint64][]uint64{1: {2, 4}},
		blocked: map[[2]uint64]bool{{1, 4}: true},
	}
	r := NewWithResolver(fakeLookup{2: friend, 4: blocked}, resolver)
	r.Route(1, inviteMsg(0))
	if len(friend.frames) != 1 || len(blocked.frames) != 0 {
		t.Fatalf("broadcast friend=%d blocked=%d, want 1/0", len(friend.frames), len(blocked.frames))
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

func TestRouteLobbyFanoutSkipsBlockedMembers(t *testing.T) {
	ok, blocked := &fakePeer{}, &fakePeer{}
	resolver := fakeResolver{
		lobbies: map[uint64][]uint64{7: {1, 2, 4}},
		blocked: map[[2]uint64]bool{{4, 1}: true},
	}
	r := NewWithResolver(fakeLookup{2: ok, 4: blocked}, resolver)
	r.Route(1, &protocol.Common_Message{
		Messages: &protocol.Common_Message_Lobby{Lobby: &protocol.Lobby{RoomId: 7}},
	})
	r.Route(1, &protocol.Common_Message{
		Messages: &protocol.Common_Message_LobbyMessages{LobbyMessages: &protocol.Lobby_Messages{Id: 7}},
	})
	if len(ok.frames) != 2 || len(blocked.frames) != 0 {
		t.Fatalf("lobby fanout allowed=%d blocked=%d, want 2/0", len(ok.frames), len(blocked.frames))
	}
	for _, frame := range ok.frames {
		if delivered(t, frame).GetSourceId() != 1 {
			t.Fatal("lobby frame lost source_id stamp")
		}
	}
}
