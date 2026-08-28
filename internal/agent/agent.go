package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/net/ipv4"

	"github.com/joey/lan-nicknames/internal/config"
	lanhosts "github.com/joey/lan-nicknames/internal/hosts"
	lannetwork "github.com/joey/lan-nicknames/internal/network"
	"github.com/joey/lan-nicknames/internal/protocol"
	"github.com/joey/lan-nicknames/internal/sshconfig"
	"github.com/joey/lan-nicknames/internal/store"
)

const (
	DefaultPort     = 47777
	DefaultInterval = 5 * time.Second
	DefaultTTL      = 16 * time.Second
	multicastGroup  = "239.255.77.77"
)

type Options struct {
	Port      int
	Interval  time.Duration
	TTL       time.Duration
	HostsPath string
	SSHPaths  sshconfig.Paths
	SyncHosts bool
	Log       io.Writer
}

type received struct {
	announcement protocol.Announcement
	address      net.IP
}

func Run(ctx context.Context, options Options) error {
	if options.Port == 0 {
		options.Port = DefaultPort
	}
	if options.Interval == 0 {
		options.Interval = DefaultInterval
	}
	if options.TTL == 0 {
		options.TTL = DefaultTTL
	}
	if options.HostsPath == "" {
		options.HostsPath = lanhosts.DefaultPath()
	}
	if options.SSHPaths == (sshconfig.Paths{}) {
		options.SSHPaths = sshconfig.DefaultPaths()
	}
	if options.Log == nil {
		options.Log = io.Discard
	}
	if options.Interval < time.Second {
		return fmt.Errorf("announcement interval must be at least 1s")
	}
	if options.TTL <= options.Interval*2 {
		return fmt.Errorf("peer TTL must be more than twice the announcement interval")
	}

	sshHostKey, err := loadSSHHostKey()
	if err != nil {
		fmt.Fprintf(options.Log, "lan-nick: SSH host key discovery disabled: %v\n", err)
	}

	peers, err := store.Load()
	if err != nil {
		return err
	}
	peers.Prune(time.Now().Add(-options.TTL))

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: options.Port})
	if err != nil {
		return fmt.Errorf("listen for LAN announcements on UDP %d: %w", options.Port, err)
	}
	defer udp.Close()
	if err := enableBroadcast(udp); err != nil {
		return fmt.Errorf("enable UDP broadcasts: %w", err)
	}
	packet := ipv4.NewPacketConn(udp)
	defer packet.Close()
	if err := packet.SetMulticastTTL(1); err != nil {
		return fmt.Errorf("set multicast TTL: %w", err)
	}
	if err := packet.SetMulticastLoopback(true); err != nil {
		return fmt.Errorf("enable multicast loopback: %w", err)
	}

	group := &net.UDPAddr{IP: net.ParseIP(multicastGroup), Port: options.Port}
	joined := make(map[int]struct{})
	var sendMu sync.Mutex
	joinInterfaces := func() ([]lannetwork.Interface, error) {
		interfaces, err := lannetwork.MulticastInterfaces()
		if err != nil {
			return nil, err
		}
		for _, local := range interfaces {
			if _, found := joined[local.Interface.Index]; found {
				continue
			}
			if err := packet.JoinGroup(&local.Interface, group); err != nil {
				fmt.Fprintf(options.Log, "lan-nick: cannot join multicast on %s: %v\n", local.Interface.Name, err)
				continue
			}
			joined[local.Interface.Index] = struct{}{}
		}
		return interfaces, nil
	}

	incoming := make(chan received, 64)
	readErrors := make(chan error, 1)
	go receive(ctx, packet, incoming, readErrors)

	advertise := func() error {
		cfg, err := config.LoadOrCreate()
		if err != nil {
			return err
		}
		interfaces, err := joinInterfaces()
		if err != nil {
			return err
		}
		announcement := protocol.New(cfg, time.Now())
		announcement.SSHHostKey = sshHostKey
		payload, err := announcement.Encode()
		if err != nil {
			return err
		}
		for _, local := range interfaces {
			broadcast := &net.UDPAddr{IP: local.Broadcast, Port: options.Port}
			sendMu.Lock()
			multicastErr := packet.SetMulticastInterface(&local.Interface)
			if multicastErr == nil {
				_, multicastErr = packet.WriteTo(payload, nil, group)
			}
			_, broadcastErr := packet.WriteTo(payload, nil, broadcast)
			sendMu.Unlock()
			if multicastErr != nil {
				fmt.Fprintf(options.Log, "lan-nick: cannot announce via multicast on %s: %v\n", local.Interface.Name, multicastErr)
			}
			if broadcastErr != nil {
				fmt.Fprintf(options.Log, "lan-nick: cannot announce via broadcast on %s: %v\n", local.Interface.Name, broadcastErr)
			}
			if multicastErr == nil || broadcastErr == nil {
				peers.Observe(announcement, local.IPv4, time.Now())
			}
		}
		return nil
	}

	flush := func() {
		peers.Prune(time.Now().Add(-options.TTL))
		if err := peers.Save(); err != nil {
			fmt.Fprintf(options.Log, "lan-nick: save peer map: %v\n", err)
		}
		if options.SyncHosts {
			snapshot := peers.Snapshot()
			if err := lanhosts.Sync(options.HostsPath, snapshot); err != nil {
				fmt.Fprintf(options.Log, "lan-nick: sync host aliases: %v\n", err)
			}
			if err := sshconfig.Sync(options.SSHPaths, snapshot); err != nil {
				fmt.Fprintf(options.Log, "lan-nick: sync SSH aliases: %v\n", err)
			}
		}
	}

	if err := advertise(); err != nil {
		return err
	}
	flush()
	announceTicker := time.NewTicker(options.Interval)
	defer announceTicker.Stop()
	flushTicker := time.NewTicker(time.Second)
	defer flushTicker.Stop()
	interfaceTicker := time.NewTicker(10 * time.Second)
	defer interfaceTicker.Stop()
	dirty := false

	for {
		select {
		case <-ctx.Done():
			// A stopped agent must not leave aliases pointing at machines it can
			// no longer verify. A forced process kill can still leave entries;
			// the next agent run replaces the managed section.
			peers.Prune(time.Now().Add(time.Hour))
			flush()
			return nil
		case err := <-readErrors:
			if ctx.Err() != nil {
				return nil
			}
			return err
		case item := <-incoming:
			peers.Observe(item.announcement, item.address, time.Now())
			dirty = true
		case <-announceTicker.C:
			if err := advertise(); err != nil {
				fmt.Fprintf(options.Log, "lan-nick: announce: %v\n", err)
			}
			dirty = true
		case <-interfaceTicker.C:
			if _, err := joinInterfaces(); err != nil {
				fmt.Fprintf(options.Log, "lan-nick: refresh interfaces: %v\n", err)
			}
		case <-flushTicker.C:
			if dirty {
				flush()
				dirty = false
			}
		}
	}
}

func receive(ctx context.Context, packet *ipv4.PacketConn, incoming chan<- received, errors chan<- error) {
	buffer := make([]byte, 2048)
	for {
		count, _, source, err := packet.ReadFrom(buffer)
		if err != nil {
			select {
			case errors <- fmt.Errorf("read LAN announcement: %w", err):
			default:
			}
			return
		}
		udpSource, ok := source.(*net.UDPAddr)
		if !ok || udpSource.IP.To4() == nil || !udpSource.IP.IsGlobalUnicast() || udpSource.IP.IsLoopback() {
			continue
		}
		announcement, err := protocol.Decode(buffer[:count], time.Now())
		if err != nil {
			continue
		}
		select {
		case incoming <- received{announcement: announcement, address: udpSource.IP.To4()}:
		case <-ctx.Done():
			return
		}
	}
}
