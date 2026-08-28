package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/joey/lan-nicknames/internal/config"
	"github.com/joey/lan-nicknames/internal/store"
)

func TestHelpExplainsBackgroundInstallation(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"help"}, &output, &output); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"lan-nick install",
		"lan-nick uninstall",
		"sudo lan-nick install",
		"Administrator terminal",
		"starts it at boot",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("help does not contain %q:\n%s", expected, output.String())
		}
	}
}

func TestGroupPersistsAssignment(t *testing.T) {
	t.Setenv("LAN_NICK_STATE_DIR", t.TempDir())
	var output bytes.Buffer
	if err := run([]string{"group", " Upstairs "}, &output, &output); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Group != "Upstairs" {
		t.Fatalf("saved group = %q, want Upstairs", cfg.Group)
	}
	if output.String() != "Group: Upstairs\n" {
		t.Fatalf("group output = %q, want %q", output.String(), "Group: Upstairs\n")
	}
}

func TestPrintPeerMapGroupsPeersAndLeavesUngroupedLast(t *testing.T) {
	peers := []store.Peer{
		{ID: "office", Nickname: "Office", Alias: "office", Group: "Upstairs", Addresses: map[string]time.Time{"10.0.0.2": {}}},
		{ID: "printer", Nickname: "Printer", Alias: "printer", Addresses: map[string]time.Time{"10.0.0.4": {}}},
		{ID: "kitchen", Nickname: "Kitchen", Alias: "kitchen", Group: "Downstairs", Addresses: map[string]time.Time{"10.0.0.3": {}}},
	}
	var output bytes.Buffer
	printPeerMap(&output, peers)
	const expected = "Downstairs:\n  kitchen\t10.0.0.3\tKitchen\n\n" +
		"Upstairs:\n  office\t10.0.0.2\tOffice\n\n" +
		"(Ungrouped):\n  printer\t10.0.0.4\tPrinter\n"
	if output.String() != expected {
		t.Fatalf("map output:\n%s\nwant:\n%s", output.String(), expected)
	}
}
