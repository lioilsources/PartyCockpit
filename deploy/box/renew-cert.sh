#!/usr/bin/env bash
# Let's Encrypt certificate for the box's own domain over DNS-01 (Cloudflare):
# no open port needed, works while the box is at home. The certificate is valid
# 90 days, so a box renewed at home carries a valid one to a party without
# internet. Run as root; the timer runs it weekly.
#   /opt/partycockpit/lego.env (mode 600):
#     CLOUDFLARE_DNS_API_TOKEN=…   (DNS edit for the one zone)
#     LEGO_EMAIL=…
#     LEGO_DOMAIN=box.example.com
set -euo pipefail

ROOT=/opt/partycockpit
[ -f "$ROOT/lego.env" ] || { echo "missing $ROOT/lego.env" >&2; exit 1; }
set -a
# shellcheck source=/dev/null
. "$ROOT/lego.env"
set +a
: "${LEGO_EMAIL:?}" "${LEGO_DOMAIN:?}" "${CLOUDFLARE_DNS_API_TOKEN:?}"

LEGO_PATH="$ROOT/lego"
args=(--accept-tos --email "$LEGO_EMAIL" --dns cloudflare --domains "$LEGO_DOMAIN" --path "$LEGO_PATH")
crt="$LEGO_PATH/certificates/$LEGO_DOMAIN.crt"
before="$(sha256sum "$crt" 2>/dev/null || true)"
if [ -f "$crt" ]; then
  lego "${args[@]}" renew --days 30
else
  lego "${args[@]}" run
fi
after="$(sha256sum "$crt")"

install -o party -g party -m 0644 "$crt" "$ROOT/certs/$LEGO_DOMAIN.crt"
install -o party -g party -m 0600 "$LEGO_PATH/certificates/$LEGO_DOMAIN.key" "$ROOT/certs/$LEGO_DOMAIN.key"
if [ "$before" != "$after" ]; then
  systemctl restart partycockpit.service
  echo "new certificate installed, backend restarted"
else
  echo "certificate still valid, nothing to do"
fi
