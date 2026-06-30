#!/bin/sh
set -eu

if [ -z "${URAPT_SERVER_URL:-}" ]; then
  echo "URAPT_SERVER_URL is required, e.g. https://urapt-production.up.railway.app" >&2
  exit 1
fi

# Caddy reverse_proxy upstreams may only contain scheme, host, and port.
# Railway/public URLs are often copied with a trailing slash; normalize that.
URAPT_UPSTREAM="$URAPT_SERVER_URL"
while [ "${URAPT_UPSTREAM%/}" != "$URAPT_UPSTREAM" ]; do
  URAPT_UPSTREAM="${URAPT_UPSTREAM%/}"
done
export URAPT_UPSTREAM

exec caddy run --config /etc/caddy/Caddyfile --adapter caddyfile
