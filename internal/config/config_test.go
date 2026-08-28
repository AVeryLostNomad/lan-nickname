package config

import "testing"

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
