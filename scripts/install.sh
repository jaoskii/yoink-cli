#!/usr/bin/env sh
# Install yoink from this checkout (macOS / Linux):
# checks Go, downloads dependencies, builds, installs, and offers to fix PATH.
set -eu

cd "$(dirname "$0")/.."

go_help() {
	echo "Install Go 1.21 or newer, then re-run this script:"
	case "$(uname -s)" in
	Darwin) echo "  brew install go            (or download from https://go.dev/dl/)" ;;
	Linux)
		echo "  sudo apt install golang-go # Debian/Ubuntu (check the version is 1.21+)"
		echo "  sudo dnf install golang    # Fedora"
		echo "  or download from https://go.dev/dl/"
		;;
	esac
	exit 1
}

if ! command -v go >/dev/null 2>&1; then
	echo "Go is not installed."
	go_help
fi

# go.mod asks for a newer toolchain; Go 1.21+ downloads it automatically.
ver=$(go env GOVERSION | sed 's/^go//')
major=${ver%%.*}
rest=${ver#*.}
minor=${rest%%.*}
if [ "$major" -lt 1 ] || { [ "$major" -eq 1 ] && [ "$minor" -lt 21 ]; }; then
	echo "Go $ver is too old."
	go_help
fi

echo "→ Downloading dependencies"
go mod download

echo "→ Building and installing yoink"
version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
CGO_ENABLED=0 go install -trimpath -ldflags "-s -w -X main.version=$version" ./cmd/yoink

bindir=$(go env GOBIN)
[ -n "$bindir" ] || bindir="$(go env GOPATH)/bin"
echo "✓ Installed $bindir/yoink ($version)"

case ":$PATH:" in
*":$bindir:"*)
	echo
	echo "Run: yoink --help"
	exit 0
	;;
esac

case "${SHELL:-}" in
*/zsh) rc="$HOME/.zshrc" ;;
*/bash) if [ "$(uname -s)" = Darwin ]; then rc="$HOME/.bash_profile"; else rc="$HOME/.bashrc"; fi ;;
*) rc="$HOME/.profile" ;;
esac
line="export PATH=\"$bindir:\$PATH\""

echo
echo "$bindir is not on your PATH yet."
answer=n
if [ -t 0 ]; then
	printf "Add it to %s now? [y/N] " "$rc"
	read -r answer || answer=n
fi
case "$answer" in
[yY]*)
	printf '\n# yoink (Go binaries)\n%s\n' "$line" >>"$rc"
	echo "✓ Added. Open a new terminal (or run: . $rc), then: yoink --help"
	;;
*)
	echo "Add this line to $rc, then open a new terminal:"
	echo "  $line"
	echo "Or run it directly: $bindir/yoink --help"
	;;
esac
