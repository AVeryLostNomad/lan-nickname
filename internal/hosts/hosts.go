package hosts

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/joey/lan-nicknames/internal/store"
)

const (
	beginMarker = "# BEGIN lan-nick (managed; do not edit)"
	endMarker   = "# END lan-nick"
)

func DefaultPath() string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		return filepath.Join(root, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

func Sync(path string, snapshot store.Snapshot) error {
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read hosts file %s: %w", path, err)
	}
	updated, err := Render(current, snapshot)
	if err != nil {
		return err
	}
	if bytes.Equal(current, updated) {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat hosts file %s: %w", path, err)
	}
	temporary := filepath.Join(filepath.Dir(path), ".lan-nick-hosts.tmp")
	if err := os.WriteFile(temporary, updated, info.Mode().Perm()); err != nil {
		return fmt.Errorf("write hosts file %s (run lan-nick serve with administrator privileges): %w", path, err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace hosts file %s (run lan-nick serve with administrator privileges): %w", path, err)
	}
	return nil
}

func Render(current []byte, snapshot store.Snapshot) ([]byte, error) {
	lineEnding := "\n"
	if bytes.Contains(current, []byte("\r\n")) {
		lineEnding = "\r\n"
	}
	text := strings.ReplaceAll(string(current), "\r\n", "\n")
	start := strings.Index(text, beginMarker)
	end := strings.Index(text, endMarker)
	if (start >= 0) != (end >= 0) || (start >= 0 && end < start) {
		return nil, fmt.Errorf("hosts file contains an incomplete lan-nick managed section")
	}
	if start >= 0 {
		end += len(endMarker)
		if end < len(text) && text[end] == '\n' {
			end++
		}
		text = text[:start] + text[end:]
	}
	text = strings.TrimRight(text, "\n")

	var block strings.Builder
	block.WriteString(beginMarker)
	block.WriteByte('\n')
	byAlias := make(map[string][]store.Peer)
	for _, peer := range snapshot.Peers {
		byAlias[peer.Alias] = append(byAlias[peer.Alias], peer)
	}
	aliases := make([]string, 0, len(byAlias))
	for alias := range byAlias {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for _, alias := range aliases {
		peers := byAlias[alias]
		if len(peers) != 1 {
			block.WriteString("# collision: ")
			block.WriteString(alias)
			block.WriteString(" is advertised by multiple machines\n")
			continue
		}
		peer := peers[0]
		addresses := make([]string, 0, len(peer.Addresses))
		for address := range peer.Addresses {
			if ip := net.ParseIP(address); ip != nil && ip.IsGlobalUnicast() && !ip.IsLoopback() {
				addresses = append(addresses, ip.String())
			}
		}
		sort.Strings(addresses)
		for _, address := range addresses {
			fmt.Fprintf(&block, "%s\t%s\t# %s\n", address, alias, peer.Nickname)
		}
	}
	block.WriteString(endMarker)
	block.WriteByte('\n')

	if text != "" {
		text += "\n\n"
	}
	text += block.String()
	if lineEnding == "\r\n" {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	return []byte(text), nil
}
