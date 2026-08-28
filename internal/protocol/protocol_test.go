package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

const testSSHHostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIN5jmvbSg4KOMAzZzqDLbP2vZ1b93I8+JHwbpdDa9rxZ"

func TestDecodeRejectsClaimedAliasThatDoesNotMatchNickname(t *testing.T) {
	payload, err := json.Marshal(Announcement{
		Version:  Version,
		ID:       "00112233445566778899aabbccddeeff",
		Nickname: "Trusted Name",
		Alias:    "other-name",
		SentAt:   time.Now().Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(payload, time.Now()); err == nil {
		t.Fatal("Decode accepted an alias that did not derive from the nickname")
	}
}

func TestDecodeDoesNotRequireSynchronizedClocks(t *testing.T) {
	announcement := Announcement{
		Version:  Version,
		ID:       "00112233445566778899aabbccddeeff",
		Nickname: "Dafni",
		Alias:    "dafni",
		SentAt:   time.Unix(1, 0).Unix(),
	}
	payload, err := json.Marshal(announcement)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(payload, time.Now()); err != nil {
		t.Fatalf("Decode rejected a valid peer with a different wall clock: %v", err)
	}
}

func TestParseSSHHostKeyNormalizesPublicKeyFile(t *testing.T) {
	key, err := ParseSSHHostKey([]byte(testSSHHostKey + " root@dafni\n"))
	if err != nil {
		t.Fatal(err)
	}
	if key != testSSHHostKey {
		t.Fatalf("ParseSSHHostKey() = %q, want %q", key, testSSHHostKey)
	}
}

func TestValidateSSHHostKeyRejectsMismatchedBlobType(t *testing.T) {
	key := "ssh-rsa " + testSSHHostKey[len("ssh-ed25519 "):]
	if err := ValidateSSHHostKey(key); err == nil {
		t.Fatal("ValidateSSHHostKey accepted a blob for a different key type")
	}
}

func TestAnnouncementRoundTripPreservesMetadata(t *testing.T) {
	announcement := Announcement{
		Version:    Version,
		ID:         "00112233445566778899aabbccddeeff",
		Nickname:   "Dafni",
		Alias:      "dafni",
		Group:      "Upstairs",
		SentAt:     time.Now().Unix(),
		SSHHostKey: testSSHHostKey,
	}
	payload, err := announcement.Encode()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(payload, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if decoded.SSHHostKey != testSSHHostKey {
		t.Fatalf("decoded SSH host key = %q, want %q", decoded.SSHHostKey, testSSHHostKey)
	}
	if decoded.Group != "Upstairs" {
		t.Fatalf("decoded group = %q, want Upstairs", decoded.Group)
	}
}
