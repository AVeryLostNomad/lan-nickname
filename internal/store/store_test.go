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

func TestObservePersistsSSHHostKey(t *testing.T) {
	const hostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN5jmvbSg4KOMAzZzqDLbP2vZ1b93I8+JHwbpdDa9rxZ"
	peers := New()
	peers.Observe(protocol.Announcement{
		ID:         "00112233445566778899aabbccddeeff",
		Nickname:   "Dafni",
		Alias:      "dafni",
		SSHHostKey: hostKey,
	}, net.ParseIP("192.168.1.91"), time.Now())

	if got := peers.Snapshot().Peers[0].SSHHostKey; got != hostKey {
		t.Fatalf("stored SSH host key = %q, want %q", got, hostKey)
	}
}

func TestObserveUpdatesGroupWithoutDroppingAddresses(t *testing.T) {
	peers := New()
	now := time.Now()
	announcement := protocol.Announcement{ID: "id", Nickname: "Dafni", Alias: "dafni", Group: "Upstairs"}
	peers.Observe(announcement, net.ParseIP("10.0.0.2"), now)
	announcement.Group = "Downstairs"
	peers.Observe(announcement, net.ParseIP("10.0.0.3"), now)

	peer := peers.Snapshot().Peers[0]
	if peer.Group != "Downstairs" {
		t.Fatalf("stored group = %q, want Downstairs", peer.Group)
	}
	if len(peer.Addresses) != 2 {
		t.Fatalf("group change retained %d addresses, want 2", len(peer.Addresses))
	}
}
