package protocol

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/joey/lan-nicknames/internal/config"
)

const (
	Version             = 1
	maxAnnouncementSize = 1536
)

type Announcement struct {
	Version    int    `json:"v"`
	ID         string `json:"id"`
	Nickname   string `json:"nickname"`
	Alias      string `json:"alias"`
	Group      string `json:"group,omitempty"`
	SentAt     int64  `json:"sent_at"`
	SSHHostKey string `json:"ssh_host_key,omitempty"`
}

func New(cfg config.Config, now time.Time) Announcement {
	return Announcement{
		Version:  Version,
		ID:       cfg.ID,
		Nickname: cfg.Nickname,
		Alias:    config.Alias(cfg.Nickname),
		Group:    cfg.Group,
		SentAt:   now.Unix(),
	}
}

func (announcement Announcement) Encode() ([]byte, error) {
	if err := announcement.Validate(time.Now()); err != nil {
		return nil, err
	}
	return json.Marshal(announcement)
}

func Decode(payload []byte, now time.Time) (Announcement, error) {
	if len(payload) > maxAnnouncementSize {
		return Announcement{}, fmt.Errorf("announcement exceeds %d bytes", maxAnnouncementSize)
	}
	var announcement Announcement
	if err := json.Unmarshal(payload, &announcement); err != nil {
		return Announcement{}, fmt.Errorf("decode announcement: %w", err)
	}
	if err := announcement.Validate(now); err != nil {
		return Announcement{}, err
	}
	return announcement, nil
}

func (announcement Announcement) Validate(_ time.Time) error {
	if announcement.Version != Version {
		return fmt.Errorf("unsupported protocol version %d", announcement.Version)
	}
	if len(announcement.ID) != 32 {
		return errors.New("invalid machine id")
	}
	if _, err := hex.DecodeString(announcement.ID); err != nil {
		return errors.New("invalid machine id")
	}
	if len([]byte(announcement.Nickname)) == 0 || len([]byte(announcement.Nickname)) > 128 {
		return errors.New("invalid nickname length")
	}
	if announcement.Alias != config.Alias(announcement.Nickname) {
		return errors.New("alias does not match nickname")
	}
	if err := config.ValidateGroup(announcement.Group); err != nil {
		return err
	}
	if err := ValidateSSHHostKey(announcement.SSHHostKey); err != nil {
		return err
	}
	return nil
}
