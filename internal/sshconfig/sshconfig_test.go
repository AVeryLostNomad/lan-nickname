package sshconfig

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joey/lan-nicknames/internal/store"
)

const testHostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN5jmvbSg4KOMAzZzqDLbP2vZ1b93I8+JHwbpdDa9rxZ"

func TestSyncInstallsStrictStableTrustAndPreservesUnmanagedContent(t *testing.T) {
	directory := t.TempDir()
	paths := Paths{
		Config:     filepath.Join(directory, "ssh_config"),
		KnownHosts: filepath.Join(directory, "ssh_known_hosts"),
		NullDevice: "/dev/null",
	}
	if err := os.WriteFile(paths.Config, []byte("Host existing\n  Port 2222\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.KnownHosts, []byte("existing ssh-ed25519 AAAA\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot := snapshotForAddress("192.168.1.91")
	if err := Sync(paths, snapshot); err != nil {
		t.Fatal(err)
	}
	config := readTestFile(t, paths.Config)
	for _, expected := range []string{
		"Host dafni",
		"AddressFamily inet",
		"CheckHostIP no",
		"HostKeyAlias lan-nick-00112233445566778899aabbccddeeff",
		"StrictHostKeyChecking yes",
		"UserKnownHostsFile \"/dev/null\"",
		"Host existing",
	} {
		if !bytes.Contains(config, []byte(expected)) {
			t.Fatalf("SSH config does not contain %q:\n%s", expected, config)
		}
	}
	knownHosts := readTestFile(t, paths.KnownHosts)
	if !bytes.Contains(knownHosts, []byte("lan-nick-00112233445566778899aabbccddeeff "+testHostKey)) {
		t.Fatalf("known-hosts file does not trust the advertised key:\n%s", knownHosts)
	}
	if !bytes.Contains(knownHosts, []byte("existing ssh-ed25519 AAAA")) {
		t.Fatalf("known-hosts file lost unmanaged content:\n%s", knownHosts)
	}

	if err := Sync(paths, snapshotForAddress("192.168.1.204")); err != nil {
		t.Fatal(err)
	}
	if updated := readTestFile(t, paths.Config); !bytes.Equal(updated, config) {
		t.Fatalf("SSH trust changed after an IP address change:\nbefore:\n%s\nafter:\n%s", config, updated)
	}

	if err := Sync(paths, store.Snapshot{}); err != nil {
		t.Fatal(err)
	}
	if cleaned := string(readTestFile(t, paths.Config)); cleaned != "Host existing\n  Port 2222\n" {
		t.Fatalf("SSH config cleanup did not preserve unmanaged content: %q", cleaned)
	}
	if cleaned := string(readTestFile(t, paths.KnownHosts)); cleaned != "existing ssh-ed25519 AAAA\n" {
		t.Fatalf("known-hosts cleanup did not preserve unmanaged content: %q", cleaned)
	}
}

func TestSyncOmitsCollidingAliases(t *testing.T) {
	directory := t.TempDir()
	paths := Paths{
		Config:     filepath.Join(directory, "ssh_config"),
		KnownHosts: filepath.Join(directory, "ssh_known_hosts"),
		NullDevice: "/dev/null",
	}
	snapshot := store.Snapshot{Peers: []store.Peer{
		{ID: "00112233445566778899aabbccddeeff", Alias: "dafni", SSHHostKey: testHostKey},
		{ID: "ffeeddccbbaa99887766554433221100", Alias: "dafni", SSHHostKey: testHostKey},
	}}
	if err := Sync(paths, snapshot); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Config, paths.KnownHosts} {
		contents, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if strings.Contains(string(contents), "dafni") || strings.Contains(string(contents), "lan-nick-") {
			t.Fatalf("%s contains trust for a colliding alias:\n%s", path, contents)
		}
	}
}

func snapshotForAddress(address string) store.Snapshot {
	return store.Snapshot{Peers: []store.Peer{{
		ID:         "00112233445566778899aabbccddeeff",
		Nickname:   "Dafni",
		Alias:      "dafni",
		SSHHostKey: testHostKey,
		Addresses:  map[string]time.Time{address: time.Now()},
	}}}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}
