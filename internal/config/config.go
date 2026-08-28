package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

const fileName = "config.json"

type Config struct {
	ID       string `json:"id"`
	Nickname string `json:"nickname"`
	Group    string `json:"group,omitempty"`
}

func Dir() (string, error) {
	if dir := os.Getenv("LAN_NICK_STATE_DIR"); dir != "" {
		return dir, nil
	}

	// sudo normally changes HOME. Keep the machine's nickname in the invoking
	// user's profile so elevated `serve` and ordinary CLI calls share state.
	if runtime.GOOS != "windows" {
		if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" && sudoUser != "root" {
			if account, err := user.Lookup(sudoUser); err == nil && account.HomeDir != "" {
				if runtime.GOOS == "darwin" {
					return filepath.Join(account.HomeDir, "Library", "Application Support", "lan-nick"), nil
				}
				return filepath.Join(account.HomeDir, ".config", "lan-nick"), nil
			}
		}
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find user config directory: %w", err)
	}
	return filepath.Join(base, "lan-nick"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

func LoadOrCreate() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	contents, err := os.ReadFile(path)
	if err == nil {
		var cfg Config
		if err := json.Unmarshal(contents, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse %s: %w", path, err)
		}
		if err := cfg.Validate(); err != nil {
			return Config{}, fmt.Errorf("validate %s: %w", path, err)
		}
		return cfg, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read %s: %w", path, err)
	}

	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unnamed-machine"
	}
	id, err := newID()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{ID: id, Nickname: hostname}
	if err := Save(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := OwnForInvoker(filepath.Dir(path)); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	contents = append(contents, '\n')
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, contents, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if err := OwnForInvoker(temporary); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func (cfg Config) Validate() error {
	if len(cfg.ID) != 32 {
		return errors.New("machine id must contain 32 hexadecimal characters")
	}
	if _, err := hex.DecodeString(cfg.ID); err != nil {
		return errors.New("machine id must contain 32 hexadecimal characters")
	}
	if strings.TrimSpace(cfg.Nickname) == "" {
		return errors.New("nickname cannot be empty")
	}
	if len([]byte(cfg.Nickname)) > 128 {
		return errors.New("nickname cannot exceed 128 bytes")
	}
	for _, r := range cfg.Nickname {
		if unicode.IsControl(r) {
			return errors.New("nickname cannot contain control characters")
		}
	}
	return ValidateGroup(cfg.Group)

}

func ValidateGroup(group string) error {
	if group == "" {
		return nil
	}
	if strings.TrimSpace(group) == "" {
		return errors.New("group cannot be blank")
	}
	if len([]byte(group)) > 128 {
		return errors.New("group cannot exceed 128 bytes")
	}
	for _, r := range group {
		if unicode.IsControl(r) {
			return errors.New("group cannot contain control characters")
		}
	}
	return nil
}

func Alias(nickname string) string {
	var result strings.Builder
	result.Grow(len(nickname))
	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(nickname)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if r <= unicode.MaxASCII {
				result.WriteRune(r)
				lastHyphen = false
			}
			continue
		}
		if result.Len() > 0 && !lastHyphen {
			result.WriteByte('-')
			lastHyphen = true
		}
	}
	alias := strings.Trim(result.String(), "-")
	if alias == "" {
		alias = "lan-machine"
	}
	if len(alias) > 63 {
		alias = strings.TrimRight(alias[:63], "-")
	}
	switch alias {
	case "localhost", "localhost.localdomain", "broadcasthost", "ip6-localhost", "ip6-loopback":
		alias = "lan-" + alias
	}
	return alias
}

// OwnForInvoker keeps state readable and writable by the ordinary user when
// serve runs through sudo or as a root-owned system service.
func OwnForInvoker(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	uidText, gidText := os.Getenv("LAN_NICK_STATE_UID"), os.Getenv("LAN_NICK_STATE_GID")
	if uidText == "" && gidText == "" {
		uidText, gidText = os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID")
	}
	if uidText == "" && gidText == "" {
		return nil
	}
	if uidText == "" || gidText == "" {
		return errors.New("state UID and GID must either both be set or both be empty")
	}
	uid, err := strconv.Atoi(uidText)
	if err != nil {
		return fmt.Errorf("parse state UID: %w", err)
	}
	gid, err := strconv.Atoi(gidText)
	if err != nil {
		return fmt.Errorf("parse state GID: %w", err)
	}
	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("set state ownership for invoking user: %w", err)
	}
	return nil
}

func newID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate machine id: %w", err)
	}
	return hex.EncodeToString(id[:]), nil
}
