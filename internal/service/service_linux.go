//go:build linux

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	linuxExecutable = "/usr/local/libexec/lan-nick"
	systemdUnit     = "/etc/systemd/system/lan-nick.service"
)

func ManagerName() string { return "systemd" }

func InstallPath() string { return linuxExecutable }

func Install(source, stateDir string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges required; run 'sudo lan-nick install'")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemd is required for automatic Linux installation: %w", err)
	}
	if err := validateInstallInputs(source, stateDir); err != nil {
		return err
	}
	owner, err := stateOwner()
	if err != nil {
		return err
	}
	if _, err := os.Stat(systemdUnit); err == nil {
		if err := systemctl("stop", "lan-nick.service"); err != nil {
			return err
		}
	}
	if err := installExecutable(source, linuxExecutable); err != nil {
		return err
	}
	if err := writeDefinition(systemdUnit, renderSystemd(linuxExecutable, stateDir, owner)); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", "--now", "lan-nick.service"); err != nil {
		return err
	}
	return nil
}

func Uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges required; run 'sudo lan-nick uninstall'")
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemd is required for automatic Linux uninstallation: %w", err)
	}
	var result error
	if _, err := os.Stat(systemdUnit); err == nil {
		if err := systemctl("disable", "--now", "lan-nick.service"); err != nil {
			result = err
		}
	}
	for _, path := range []string{systemdUnit, linuxExecutable} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && result == nil {
			result = fmt.Errorf("remove %s: %w", path, err)
		}
	}
	if err := systemctl("daemon-reload"); err != nil && result == nil {
		result = err
	}
	return result
}

func systemctl(arguments ...string) error {
	output, err := exec.Command("systemctl", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
