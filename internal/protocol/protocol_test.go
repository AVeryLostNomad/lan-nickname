package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

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
