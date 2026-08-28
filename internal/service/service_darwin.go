//go:build darwin

package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	darwinExecutable = "/Library/PrivilegedHelperTools/lan-nick"
	launchdPlist     = "/Library/LaunchDaemons/dev.lan-nick.agent.plist"
)

func ManagerName() string { return "launchd" }

func InstallPath() string { return darwinExecutable }

func Install(source, stateDir string) error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges required; run 'sudo lan-nick install'")
	}
	if err := validateInstallInputs(source, stateDir); err != nil {
		return err
	}
	owner, err := stateOwner()
	if err != nil {
		return err
	}

	// Stop the old process before replacing its executable.
	if err := bootoutLaunchd(); err != nil {
		return err
	}
	if err := installExecutable(source, darwinExecutable); err != nil {
		return err
	}
	if err := writeDefinition(launchdPlist, renderLaunchd(darwinExecutable, stateDir, owner)); err != nil {
		return err
	}
	_ = exec.Command("/bin/launchctl", "enable", "system/"+launchdLabel).Run()
	if output, err := exec.Command("/bin/launchctl", "bootstrap", "system", launchdPlist).CombinedOutput(); err != nil {
		return fmt.Errorf("start launchd service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func Uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("administrator privileges required; run 'sudo lan-nick uninstall'")
	}
	result := bootoutLaunchd()
	for _, path := range []string{launchdPlist, darwinExecutable} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) && result == nil {
			result = fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return result
}

func bootoutLaunchd() error {
	output, err := exec.Command("/bin/launchctl", "bootout", "system/"+launchdLabel).CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if strings.Contains(message, "Could not find service") || strings.Contains(message, "No such process") {
		return nil
	}
	return fmt.Errorf("stop launchd service: %w: %s", err, message)
}
