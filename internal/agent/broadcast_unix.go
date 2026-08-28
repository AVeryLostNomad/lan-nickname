//go:build darwin || linux

package agent

import (
	"net"

	"golang.org/x/sys/unix"
)

func enableBroadcast(connection *net.UDPConn) error {
	raw, err := connection.SyscallConn()
	if err != nil {
		return err
	}
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		socketErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return socketErr
}
