# lan-nick

`lan-nick` discovers machines on the local network, associates their current LAN IP addresses with human-friendly nicknames, and maintains those nicknames in the operating system's hosts file.

After installing it on two machines, a machine named `Dafni` can be reached from the other machine with commands such as:

```bash
ssh root@dafni
```

Nicknames may contain spaces and punctuation. `lan-nick rename "Dafni Living Room"` creates the resolvable host alias `dafni-living-room`.

## Table of contents

| Get running | Operate and maintain |
|---|---|
| [Requirements](#requirements)<br>[Build from source](#build-from-source)<br>[Configure the nickname](#configure-the-nickname) | [Verification](#verification)<br>[Everyday commands](#everyday-commands)<br>[Security model](#security-model) |
| **Install by platform**<br>[macOS — launchd](#macos-launchd)<br>[Linux — systemd](#linux-systemd)<br>[Windows Service Manager](#windows-windows-service-manager) | **Verify step by step**<br>[Background service](#1-verify-the-background-service)<br>[Peer discovery](#2-verify-peer-discovery)<br>[Hostname resolution](#3-verify-hostname-resolution)<br>[Troubleshooting](#troubleshooting-verification-failures) |
| **Remove by platform**<br>[macOS](#uninstall-macos)<br>[Linux](#uninstall-linux)<br>[Windows](#uninstall-windows) | **Quick path**<br>Build → Rename → Install → Verify |

## Requirements

- Go 1.24 or newer when building from source.
- Administrator access to install the background service and update the hosts file.
- Machines connected to the same IPv4 LAN.
- Inbound UDP port `47777` permitted on trusted LAN interfaces.

Discovery sends each announcement to multicast address `239.255.77.77` and to
the interface's directed broadcast address. Both stay on the local subnet and
do not cross routers. The broadcast path preserves discovery on Wi-Fi networks
that suppress multicast between clients.

## Build from source

Clone the repository, enter its directory, and build the CLI.

### macOS and Linux

```bash
go build -o lan-nick ./cmd/lan-nick
```

Optionally install the CLI into your Go binary directory:

```bash
go install ./cmd/lan-nick
LAN_NICK_BIN="$(go env GOPATH)/bin/lan-nick"
```

### Windows

From PowerShell:

```powershell
go build -o lan-nick.exe ./cmd/lan-nick
```

## Configure the nickname

Set the nickname before installing the service so the service and ordinary CLI commands use the same state directory.

### macOS and Linux

Using the repository build:

```bash
./lan-nick rename "Dafni"
./lan-nick
```

Using `go install`:

```bash
"$(go env GOPATH)/bin/lan-nick" rename "Dafni"
"$(go env GOPATH)/bin/lan-nick"
```

### Windows

```powershell
.\lan-nick.exe rename "Dafni"
.\lan-nick.exe
```

Status output resembles:

```text
Nickname: Dafni
Host alias: dafni
Local IPs:
  192.168.1.20
```

## Install the background service

The installer copies the current executable to a stable system location, registers it with the native service manager, starts it immediately, and configures it to start at boot.

Re-running `install` from a newer binary upgrades and restarts an existing service.

### macOS: launchd

From the repository build:

```bash
sudo ./lan-nick install
```

When using `go install`, pass the absolute binary path because `sudo` may not include the Go binary directory in its `PATH`:

```bash
LAN_NICK_BIN="$(go env GOPATH)/bin/lan-nick"
sudo "$LAN_NICK_BIN" install
```

The installer creates:

- Service executable: `/Library/PrivilegedHelperTools/lan-nick`
- LaunchDaemon: `/Library/LaunchDaemons/dev.lan-nick.agent.plist`
- User state: `~/Library/Application Support/lan-nick`
- SSH client aliases: `/etc/ssh/ssh_config.d/90-lan-nick.conf`
- Automatically trusted SSH keys: `/etc/ssh/ssh_known_hosts`

If the macOS application firewall blocks discovery, allow incoming connections for:

```text
/Library/PrivilegedHelperTools/lan-nick
```

The installer does not modify macOS firewall policy.

### Linux: systemd

The automatic Linux installer currently requires systemd.

From the repository build:

```bash
sudo ./lan-nick install
```

When using `go install`:

```bash
LAN_NICK_BIN="$(go env GOPATH)/bin/lan-nick"
sudo "$LAN_NICK_BIN" install
```

The installer creates:

- Service executable: `/usr/local/libexec/lan-nick`
- systemd unit: `/etc/systemd/system/lan-nick.service`
- User state: `~/.config/lan-nick`
- SSH client aliases: `/etc/ssh/ssh_config.d/90-lan-nick.conf`
- Automatically trusted SSH keys: `/etc/ssh/ssh_known_hosts`

If a host firewall is enabled, permit inbound UDP port `47777` on trusted LAN interfaces. Firewall management differs between Linux distributions, so the installer does not modify Linux firewall rules.

Non-systemd distributions can run the foreground agent instead:

```bash
sudo ./lan-nick serve
```

### Windows: Windows Service Manager

Open PowerShell **as Administrator**, return to the directory containing `lan-nick.exe`, and run:

```powershell
.\lan-nick.exe install
```

The installer creates:

- Windows service name: `LanNick`
- Service executable: `%ProgramFiles%\lan-nick\lan-nick.exe`
- An Application event-log source named `LanNick`
- A private-network inbound Windows Firewall rule for UDP port `47777`, restricted to the installed executable
- SSH client aliases: `%ProgramData%\ssh\ssh_config`
- Automatically trusted SSH keys: `%ProgramData%\ssh\ssh_known_hosts`

Ensure Windows classifies the connected LAN as **Private**. The installed firewall rule intentionally does not enable discovery on Public networks.

## Verification

Complete these checks after installing `lan-nick` on at least two machines on the same LAN.

### 1. Verify the background service

#### macOS

```bash
sudo launchctl print system/dev.lan-nick.agent
```

The output should identify `/Library/PrivilegedHelperTools/lan-nick` as the program and show a running process.

#### Linux

```bash
systemctl status lan-nick.service
```

The unit should report `active (running)`. Follow service logs with:

```bash
journalctl -u lan-nick.service -f
```

#### Windows

```powershell
Get-Service LanNick
```

The service status should be `Running`. Read recent service events with:

```powershell
Get-WinEvent -FilterHashtable @{
    LogName = 'Application'
    ProviderName = 'LanNick'
} -MaxEvents 20
```

### 2. Verify peer discovery

Give one participating machine a distinct nickname:

```bash
lan-nick rename "Dafni"
```

If using a repository build rather than a CLI installed in `PATH`, substitute `./lan-nick` or `.\lan-nick.exe`.

Wait approximately five seconds, then run this on another participating machine:

```bash
lan-nick map
```

Expected output resembles:

```text
dafni	192.168.1.20	Dafni
```

### 3. Verify hostname resolution

From the second machine:

```bash
ping dafni
ssh root@dafni
```

The operating system should resolve `dafni` to the IP shown by `lan-nick map`. If the target advertises a readable OpenSSH host public key, lan-nick configures the system SSH client to use IPv4 and automatically trusts that key under the machine's stable lan-nick ID. The target machine must separately have the requested service, such as SSH, enabled.

### Troubleshooting verification failures

If `lan-nick map` works but `ping dafni` does not:

1. Confirm the background service is running with administrator privileges.
2. Inspect the managed section in the OS hosts file.
3. Confirm the alias is not marked as a collision in `lan-nick map`.

If `lan-nick map` does not discover the other machine:

1. Confirm both machines are on the same IPv4 LAN.
2. Confirm both services are running.
3. Permit inbound UDP port `47777` on trusted interfaces.
4. On Windows, confirm the network profile is Private.
5. Confirm the network does not isolate wireless clients or suppress both multicast and broadcast traffic.

## Everyday commands

```bash
lan-nick                         # Show this machine's nickname and IP addresses
lan-nick rename "Living Room"   # Change the nickname and alias
lan-nick map                     # Show active peers
lan-nick serve                   # Run the agent in the foreground
lan-nick serve --no-hosts        # Discover peers without changing host or SSH files
```

## Uninstall

Uninstallation stops the service, removes its installed system binary and service definition, and clears the managed hosts-file, SSH-client, and SSH-known-hosts mappings. It preserves the user's nickname and cached state directory.

<a id="uninstall-macos"></a>

### macOS

```bash
sudo ./lan-nick uninstall
```

Or, when using `go install`:

```bash
sudo "$(go env GOPATH)/bin/lan-nick" uninstall
```

<a id="uninstall-linux"></a>

### Linux

```bash
sudo ./lan-nick uninstall
```

Or, when using `go install`:

```bash
sudo "$(go env GOPATH)/bin/lan-nick" uninstall
```

<a id="uninstall-windows"></a>

### Windows

From an Administrator PowerShell:

```powershell
.\lan-nick.exe uninstall
```

## Security model

`lan-nick` accepts nickname announcements from the local LAN. It derives a peer's IP from the UDP packet source rather than trusting an advertised address, prevents nicknames from overriding fully qualified domain names, and disables aliases claimed by multiple machines. When a peer advertises an SSH host public key, lan-nick writes that key to the system known-hosts file and configures the alias to use it without an interactive trust prompt. The stable machine ID keeps that trust working when the peer's IPv4 address changes.

Announcements and advertised SSH keys are not authenticated. A device on the same LAN can claim a previously unused nickname and make its SSH key trusted for that alias. Use this automatic trust only on LANs whose participants you trust; an untrusted LAN participant can impersonate a lan-nick SSH destination.
