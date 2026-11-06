#!/usr/bin/env bash
# PartyCockpit party box installer (Raspberry Pi OS Bookworm Lite 64-bit, or
# Debian 12 on x86). Run from the unpacked release as root:
#   sudo ./deploy/box/install.sh
# Idempotent: re-running updates the binary and units, keeps .env and data.
set -euo pipefail

ROOT=/opt/partycockpit
USER_NAME=party
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PKG="$(cd "$HERE/../.." && pwd)"

[ "$(id -u)" -eq 0 ] || { echo "run as root (sudo)" >&2; exit 1; }
[ -x "$PKG/bin/partycockpit" ] || { echo "bin/partycockpit missing: run from the unpacked release" >&2; exit 1; }

echo "==> packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
# Raspberry Pi OS ships Chromium as chromium-browser, Debian as chromium
if apt-cache show chromium-browser >/dev/null 2>&1; then CHROMIUM=chromium-browser; else CHROMIUM=chromium; fi
apt-get install -y -qq --no-install-recommends cage "$CHROMIUM" curl ca-certificates lego fonts-noto-color-emoji >/dev/null

echo "==> user and folders"
if ! id "$USER_NAME" >/dev/null 2>&1; then
  useradd --system --home-dir "$ROOT" --shell /usr/sbin/nologin "$USER_NAME"
fi
# the kiosk draws on the GPU and needs the seat's input devices
for g in video render input audio; do
  getent group "$g" >/dev/null && usermod -aG "$g" "$USER_NAME"
done
install -d -o "$USER_NAME" -g "$USER_NAME" -m 0750 "$ROOT" "$ROOT/data" "$ROOT/certs" "$ROOT/.cache"
install -d -m 0755 "$ROOT/bin" "$ROOT/assets"

echo "==> binary and scripts"
install -m 0755 "$PKG/bin/partycockpit" "$ROOT/bin/partycockpit.new"
mv -f "$ROOT/bin/partycockpit.new" "$ROOT/bin/partycockpit"
for s in kiosk.sh renew-cert.sh update.sh; do
  install -m 0755 "$HERE/$s" "$ROOT/bin/$s"
done
rm -rf "$ROOT/assets/drivers"
cp -R "$PKG/assets/drivers" "$ROOT/assets/drivers"
[ -f "$PKG/VERSION" ] && install -m 0644 "$PKG/VERSION" "$ROOT/VERSION"
printf 'CHROMIUM=%s\n' "$(command -v "$CHROMIUM")" > "$ROOT/kiosk.env"

echo "==> configuration"
if [ ! -f "$ROOT/.env" ]; then
  install -o "$USER_NAME" -g "$USER_NAME" -m 0600 "$HERE/box.env" "$ROOT/.env"
  install -m 0644 "$PKG/.env.example" "$ROOT/.env.example"
  echo "    created $ROOT/.env — edit it (tokens, PC_PUBLIC_URL), then restart"
else
  echo "    keeping $ROOT/.env"
fi

echo "==> systemd"
install -m 0644 "$HERE/partycockpit.service" "$HERE/kiosk.service" \
  "$HERE/renew-cert.service" "$HERE/renew-cert.timer" /etc/systemd/system/
# the kiosk takes over tty1
systemctl disable --now getty@tty1.service >/dev/null 2>&1 || true
systemctl daemon-reload
systemctl enable --now partycockpit.service
systemctl enable kiosk.service
systemctl restart kiosk.service || true
if [ -f "$ROOT/lego.env" ]; then
  systemctl enable --now renew-cert.timer
else
  echo "    no $ROOT/lego.env yet: certificate renewal stays off (see deploy/box/README.md)"
fi

echo "==> done: $(cat "$ROOT/VERSION" 2>/dev/null || echo unknown version)"
echo "    logs: journalctl -u partycockpit -u kiosk -f"
