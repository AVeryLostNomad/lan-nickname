package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	DisplayName  = "lan-nick LAN discovery"
	launchdLabel = "dev.lan-nick.agent"
	windowsName  = "LanNick"
)

type StateOwner struct {
	UID string
	GID string
}

func stateOwner() (StateOwner, error) {
	owner := StateOwner{UID: os.Getenv("SUDO_UID"), GID: os.Getenv("SUDO_GID")}
	if owner.UID == "" && owner.GID == "" {
		return owner, nil
	}
	if owner.UID == "" || owner.GID == "" {
		return StateOwner{}, fmt.Errorf("SUDO_UID and SUDO_GID must either both be set or both be empty")
	}
	if _, err := strconv.ParseUint(owner.UID, 10, 32); err != nil {
		return StateOwner{}, fmt.Errorf("parse SUDO_UID: %w", err)
	}
	if _, err := strconv.ParseUint(owner.GID, 10, 32); err != nil {
		return StateOwner{}, fmt.Errorf("parse SUDO_GID: %w", err)
	}
	return owner, nil
}

func validateInstallInputs(source, stateDir string) error {
	if source == "" || !filepath.IsAbs(source) {
		return fmt.Errorf("service executable path must be absolute")
	}
	if stateDir == "" || !filepath.IsAbs(stateDir) {
		return fmt.Errorf("service state directory must be absolute")
	}
	if strings.ContainsAny(source, "\r\n\x00") || strings.ContainsAny(stateDir, "\r\n\x00") {
		return fmt.Errorf("service paths cannot contain control characters")
	}
	return nil
}

func installExecutable(source, destination string) error {
	if sameFile(source, destination) {
		return nil
	}
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open current executable: %w", err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return fmt.Errorf("inspect current executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("current executable %s is not a regular file", source)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create service binary directory: %w", err)
	}
	temporary := destination + ".tmp"
	output, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create service executable: %w", err)
	}
	_, copyErr := io.Copy(output, input)
	syncErr := output.Sync()
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("copy service executable: %w", copyErr)
	}
	if syncErr != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("flush service executable: %w", syncErr)
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("close service executable: %w", closeErr)
	}
	if err := os.Chmod(temporary, 0o755); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("make service executable runnable: %w", err)
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace service executable: %w", err)
	}
	return nil
}

func writeDefinition(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create service definition directory: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, contents, 0o644); err != nil {
		return fmt.Errorf("write service definition: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("replace service definition: %w", err)
	}
	return nil
}

func sameFile(first, second string) bool {
	firstInfo, firstErr := os.Stat(first)
	secondInfo, secondErr := os.Stat(second)
	if firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo) {
		return true
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(first), filepath.Clean(second))
	}
	return filepath.Clean(first) == filepath.Clean(second)
}
