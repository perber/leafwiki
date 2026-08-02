#!/usr/bin/env bash
# create-users.sh — one-time setup for the load test.
#
# Creates N editor accounts against a running (throwaway!) LeafWiki instance,
# logs each one in, and writes their {cookie, csrfToken} pairs to a session
# pool file that the k6 scripts read via lib/auth.js. This is the only script
# that needs admin credentials; the k6 runs themselves only ever use the
# already-authenticated session pool.
#
# Usage:
#   BASE_URL=http://127.0.0.1:8091 \
#   ADMIN_IDENTIFIER=admin \
#   ADMIN_PASSWORD='...' \
#   USER_COUNT=100 \
#   ./create-users.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8091}"
ADMIN_IDENTIFIER="${ADMIN_IDENTIFIER:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:?ADMIN_PASSWORD is required}"
USER_COUNT="${USER_COUNT:-100}"
USER_PASSWORD="${USER_PASSWORD:-LoadTest-Editor-Pw-1!}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT_FILE="${OUT_FILE:-$SCRIPT_DIR/.session-pool.json}"

command -v curl >/dev/null || { echo "create-users.sh: curl is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "create-users.sh: python3 is required (JSON parsing)" >&2; exit 1; }

admin_jar="$(mktemp)"
trap 'rm -f "$admin_jar"' EXIT

echo "create-users.sh: logging in as admin ($ADMIN_IDENTIFIER)..."
login_status=$(curl -s -o /dev/null -w "%{http_code}" -c "$admin_jar" \
  -H "Content-Type: application/json" \
  -d "{\"identifier\":\"$ADMIN_IDENTIFIER\",\"password\":\"$ADMIN_PASSWORD\"}" \
  "$BASE_URL/api/auth/login")
if [ "$login_status" != "200" ]; then
  echo "create-users.sh: admin login failed (HTTP $login_status)" >&2
  exit 1
fi
admin_csrf=$(python3 -c "
import re
with open('$admin_jar') as f:
    for line in f:
        if 'leafwiki_csrf' in line:
            print(line.strip().split('\t')[-1])
            break
")

echo "create-users.sh: creating $USER_COUNT editor accounts and logging each in..."
echo "[" > "$OUT_FILE.tmp"
first=1
for i in $(seq 1 "$USER_COUNT"); do
  uname=$(printf "loadtest-editor-%03d" "$i")
  email="${uname}@loadtest.local"

  create_status=$(curl -s -o /tmp/create-user-resp.json -w "%{http_code}" \
    -b "$admin_jar" \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: $admin_csrf" \
    -d "{\"username\":\"$uname\",\"email\":\"$email\",\"password\":\"$USER_PASSWORD\",\"role\":\"editor\"}" \
    "$BASE_URL/api/users")
  if [ "$create_status" != "201" ] && [ "$create_status" != "200" ] && [ "$create_status" != "409" ]; then
    echo "create-users.sh: WARN failed to create $uname (HTTP $create_status), skipping" >&2
    cat /tmp/create-user-resp.json >&2
    continue
  fi
  # 409 (already exists, e.g. a re-run) is fine — the account exists with
  # USER_PASSWORD from a prior run, so login below still works.

  user_jar="$(mktemp)"
  login2_status=$(curl -s -o /dev/null -w "%{http_code}" -c "$user_jar" \
    -H "Content-Type: application/json" \
    -d "{\"identifier\":\"$uname\",\"password\":\"$USER_PASSWORD\"}" \
    "$BASE_URL/api/auth/login")
  if [ "$login2_status" != "200" ]; then
    echo "create-users.sh: WARN failed to log in $uname (HTTP $login2_status), skipping" >&2
    rm -f "$user_jar"
    continue
  fi

  entry=$(python3 -c "
import json, sys

def read_cookie(path, name):
    with open(path) as f:
        for line in f:
            parts = line.strip().split('\t')
            if len(parts) >= 7 and parts[5] == name:
                return parts[6]
    return None

at = read_cookie('$user_jar', 'leafwiki_at')
rt = read_cookie('$user_jar', 'leafwiki_rt')
csrf = read_cookie('$user_jar', 'leafwiki_csrf')
cookie_header = f'leafwiki_at={at}; leafwiki_rt={rt}; leafwiki_csrf={csrf}'
print(json.dumps({'username': '$uname', 'cookie': cookie_header, 'csrfToken': csrf}))
")
  rm -f "$user_jar"

  if [ "$first" -eq 1 ]; then first=0; else echo "," >> "$OUT_FILE.tmp"; fi
  printf '%s' "$entry" >> "$OUT_FILE.tmp"

  if [ $((i % 20)) -eq 0 ]; then
    echo "create-users.sh: $i/$USER_COUNT done"
  fi
done
echo "]" >> "$OUT_FILE.tmp"
mv "$OUT_FILE.tmp" "$OUT_FILE"

pool_size=$(python3 -c "import json; print(len(json.load(open('$OUT_FILE'))))")
echo "create-users.sh: done, $pool_size sessions written to $OUT_FILE"
