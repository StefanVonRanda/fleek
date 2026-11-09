#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")"

BINDIR="${BINDIR:-${PREFIX:-$HOME/.local}/bin}"
CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
CONFIG_FILE="$CONFIG_HOME/fleek/config.toml"

mkdir -p "$BINDIR"
tmp="$BINDIR/.fleek.$$"
trap 'rm -f "$tmp"' EXIT INT TERM
go build -trimpath -ldflags="-s -w" -o "$tmp" .
mv "$tmp" "$BINDIR/fleek"
chmod 0755 "$BINDIR/fleek"
trap - EXIT INT TERM

if [ ! -e "$CONFIG_FILE" ]; then
  mkdir -p "$(dirname "$CONFIG_FILE")"
  cp examples/config.toml "$CONFIG_FILE"
  echo "Installed a sample config at $CONFIG_FILE"
fi

echo "Installed $BINDIR/fleek"
case ":$PATH:" in
  *":$BINDIR:"*) ;;
  *) echo "Add $BINDIR to PATH to run fleek from anywhere." ;;
esac
echo "Run: fleek"
