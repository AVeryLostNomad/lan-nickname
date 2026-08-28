#!/bin/sh
set -eu

case "$(uname -s)" in
	MINGW*|MSYS*|CYGWIN*|Windows_NT)
		;;
	*)
		printf '%s\n' "lan-nick: win-install.sh must be run from a Windows shell such as Git Bash" >&2
		exit 1
		;;
esac

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"

go_bin=${GO_BIN:-}
if [ -z "$go_bin" ]; then
	go_bin=$(command -v go || true)
fi
if [ -z "$go_bin" ] || [ ! -x "$go_bin" ]; then
	printf '%s\n' "lan-nick: Go was not found; run 'GO_BIN=/path/to/go ./win-install.sh'" >&2
	exit 1
fi

powershell_bin=${POWERSHELL_BIN:-}
if [ -z "$powershell_bin" ]; then
	powershell_bin=$(command -v powershell.exe || command -v pwsh.exe || true)
fi
if [ -z "$powershell_bin" ] || [ ! -x "$powershell_bin" ]; then
	printf '%s\n' "lan-nick: PowerShell was not found; run 'POWERSHELL_BIN=/path/to/powershell.exe ./win-install.sh'" >&2
	exit 1
fi

printf '%s\n' "Building lan-nick for Windows..."
"$go_bin" build -o "$script_dir/lan-nick.exe" ./cmd/lan-nick

printf '%s\n' "Installing lan-nick into your Go binary directory..."
"$go_bin" install ./cmd/lan-nick

is_admin=$("$powershell_bin" -NoProfile -NonInteractive -Command '([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)')
case "$is_admin" in
	True*)
		printf '%s\n' "Installing and starting the privileged lan-nick service..."
		"$script_dir/lan-nick.exe" install
		;;
	*)
		if ! command -v cygpath >/dev/null 2>&1; then
			printf '%s\n' "lan-nick: cygpath is required to request Administrator access" >&2
			exit 1
		fi
		windows_exe=$(cygpath -aw "$script_dir/lan-nick.exe")
		printf '%s\n' "Requesting Administrator access to install and start the lan-nick service..."
		LAN_NICK_INSTALL_EXE=$windows_exe \
			"$powershell_bin" -NoProfile -NonInteractive -Command '$process = Start-Process -FilePath $env:LAN_NICK_INSTALL_EXE -ArgumentList "install" -Verb RunAs -Wait -PassThru; exit $process.ExitCode'
		;;
esac
