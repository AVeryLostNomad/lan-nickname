package store

import (
	"net"
	"testing"
	"time"

	"github.com/joey/lan-nicknames/internal/protocol"
)

func TestObserveRenameDropsAddressesFromOldIdentity(t *testing.T) {
	peers := New()
	now := time.Now()
	peers.Observe(protocol.Announcement{ID: "id", Nickname: "Old", Alias: "old"}, net.ParseIP("10.0.0.2"), now)
	peers.Observe(protocol.Announcement{ID: "id", Nickname: "New", Alias: "new"}, net.ParseIP("10.0.0.3"), now)

	snapshot := peers.Snapshot()
	if len(snapshot.Peers) != 1 {
		t.Fatalf("got %d peers, want 1", len(snapshot.Peers))
	}
	peer := snapshot.Peers[0]
	if peer.Nickname != "New" || peer.Alias != "new" {
		t.Fatalf("peer identity = %q/%q, want New/new", peer.Nickname, peer.Alias)
	}
	if _, found := peer.Addresses["10.0.0.2"]; found {
		t.Fatal("renamed peer retained an address from its old identity")
	}
	if _, found := peer.Addresses["10.0.0.3"]; !found {
		t.Fatal("renamed peer lost its current address")
	}
}

func TestPruneExpiresAddressesIndependently(t *testing.T) {
	peers := New()
	now := time.Now()
	announcement := protocol.Announcement{ID: "id", Nickname: "Dafni", Alias: "dafni"}
	peers.Observe(announcement, net.ParseIP("10.0.0.2"), now.Add(-time.Minute))
	peers.Observe(announcement, net.ParseIP("10.0.0.3"), now)
	peers.Prune(now.Add(-10 * time.Second))

	addresses := peers.Snapshot().Peers[0].Addresses
	if _, found := addresses["10.0.0.2"]; found {
		t.Fatal("Prune retained an expired address")
	}
	if _, found := addresses["10.0.0.3"]; !found {
		t.Fatal("Prune removed a current address")
	}
}
