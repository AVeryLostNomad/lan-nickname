package network

import (
	"net"
	"testing"
)

func TestBroadcastAddress(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
		mask net.IPMask
		want string
	}{
		{name: "slash 24", ip: net.IPv4(192, 168, 1, 91).To4(), mask: net.CIDRMask(24, 32), want: "192.168.1.255"},
		{name: "slash 20", ip: net.IPv4(192, 168, 16, 42).To4(), mask: net.CIDRMask(20, 32), want: "192.168.31.255"},
		{name: "slash 32", ip: net.IPv4(192, 168, 1, 91).To4(), mask: net.CIDRMask(32, 32), want: "192.168.1.91"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := broadcastAddress(test.ip, test.mask).String(); got != test.want {
				t.Fatalf("broadcastAddress(%s, %v) = %s, want %s", test.ip, test.mask, got, test.want)
			}
		})
	}
}
