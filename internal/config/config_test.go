package config

import (
	"os/user"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAlias(t *testing.T) {
	tests := map[string]string{
		"Dafni":                  "dafni",
		"Living Room Mac":        "living-room-mac",
		"  punctuation...works ": "punctuation-works",
		"localhost":              "lan-localhost",
		"日本語":                    "lan-machine",
	}
	for nickname, expected := range tests {
		t.Run(nickname, func(t *testing.T) {
			if actual := Alias(nickname); actual != expected {
				t.Fatalf("Alias(%q) = %q, want %q", nickname, actual, expected)
			}
		})
	}
}

func TestValidateRejectsHostFileControlCharacters(t *testing.T) {
	cfg := Config{ID: "00112233445566778899aabbccddeeff", Nickname: "safe\n127.0.0.1 bad"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted a nickname containing a newline")
	}
}

func TestValidateRejectsGroupControlCharacters(t *testing.T) {
	cfg := Config{
		ID:       "00112233445566778899aabbccddeeff",
		Nickname: "safe",
		Group:    "Upstairs\n(Ungrouped)",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted a group containing a newline")
	}
}

func TestDirUsesInvokingUsersNativeConfigDirectoryUnderSudo(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sudo path handling is Unix-only")
	}
	account, err := user.Current()
	if err != nil || account.Username == "root" {
		t.Skip("test requires a non-root current user")
	}
	t.Setenv("LAN_NICK_STATE_DIR", "")
	t.Setenv("SUDO_USER", account.Username)
	actual, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(account.HomeDir, ".config", "lan-nick")
	if runtime.GOOS == "darwin" {
		expected = filepath.Join(account.HomeDir, "Library", "Application Support", "lan-nick")
	}
	if actual != expected {
		t.Fatalf("Dir() = %q, want %q", actual, expected)
	}
}
