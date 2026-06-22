#!/usr/bin/env bash
# Build botany as a static binary and deploy it to a tilde-style SSH host.
#
# Usage:  ./deploy.sh [user@host]      (default host: odonian.net)
#
# Installs the self-contained binary to /opt/botany/botany, symlinks it onto
# PATH at /usr/local/bin/botany, and ensures the shared community-garden dir
# exists. Re-running upgrades the binary in place; the garden and players'
# ~/.botany are left untouched. Does not modify /etc/motd.
set -euo pipefail

HOST="${1:-odonian.net}"
PREFIX="/opt/botany"

[ -f go.mod ] || { echo "error: run from the repo root (no go.mod here)"; exit 1; }

tmp="$(mktemp -t botany.XXXXXX)"
trap 'rm -f "$tmp"' EXIT

echo "==> building static linux/amd64 binary"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$tmp" .

echo "==> uploading to $HOST"
scp -q "$tmp" "$HOST:/tmp/botany.upload"

echo "==> installing on $HOST (sudo)"
ssh "$HOST" "sudo bash -s -- '$PREFIX'" <<'REMOTE'
set -euo pipefail
PREFIX="${1:-/opt/botany}"
install -D -m0755 -o root -g root /tmp/botany.upload "$PREFIX/botany"
rm -f /tmp/botany.upload
# Shared community garden: create once, then leave perms/data intact on upgrade.
mkdir -p "$PREFIX/sqlite"
if [ ! -e "$PREFIX/sqlite/garden_db.sqlite" ]; then
    : > "$PREFIX/sqlite/garden_db.sqlite"
    chmod 0666 "$PREFIX/sqlite/garden_db.sqlite"
fi
if [ ! -e "$PREFIX/garden_file.json" ]; then
    printf '{}' > "$PREFIX/garden_file.json"
    chmod 0666 "$PREFIX/garden_file.json"
fi
chmod 0777 "$PREFIX/sqlite"
ln -sf "$PREFIX/botany" /usr/local/bin/botany
echo "installed:"
ls -la "$PREFIX/botany" /usr/local/bin/botany
REMOTE

echo "==> done: users on $HOST can now run 'botany'"
