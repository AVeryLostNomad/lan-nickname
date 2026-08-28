package sshconfig

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/joey/lan-nicknames/internal/protocol"
	"github.com/joey/lan-nicknames/internal/store"
)

const (
	configBeginMarker = "# BEGIN lan-nick SSH aliases (managed; do not edit)"
	configEndMarker   = "# END lan-nick SSH aliases"
	keysBeginMarker   = "# BEGIN lan-nick SSH host keys (managed; do not edit)"
	keysEndMarker     = "# END lan-nick SSH host keys"
)

type Paths struct {
	Config     string
	KnownHosts string
	NullDevice string
}

func DefaultPaths() Paths {
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		root := filepath.Join(programData, "ssh")
		return Paths{
			Config:     filepath.Join(root, "ssh_config"),
			KnownHosts: filepath.Join(root, "ssh_known_hosts"),
			NullDevice: "NUL",
		}
	}
	return Paths{
		Config:     "/etc/ssh/ssh_config.d/90-lan-nick.conf",
		KnownHosts: "/etc/ssh/ssh_known_hosts",
		NullDevice: "/dev/null",
	}
}

func Sync(paths Paths, snapshot store.Snapshot) error {
	if paths.Config == "" || paths.KnownHosts == "" || paths.NullDevice == "" {
		return errors.New("SSH configuration, known-hosts, and null-device paths are required")
	}
	peers := trustedPeers(snapshot)
	if err := syncManagedFile(paths.KnownHosts, keysBeginMarker, keysEndMarker, renderKnownHosts(peers), false); err != nil {
		return err
	}
	if err := syncManagedFile(paths.Config, configBeginMarker, configEndMarker, renderConfig(paths, peers), true); err != nil {
		return err
	}
	return nil
}

func trustedPeers(snapshot store.Snapshot) []store.Peer {
	counts := make(map[string]int)
	for _, peer := range snapshot.Peers {
		counts[peer.Alias]++
	}
	peers := make([]store.Peer, 0, len(snapshot.Peers))
	for _, peer := range snapshot.Peers {
		if counts[peer.Alias] != 1 || peer.SSHHostKey == "" || !validMachineID(peer.ID) {
			continue
		}
		if err := protocol.ValidateSSHHostKey(peer.SSHHostKey); err != nil {
			continue
		}
		peers = append(peers, peer)
	}
	sort.Slice(peers, func(i, j int) bool {
		return peers[i].Alias < peers[j].Alias
	})
	return peers
}

func validMachineID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func renderConfig(paths Paths, peers []store.Peer) string {
	if len(peers) == 0 {
		return ""
	}
	var output strings.Builder
	for _, peer := range peers {
		fmt.Fprintf(&output, "Host %s\n", peer.Alias)
		fmt.Fprintln(&output, "  AddressFamily inet")
		fmt.Fprintln(&output, "  CheckHostIP no")
		fmt.Fprintf(&output, "  GlobalKnownHostsFile %s\n", quotePath(paths.KnownHosts))
		fmt.Fprintf(&output, "  HostKeyAlias lan-nick-%s\n", peer.ID)
		fmt.Fprintln(&output, "  StrictHostKeyChecking yes")
		fmt.Fprintf(&output, "  UserKnownHostsFile %s\n", quotePath(paths.NullDevice))
	}
	fmt.Fprintln(&output, "Host *")
	return output.String()
}

func renderKnownHosts(peers []store.Peer) string {
	if len(peers) == 0 {
		return ""
	}
	var output strings.Builder
	for _, peer := range peers {
		fmt.Fprintf(&output, "lan-nick-%s %s\n", peer.ID, peer.SSHHostKey)
	}
	return output.String()
}

func quotePath(path string) string {
	return strconv.Quote(filepath.ToSlash(path))
}

func syncManagedFile(path, beginMarker, endMarker, body string, prepend bool) error {
	current, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read SSH file %s: %w", path, err)
	}
	if errors.Is(err, os.ErrNotExist) && body == "" {
		return nil
	}
	updated, err := renderManaged(current, beginMarker, endMarker, body, prepend)
	if err != nil {
		return fmt.Errorf("render SSH file %s: %w", path, err)
	}
	if bytes.Equal(current, updated) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create SSH directory %s: %w", filepath.Dir(path), err)
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	temporary := path + ".lan-nick.tmp"
	if err := os.WriteFile(temporary, updated, mode); err != nil {
		return fmt.Errorf("write SSH file %s: %w", path, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace SSH file %s: %w", path, err)
	}
	return nil
}

func renderManaged(current []byte, beginMarker, endMarker, body string, prepend bool) ([]byte, error) {
	lineEnding := "\n"
	if bytes.Contains(current, []byte("\r\n")) {
		lineEnding = "\r\n"
	}
	text := strings.ReplaceAll(string(current), "\r\n", "\n")
	start := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if (start >= 0) != (end >= 0) || (start >= 0 && end < start) {
		return nil, errors.New("file contains an incomplete lan-nick managed section")
	}
	if start >= 0 {
		end += len(endMarker)
		if end < len(text) && text[end] == '\n' {
			end++
		}
		text = text[:start] + text[end:]
	}
	text = strings.Trim(text, "\n")
	if body != "" {
		block := beginMarker + "\n" + body + endMarker
		if text == "" {
			text = block
		} else if prepend {
			text = block + "\n\n" + text
		} else {
			text += "\n\n" + block
		}
	}
	if text != "" {
		text += "\n"
	}
	if lineEnding == "\r\n" {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return []byte(text), nil
}
