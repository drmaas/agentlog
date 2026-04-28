#!/bin/sh

set -eu

REPO="${AGENTLOG_REPO:-drmaas/agentlog}"
VERSION="${AGENTLOG_VERSION:-latest}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"
RELEASE_BASE_URL="${AGENTLOG_RELEASE_BASE_URL:-}"

has_cmd() {
	command -v "$1" >/dev/null 2>&1
}

say() {
	printf '%s\n' "$*"
}

fail() {
	say "agentlog installer: $*" >&2
	exit 1
}

detect_os() {
	case "$(uname -s)" in
		Linux) printf 'linux' ;;
		Darwin) printf 'darwin' ;;
		MINGW*|MSYS*|CYGWIN*) printf 'windows' ;;
		*) fail "unsupported OS: $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
		x86_64|amd64) printf 'amd64' ;;
		arm64|aarch64) printf 'arm64' ;;
		*) fail "unsupported architecture: $(uname -m)" ;;
	esac
}

download_to() {
	url="$1"
	out="$2"
	if has_cmd curl; then
		curl -fsSL "$url" -o "$out"
		return 0
	fi
	if has_cmd wget; then
		wget -qO "$out" "$url"
		return 0
	fi
	fail "curl or wget is required"
}

install_from_archive() {
	url="$1"
	workdir="$2"
	archive="$workdir/agentlog.tgz"

	say "Downloading $url"
	download_to "$url" "$archive" || return 1
	test -s "$archive" || return 1
	tar -xzf "$archive" -C "$workdir" || return 1
	test -f "$workdir/agentlog" || return 1
	install -m 0755 "$workdir/agentlog" "$BIN_DIR/agentlog"
}

install_from_go() {
	has_cmd go || fail "no release artifact found and Go is not installed"
	say "Falling back to go install"
	go_version="$VERSION"
	if [ "$go_version" = "latest" ]; then
		go_version="latest"
	fi
	GOBIN="$BIN_DIR" go install "github.com/drmaas/agentlog/cmd/agentlog@$go_version"
}

main() {
	os="$(detect_os)"
	
	# Redirect Windows users to PowerShell installer
	if [ "$os" = "windows" ]; then
		fail "Please use the PowerShell installer for Windows:
		powershell -ExecutionPolicy Bypass -Command \"& { \$(irm https://raw.githubusercontent.com/drmaas/agentlog/main/scripts/install.ps1) }\""
	fi
	
	arch="$(detect_arch)"
	asset="agentlog_${os}_${arch}.tar.gz"

	if [ -n "$RELEASE_BASE_URL" ]; then
		base_url="$RELEASE_BASE_URL"
	elif [ "$VERSION" = "latest" ]; then
		base_url="https://github.com/$REPO/releases/latest/download"
	else
		base_url="https://github.com/$REPO/releases/download/$VERSION"
	fi

	mkdir -p "$BIN_DIR"
	workdir="$(mktemp -d)"
	trap 'rm -rf "$workdir"' EXIT INT TERM HUP

	if install_from_archive "$base_url/$asset" "$workdir"; then
		say "Installed agentlog to $BIN_DIR/agentlog"
	else
		install_from_go
		say "Installed agentlog to $BIN_DIR/agentlog"
	fi

	case ":$PATH:" in
		*":$BIN_DIR:"*) ;;
		*) say "Note: add $BIN_DIR to PATH to run agentlog directly." ;;
	esac
}

main "$@"
