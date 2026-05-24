#!/usr/bin/env bash
# Usage:
#
#   ./run.sh with-logout      Start LeafWiki behind oauth2-proxy + Dex WITH a logout URL configured.
#                             Clicking logout in LeafWiki redirects to oauth2-proxy's sign-out endpoint,
#                             which clears the proxy session before sending you back to the Dex login page.
#
#   ./run.sh without-logout   Start the same stack WITHOUT a logout URL.
#                             Clicking logout in LeafWiki only clears the internal JWT — oauth2-proxy
#                             still has a valid session and will silently re-authenticate you on the
#                             next request, so there is no way to actually log out.
#
#   ./run.sh down             Stop and remove containers from whichever variant is running.
#
# Prerequisites:
#   - Docker with the Compose plugin (docker compose)
#   - Ports 4180 (oauth2-proxy) and 5556 (Dex) must be free
#
# Login credentials (configured in dex/config.yaml):
#   Email:    admin@leafwiki.local
#   Password: password
#
# The "admin" username is forwarded to LeafWiki via the X-Forwarded-User header.
# LeafWiki resolves it against its own user database — the built-in admin account
# (created by --admin-password=admin) matches, so login works out of the box.
#
# Access:
#   http://localhost:4180   ← entry point (oauth2-proxy handles auth, proxies to LeafWiki)
#   http://localhost:5556   ← Dex OIDC provider (login UI lives here during the OAuth2 flow)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

usage() {
  grep '^#' "$0" | sed 's/^# \{0,1\}//'
  exit 1
}

stop_all() {
  docker compose -f "$SCRIPT_DIR/with-logout.yml" down --remove-orphans 2>/dev/null || true
  docker compose -f "$SCRIPT_DIR/without-logout.yml" down --remove-orphans 2>/dev/null || true
  # Force-remove any leftover containers in case compose down missed them (e.g. after Ctrl+C)
  local leftover
  leftover=$(docker ps -aq --filter "name=proxy-auth-" 2>/dev/null)
  if [ -n "$leftover" ]; then
    docker rm -f $leftover 2>/dev/null || true
  fi
}

case "${1:-}" in
  with-logout)
    echo "Stopping any running proxy-auth stacks..."
    stop_all
    echo "Starting proxy-auth stack WITH logout URL..."
    docker compose -f "$SCRIPT_DIR/with-logout.yml" up --build
    ;;
  without-logout)
    echo "Stopping any running proxy-auth stacks..."
    stop_all
    echo "Starting proxy-auth stack WITHOUT logout URL..."
    docker compose -f "$SCRIPT_DIR/without-logout.yml" up --build
    ;;
  down)
    stop_all
    ;;
  *)
    usage
    ;;
esac
