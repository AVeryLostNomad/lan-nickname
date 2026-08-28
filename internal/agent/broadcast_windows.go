//go:build windows

package agent

import (
	"net"

	"golang.org/x/sys/windows"
)

func enableBroadcast(connection *net.UDPConn) error {
	raw, err := connection.SyscallConn()
	if err != nil {
		return err
	}
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		socketErr = windows.SetsockoptInt(windows.Handle(fd), windows.SOL_SOCKET, windows.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return socketErr
}
