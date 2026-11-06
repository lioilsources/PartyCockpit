#!/usr/bin/env bash
# Update the box: the latest GitHub release, a release tarball, or a bare
# binary. Data, .env and certificates stay.
#   sudo /opt/partycockpit/bin/update.sh                 # latest release
#   sudo /opt/partycockpit/bin/update.sh pkg.tar.gz      # a release tarball
#   sudo /opt/partycockpit/bin/update.sh partycockpit    # just the binary
set -euo pipefail

REPO="${REPO:-lioilsources/PartyCockpit}"
[ "$(id -u)" -eq 0 ] || { echo "run as root (sudo)" >&2; exit 1; }
case "$(uname -m)" in
  aarch64|arm64) ARCH=arm64 ;;
  x86_64|amd64) ARCH=amd64 ;;
  *) echo "unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
src="${1:-}"
if [ -z "$src" ]; then
  src="$work/pkg.tar.gz"
  curl -fL -o "$src" "https://github.com/$REPO/releases/latest/download/partycockpit-linux-$ARCH.tar.gz"
fi

case "$src" in
  *.tar.gz|*.tgz)
    tar -xzf "$src" -C "$work"
    pkg="$(find "$work" -mindepth 1 -maxdepth 1 -type d -name 'partycockpit-*' | head -n1)"
    [ -n "$pkg" ] || { echo "not a PartyCockpit release tarball" >&2; exit 1; }
    "$pkg/deploy/box/install.sh"
    ;;
  *)
    # Pi OS Lite has no `file`: check the ELF magic directly
    [ "$(head -c 4 "$src" | tail -c 3)" = "ELF" ] || { echo "$src is not a Linux binary" >&2; exit 1; }
    install -m 0755 "$src" /opt/partycockpit/bin/partycockpit.new
    mv -f /opt/partycockpit/bin/partycockpit.new /opt/partycockpit/bin/partycockpit
    systemctl restart partycockpit.service
    ;;
esac
echo "updated: $(/opt/partycockpit/bin/partycockpit -version 2>/dev/null || cat /opt/partycockpit/VERSION 2>/dev/null || echo ok)"
