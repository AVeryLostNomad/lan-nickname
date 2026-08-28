package hosts

import (
	"strings"
	"testing"
	"time"

	"github.com/joey/lan-nicknames/internal/store"
)

func TestRenderReplacesManagedSectionAndPreservesOtherEntries(t *testing.T) {
	current := []byte("127.0.0.1 localhost\n\n" + beginMarker + "\n10.0.0.9 old\n" + endMarker + "\n")
	snapshot := store.Snapshot{Peers: []store.Peer{{
		ID:       "a",
		Nickname: "Living Room",
		Alias:    "living-room",
		Addresses: map[string]time.Time{
			"192.168.1.20": time.Now(),
			"not-an-ip":    time.Now(),
		},
	}}}

	actual, err := Render(current, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	text := string(actual)
	if !strings.Contains(text, "127.0.0.1 localhost") {
		t.Fatal("Render removed an unmanaged host entry")
	}
	if strings.Contains(text, "10.0.0.9 old") {
		t.Fatal("Render retained a stale managed host entry")
	}
	if !strings.Contains(text, "192.168.1.20\tliving-room\t# Living Room") {
		t.Fatalf("Render did not add the active peer:\n%s", text)
	}
	if strings.Contains(text, "not-an-ip") {
		t.Fatal("Render included an invalid address")
	}
}

func TestRenderDisablesAmbiguousAlias(t *testing.T) {
	snapshot := store.Snapshot{Peers: []store.Peer{
		{ID: "a", Nickname: "Dafni", Alias: "dafni", Addresses: map[string]time.Time{"10.0.0.2": time.Now()}},
		{ID: "b", Nickname: "dafni", Alias: "dafni", Addresses: map[string]time.Time{"10.0.0.3": time.Now()}},
	}}

	actual, err := Render(nil, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	text := string(actual)
	if strings.Contains(text, "10.0.0.2") || strings.Contains(text, "10.0.0.3") {
		t.Fatalf("Render installed an ambiguous alias:\n%s", text)
	}
	if !strings.Contains(text, "collision: dafni") {
		t.Fatalf("Render did not explain the collision:\n%s", text)
	}
}

func TestRenderRejectsIncompleteManagedSection(t *testing.T) {
	if _, err := Render([]byte(beginMarker+"\n"), store.Snapshot{}); err == nil {
		t.Fatal("Render accepted an incomplete managed section")
	}
}
