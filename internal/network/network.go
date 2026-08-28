package network

import (
	"fmt"
	"net"
	"sort"
)

type Interface struct {
	Interface net.Interface
	IPv4      net.IP
}

func LocalIPs() ([]net.IP, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	seen := make(map[string]net.IP)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || !ip.IsGlobalUnicast() || ip.IsLoopback() {
				continue
			}
			seen[ip.String()] = ip
		}
	}

	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]net.IP, 0, len(keys))
	for _, key := range keys {
		result = append(result, seen[key])
	}
	return result, nil
}

func MulticastInterfaces() ([]Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	var result []Interface
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagMulticast == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.To4() != nil && ip.IsGlobalUnicast() {
				result = append(result, Interface{Interface: iface, IPv4: ip.To4()})
				break
			}
		}
	}
	return result, nil
}
