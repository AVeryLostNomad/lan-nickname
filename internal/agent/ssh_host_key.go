package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/joey/lan-nicknames/internal/protocol"
)

func loadSSHHostKey() (string, error) {
	for _, path := range sshHostKeyPaths() {
		contents, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("read SSH host public key %s: %w", path, err)
		}
		key, err := protocol.ParseSSHHostKey(contents)
		if err != nil {
			return "", fmt.Errorf("parse SSH host public key %s: %w", path, err)
		}
		return key, nil
	}
	return "", nil
}

func sshHostKeyPaths() []string {
	root := "/etc/ssh"
	if runtime.GOOS == "windows" {
		root = os.Getenv("ProgramData")
		if root == "" {
			root = `C:\ProgramData`
		}
		root = filepath.Join(root, "ssh")
	}
	return []string{
		filepath.Join(root, "ssh_host_ed25519_key.pub"),
		filepath.Join(root, "ssh_host_ecdsa_key.pub"),
		filepath.Join(root, "ssh_host_rsa_key.pub"),
	}
}
