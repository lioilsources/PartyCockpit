#!/usr/bin/env bash
# Kiosk: waits for the backend, then Chromium full screen under cage (a
# single-app Wayland compositor) on /box, which redirects to the party's stage.
set -euo pipefail

# shellcheck source=/dev/null
[ -f /opt/partycockpit/kiosk.env ] && . /opt/partycockpit/kiosk.env
CHROMIUM="${CHROMIUM:-$(command -v chromium-browser || command -v chromium)}"
URL="${KIOSK_URL:-http://127.0.0.1:8098/box}"

for _ in $(seq 1 60); do
  curl -fs -o /dev/null http://127.0.0.1:8098/healthz && break
  sleep 1
done

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"
exec cage -d -- "$CHROMIUM" \
  --kiosk --noerrdialogs --disable-infobars --no-first-run \
  --ozone-platform=wayland --enable-features=OverlayScrollbar \
  --disable-features=WebRtcHideLocalIpsWithMdns,Translate \
  --use-fake-ui-for-media-stream --autoplay-policy=no-user-gesture-required \
  --check-for-update-interval=31536000 --disk-cache-dir=/opt/partycockpit/.cache \
  --user-data-dir=/opt/partycockpit/.cache/chromium \
  "$URL"
