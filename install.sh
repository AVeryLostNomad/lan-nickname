#!/bin/sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
	printf '%s\n' "lan-nick: run this script with sudo: sudo ./install.sh" >&2
	exit 1
fi

invoking_user=${SUDO_USER:-}
if [ -z "$invoking_user" ] || [ "$invoking_user" = root ]; then
	printf '%s\n' "lan-nick: cannot determine the user who invoked sudo" >&2
	exit 1
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"

go_bin=${GO_BIN:-}
if [ -z "$go_bin" ]; then
	go_bin=$(command -v go || true)
fi
if [ -z "$go_bin" ]; then
	for candidate in /usr/local/go/bin/go /opt/homebrew/bin/go /usr/local/bin/go /usr/bin/go; do
		if [ -x "$candidate" ]; then
			go_bin=$candidate
			break
		fi
	done
fi
if [ -z "$go_bin" ] || [ ! -x "$go_bin" ]; then
	printf '%s\n' "lan-nick: Go was not found; run 'sudo env GO_BIN=/path/to/go ./install.sh'" >&2
	exit 1
fi

sudo_bin=$(command -v sudo || true)
if [ -z "$sudo_bin" ]; then
	printf '%s\n' "lan-nick: sudo is required to build as $invoking_user" >&2
	exit 1
fi

run_as_invoking_user() {
	"$sudo_bin" -H -u "$invoking_user" -- "$@"
}

printf 'Building lan-nick for %s as %s...\n' "$(uname -s)/$(uname -m)" "$invoking_user"
run_as_invoking_user "$go_bin" build -o "$script_dir/lan-nick" ./cmd/lan-nick

printf 'Installing lan-nick into the Go binary directory for %s...\n' "$invoking_user"
run_as_invoking_user "$go_bin" install ./cmd/lan-nick

printf '%s\n' "Installing and starting the privileged lan-nick service..."
"$script_dir/lan-nick" install
