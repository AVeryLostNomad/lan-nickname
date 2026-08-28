//go:build windows

package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/joey/lan-nicknames/internal/agent"
)

func ManagerName() string { return "Windows Service Manager" }

func InstallPath() string {
	programFiles := os.Getenv("ProgramFiles")
	if programFiles == "" {
		programFiles = `C:\Program Files`
	}
	return filepath.Join(programFiles, "lan-nick", "lan-nick.exe")
}

func Install(source, stateDir string) error {
	if err := validateInstallInputs(source, stateDir); err != nil {
		return err
	}
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Windows Service Manager (run from an Administrator terminal): %w", err)
	}
	defer manager.Disconnect()
	if err := removeWindowsService(manager); err != nil {
		return err
	}
	_ = eventlog.Remove(windowsName)
	if err := installExecutable(source, InstallPath()); err != nil {
		return err
	}
	installed, err := manager.CreateService(windowsName, InstallPath(), mgr.Config{
		DisplayName:      DisplayName,
		Description:      "Advertises and resolves machine nicknames on the local network.",
		StartType:        mgr.StartAutomatic,
		ErrorControl:     mgr.ErrorNormal,
		DelayedAutoStart: true,
	}, "service-run", stateDir)
	if err != nil {
		return fmt.Errorf("create Windows service (run from an Administrator terminal): %w", err)
	}
	defer installed.Close()
	if err := eventlog.InstallAsEventCreate(windowsName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		_ = installed.Delete()
		return fmt.Errorf("install Windows event log source: %w", err)
	}
	if err := configureWindowsFirewall(); err != nil {
		_ = eventlog.Remove(windowsName)
		_ = installed.Delete()
		return err
	}
	if err := installed.Start(); err != nil {
		_ = eventlog.Remove(windowsName)
		removeWindowsFirewallRule()
		_ = installed.Delete()
		return fmt.Errorf("start Windows service: %w", err)
	}
	return nil
}

func Uninstall() error {
	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to Windows Service Manager (run from an Administrator terminal): %w", err)
	}
	defer manager.Disconnect()
	if err := removeWindowsService(manager); err != nil {
		return err
	}
	_ = eventlog.Remove(windowsName)
	removeWindowsFirewallRule()
	if err := os.Remove(InstallPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove service executable %s: %w", InstallPath(), err)
	}
	_ = os.Remove(filepath.Dir(InstallPath()))
	return nil
}

func configureWindowsFirewall() error {
	removeWindowsFirewallRule()
	arguments := []string{
		"advfirewall", "firewall", "add", "rule",
		"name=lan-nick LAN discovery",
		"dir=in",
		"action=allow",
		"protocol=UDP",
		fmt.Sprintf("localport=%d", agent.DefaultPort),
		"program=" + InstallPath(),
		"profile=private",
		"enable=yes",
	}
	output, err := exec.Command("netsh", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("add private-network Windows Firewall rule: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func removeWindowsFirewallRule() {
	_ = exec.Command(
		"netsh", "advfirewall", "firewall", "delete", "rule",
		"name=lan-nick LAN discovery",
	).Run()
}

func removeWindowsService(manager *mgr.Mgr) error {
	installed, err := manager.OpenService(windowsName)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open Windows service: %w", err)
	}
	if err := stopWindowsService(installed); err != nil {
		installed.Close()
		return err
	}
	if err := installed.Delete(); err != nil {
		installed.Close()
		return fmt.Errorf("delete Windows service: %w", err)
	}
	installed.Close()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		candidate, err := manager.OpenService(windowsName)
		if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
			return nil
		}
		if err == nil {
			candidate.Close()
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for Windows service deletion")
}

func stopWindowsService(installed *mgr.Service) error {
	status, err := installed.Query()
	if err != nil {
		return fmt.Errorf("query Windows service: %w", err)
	}
	if status.State == svc.Stopped {
		return nil
	}
	status, err = installed.Control(svc.Stop)
	if errors.Is(err, windows.ERROR_SERVICE_NOT_ACTIVE) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stop Windows service: %w", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for Windows service to stop")
		}
		time.Sleep(250 * time.Millisecond)
		status, err = installed.Query()
		if err != nil {
			return fmt.Errorf("query stopping Windows service: %w", err)
		}
	}
	return nil
}

type windowsHandler struct {
	log *eventlog.Log
}

func (handler *windowsHandler) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const accepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errors := make(chan error, 1)
	go func() {
		errors <- agent.Run(ctx, agent.Options{SyncHosts: true, Log: eventLogWriter{log: handler.log}})
	}()
	status := svc.Status{State: svc.Running, Accepts: accepted}
	changes <- status

	for {
		select {
		case err := <-errors:
			if err != nil {
				handler.log.Error(1, fmt.Sprintf("lan-nick agent failed: %v", err))
				return true, 1
			}
			return false, 0
		case request := <-requests:
			switch request.Cmd {
			case svc.Interrogate:
				changes <- status
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				select {
				case err := <-errors:
					if err != nil {
						handler.log.Error(1, fmt.Sprintf("lan-nick shutdown failed: %v", err))
						return true, 1
					}
					return false, 0
				case <-time.After(20 * time.Second):
					handler.log.Error(1, "lan-nick shutdown timed out")
					return true, 1
				}
			default:
				handler.log.Warning(1, fmt.Sprintf("unexpected service control request %d", request.Cmd))
			}
		}
	}
}

type eventLogWriter struct {
	log *eventlog.Log
}

func (writer eventLogWriter) Write(payload []byte) (int, error) {
	if message := strings.TrimSpace(string(payload)); message != "" {
		writer.log.Warning(2, message)
	}
	return len(payload), nil
}

func Run(stateDir string) error {
	if stateDir == "" || !filepath.IsAbs(stateDir) {
		return fmt.Errorf("Windows service state directory must be absolute")
	}
	if err := os.Setenv("LAN_NICK_STATE_DIR", stateDir); err != nil {
		return fmt.Errorf("set service state directory: %w", err)
	}
	log, err := eventlog.Open(windowsName)
	if err != nil {
		return fmt.Errorf("open Windows event log: %w", err)
	}
	defer log.Close()
	log.Info(1, "lan-nick service starting")
	if err := svc.Run(windowsName, &windowsHandler{log: log}); err != nil {
		return fmt.Errorf("run Windows service: %w", err)
	}
	log.Info(1, "lan-nick service stopped")
	return nil
}
