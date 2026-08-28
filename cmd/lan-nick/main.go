package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/joey/lan-nicknames/internal/agent"
	"github.com/joey/lan-nicknames/internal/config"
	lanhosts "github.com/joey/lan-nicknames/internal/hosts"
	lannetwork "github.com/joey/lan-nicknames/internal/network"
	"github.com/joey/lan-nicknames/internal/service"
	"github.com/joey/lan-nicknames/internal/sshconfig"
	"github.com/joey/lan-nicknames/internal/store"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "lan-nick: %v\n", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		return status(stdout)
	}
	switch arguments[0] {
	case "rename":
		if len(arguments) < 2 {
			return fmt.Errorf("usage: lan-nick rename \"<new name>\"")
		}
		return rename(strings.Join(arguments[1:], " "), stdout)
	case "group":
		if len(arguments) < 2 {
			return fmt.Errorf("usage: lan-nick group \"<name>\"")
		}
		return setGroup(strings.Join(arguments[1:], " "), stdout)
	case "map":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: lan-nick map")
		}
		return printMap(stdout)
	case "install":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: lan-nick install")
		}
		return installBackgroundService(stdout)
	case "uninstall":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: lan-nick uninstall")
		}
		return uninstallBackgroundService(stdout)
	case "serve":
		return serve(arguments[1:], stdout, stderr)
	case "service-run":
		if len(arguments) != 2 {
			return fmt.Errorf("internal service runner requires a state directory")
		}
		return service.Run(arguments[1])
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q; run 'lan-nick help'", arguments[0])
	}
}

func status(output io.Writer) error {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	addresses, err := lannetwork.LocalIPs()
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Nickname: %s\n", cfg.Nickname)
	fmt.Fprintf(output, "Host alias: %s\n", config.Alias(cfg.Nickname))
	fmt.Fprintln(output, "Local IPs:")
	if len(addresses) == 0 {
		fmt.Fprintln(output, "  (none)")
	}
	for _, address := range addresses {
		fmt.Fprintf(output, "  %s\n", address)
	}
	return nil
}

func rename(nickname string, output io.Writer) error {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	cfg.Nickname = strings.TrimSpace(nickname)
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(output, "Nickname: %s\n", cfg.Nickname)
	fmt.Fprintf(output, "Host alias: %s\n", config.Alias(cfg.Nickname))
	return nil
}

func setGroup(name string, output io.Writer) error {
	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	cfg.Group = strings.TrimSpace(name)
	if cfg.Group == "" {
		return fmt.Errorf("group name cannot be empty")
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(output, "Group: %s\n", cfg.Group)
	return nil
}

func printMap(output io.Writer) error {
	snapshot, err := store.ReadSnapshot(agent.DefaultTTL, time.Now())
	if err != nil {
		return err
	}
	if len(snapshot.Peers) == 0 {
		fmt.Fprintln(output, "No active lan-nick machines found. Is the lan-nick service running?")
		return nil
	}
	printPeerMap(output, snapshot.Peers)
	return nil
}

func printPeerMap(output io.Writer, peers []store.Peer) {
	counts := make(map[string]int)
	grouped := make(map[string][]store.Peer)
	groupNames := make([]string, 0)
	for _, peer := range peers {
		counts[peer.Alias]++
		if _, found := grouped[peer.Group]; !found && peer.Group != "" {
			groupNames = append(groupNames, peer.Group)
		}
		grouped[peer.Group] = append(grouped[peer.Group], peer)
	}
	sort.Strings(groupNames)
	if len(grouped[""]) > 0 {
		groupNames = append(groupNames, "")
	}

	for groupIndex, group := range groupNames {
		if groupIndex > 0 {
			fmt.Fprintln(output)
		}
		heading := group
		if heading == "" {
			heading = "(Ungrouped)"
		}
		fmt.Fprintf(output, "%s:\n", heading)
		for _, peer := range grouped[group] {
			addresses := make([]string, 0, len(peer.Addresses))
			for address := range peer.Addresses {
				addresses = append(addresses, address)
			}
			sort.Strings(addresses)
			alias := peer.Alias
			if counts[peer.Alias] > 1 {
				alias += " (collision; host alias disabled)"
			}
			fmt.Fprintf(output, "  %s\t%s\t%s\n", alias, strings.Join(addresses, ", "), peer.Nickname)
		}
	}
}

func installBackgroundService(output io.Writer) error {
	if _, err := config.LoadOrCreate(); err != nil {
		return err
	}
	stateDir, err := config.Dir()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("find current executable: %w", err)
	}
	if err := service.Install(executable, stateDir); err != nil {
		return err
	}
	fmt.Fprintf(output, "Installed and started lan-nick with %s.\\n", service.ManagerName())
	fmt.Fprintf(output, "Service executable: %s\\n", service.InstallPath())
	fmt.Fprintf(output, "State directory: %s\\n", stateDir)
	return nil
}

func uninstallBackgroundService(output io.Writer) error {
	if err := service.Uninstall(); err != nil {
		return err
	}
	hostsErr := lanhosts.Sync(lanhosts.DefaultPath(), store.Snapshot{})
	sshErr := sshconfig.Sync(sshconfig.DefaultPaths(), store.Snapshot{})
	if hostsErr != nil && sshErr != nil {
		return fmt.Errorf("service removed, but host and SSH alias cleanup failed: hosts: %v; SSH: %w", hostsErr, sshErr)
	}
	if hostsErr != nil {
		return fmt.Errorf("service removed, but host alias cleanup failed: %w", hostsErr)
	}
	if sshErr != nil {
		return fmt.Errorf("service removed, but SSH alias cleanup failed: %w", sshErr)
	}
	fmt.Fprintf(output, "Stopped and uninstalled lan-nick from %s.\\n", service.ManagerName())
	return nil
}

func serve(arguments []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	interval := flags.Duration("interval", agent.DefaultInterval, "announcement interval")
	ttl := flags.Duration("ttl", agent.DefaultTTL, "time before a silent peer expires")
	port := flags.Int("port", agent.DefaultPort, "UDP discovery port")
	hostsPath := flags.String("hosts-file", "", "hosts file path (defaults to the OS hosts file)")
	noHosts := flags.Bool("no-hosts", false, "discover peers without changing host or SSH files")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: lan-nick serve [options]")
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Advertising %q as %s every %s on UDP %d\n", cfg.Nickname, config.Alias(cfg.Nickname), *interval, *port)
	if *noHosts {
		fmt.Fprintln(stdout, "Host and SSH configuration synchronization is disabled.")
	} else {
		fmt.Fprintln(stdout, "Host and SSH configuration synchronization is enabled; administrator privileges are normally required.")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return agent.Run(ctx, agent.Options{
		Port:      *port,
		Interval:  *interval,
		TTL:       *ttl,
		HostsPath: *hostsPath,
		SyncHosts: !*noHosts,
		Log:       stderr,
	})
}

func printHelp(output io.Writer) {
	fmt.Fprint(output, `lan-nick discovers machine nicknames on the local IPv4 LAN.

Usage:
  lan-nick                         Show this machine's nickname and local IPs
  lan-nick rename "Living Room"   Change the nickname (alias: living-room)
  lan-nick group "Upstairs"       Assign this machine to a display group
  lan-nick map                     Show active nicknames, aliases, and IPs
  lan-nick install                 Install and start the OS background service
  lan-nick uninstall               Stop and remove the OS background service
  lan-nick serve [options]         Run the agent in the foreground

Install once from an elevated shell so the service can maintain the OS hosts
file. On macOS and Linux, run 'sudo lan-nick install'. On Windows, run
'lan-nick install' in an Administrator terminal. The installer copies its
current executable to a stable system location and starts it at boot.

Discovery uses multicast and directed IPv4 broadcast and never crosses routers.
Naked names such as 'ssh root@living-room' work through managed OS host and
SSH configuration. Advertised SSH host keys are trusted automatically. Use
'lan-nick serve --no-hosts' for discovery without changing system files.

Host aliases contain lowercase ASCII letters, digits, and hyphens. Spaces and
punctuation in display nicknames become hyphens. If two machines advertise the
same alias, lan-nick reports the collision and installs neither mapping.
`)
}
