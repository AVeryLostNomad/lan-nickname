package service

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderLaunchdProducesValidEscapedPlist(t *testing.T) {
	contents := renderLaunchd("/Library/A&B/lan-nick", "/Users/Joey & Dafni/state", StateOwner{UID: "501", GID: "20"})
	var document any
	if err := xml.Unmarshal(contents, &document); err != nil {
		t.Fatalf("launchd definition is not valid XML: %v\n%s", err, contents)
	}
	text := string(contents)
	for _, expected := range []string{
		"/Library/A&amp;B/lan-nick",
		"/Users/Joey &amp; Dafni/state",
		"LAN_NICK_STATE_UID",
		"<string>501</string>",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("launchd definition does not contain %q:\n%s", expected, text)
		}
	}
}

func TestRenderSystemdQuotesPathsAndOwnership(t *testing.T) {
	contents := string(renderSystemd("/usr/local/libexec/lan nick", "/home/joey/My State", StateOwner{UID: "501", GID: "20"}))
	for _, expected := range []string{
		`ExecStart="/usr/local/libexec/lan nick" serve`,
		`Environment="LAN_NICK_STATE_DIR=/home/joey/My State"`,
		`Environment="LAN_NICK_STATE_UID=501"`,
		`Restart=on-failure`,
	} {
		if !strings.Contains(contents, expected) {
			t.Fatalf("systemd definition does not contain %q:\n%s", expected, contents)
		}
	}
}

func TestInstallExecutableCopiesRunnableBinary(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	destination := filepath.Join(directory, "system", "lan-nick")
	if err := os.WriteFile(source, []byte("binary contents"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := installExecutable(source, destination); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "binary contents" {
		t.Fatalf("installed contents = %q", contents)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("installed mode %v is not executable", info.Mode().Perm())
	}
}

func TestStateOwnerRejectsPartialSudoIdentity(t *testing.T) {
	t.Setenv("SUDO_UID", "501")
	t.Setenv("SUDO_GID", "")
	if _, err := stateOwner(); err == nil {
		t.Fatal("stateOwner accepted SUDO_UID without SUDO_GID")
	}
}
