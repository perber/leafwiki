#!/usr/bin/env bash
# create-writer-accounts.sh — setup for the reads-during-user-writes.js
# scenario. Creates a handful of dedicated, non-reader accounts an admin
# session can toggle (role flip) to generate a steady stream of users.db
# writes, plus saves the admin session itself for the k6 writer VUs to use.
#
# Kept separate from create-users.sh's 100-account reader pool so writer
# traffic never touches reader sessions.
#
# Usage:
#   BASE_URL=http://127.0.0.1:8091 \
#   ADMIN_IDENTIFIER=admin \
#   ADMIN_PASSWORD='...' \
#   WRITER_ACCOUNT_COUNT=5 \
#   ./create-writer-accounts.sh
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8091}"
ADMIN_IDENTIFIER="${ADMIN_IDENTIFIER:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:?ADMIN_PASSWORD is required}"
WRITER_ACCOUNT_COUNT="${WRITER_ACCOUNT_COUNT:-5}"
WRITER_ACCOUNT_PASSWORD="${WRITER_ACCOUNT_PASSWORD:-LoadTest-Writer-Pw-1!}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ADMIN_SESSION_OUT="${ADMIN_SESSION_OUT:-$SCRIPT_DIR/.admin-session.json}"
WRITER_ACCOUNTS_OUT="${WRITER_ACCOUNTS_OUT:-$SCRIPT_DIR/.writer-accounts.json}"

command -v curl >/dev/null || { echo "create-writer-accounts.sh: curl is required" >&2; exit 1; }
command -v python3 >/dev/null || { echo "create-writer-accounts.sh: python3 is required" >&2; exit 1; }

admin_jar="$(mktemp)"
trap 'rm -f "$admin_jar"' EXIT

echo "create-writer-accounts.sh: logging in as admin ($ADMIN_IDENTIFIER)..."
login_status=$(curl -s -o /dev/null -w "%{http_code}" -c "$admin_jar" \
  -H "Content-Type: application/json" \
  -d "{\"identifier\":\"$ADMIN_IDENTIFIER\",\"password\":\"$ADMIN_PASSWORD\"}" \
  "$BASE_URL/api/auth/login")
if [ "$login_status" != "200" ]; then
  echo "create-writer-accounts.sh: admin login failed (HTTP $login_status)" >&2
  exit 1
fi

# Save the admin session in the same {cookie, csrfToken} shape lib/auth.js expects.
python3 -c "
import json

def read_cookie(path, name):
    with open(path) as f:
        for line in f:
            parts = line.strip().split('\t')
            if len(parts) >= 7 and parts[5] == name:
                return parts[6]
    return None

at = read_cookie('$admin_jar', 'leafwiki_at')
rt = read_cookie('$admin_jar', 'leafwiki_rt')
csrf = read_cookie('$admin_jar', 'leafwiki_csrf')
cookie_header = f'leafwiki_at={at}; leafwiki_rt={rt}; leafwiki_csrf={csrf}'
json.dump({'username': '$ADMIN_IDENTIFIER', 'cookie': cookie_header, 'csrfToken': csrf}, open('$ADMIN_SESSION_OUT', 'w'))
"
echo "create-writer-accounts.sh: admin session written to $ADMIN_SESSION_OUT"

admin_csrf=$(python3 -c "import json; print(json.load(open('$ADMIN_SESSION_OUT'))['csrfToken'])")

echo "create-writer-accounts.sh: ensuring $WRITER_ACCOUNT_COUNT writer accounts exist..."
echo "[" > "$WRITER_ACCOUNTS_OUT.tmp"
first=1
for i in $(seq 1 "$WRITER_ACCOUNT_COUNT"); do
  uname=$(printf "loadtest-writer-%03d" "$i")
  email="${uname}@loadtest.local"

  create_resp=$(mktemp)
  create_status=$(curl -s -o "$create_resp" -w "%{http_code}" \
    -b "$admin_jar" \
    -H "Content-Type: application/json" \
    -H "X-CSRF-Token: $admin_csrf" \
    -d "{\"username\":\"$uname\",\"email\":\"$email\",\"password\":\"$WRITER_ACCOUNT_PASSWORD\",\"role\":\"editor\"}" \
    "$BASE_URL/api/users")

  if [ "$create_status" = "201" ] || [ "$create_status" = "200" ]; then
    user_id=$(python3 -c "import json; print(json.load(open('$create_resp'))['id'])")
  elif [ "$create_status" = "409" ]; then
    # Already exists from a prior run — look it up via the admin user list.
    user_id=$(curl -s -b "$admin_jar" "$BASE_URL/api/users" | python3 -c "
import json, sys
users = json.load(sys.stdin)
match = [u for u in users if u['username'] == '$uname']
print(match[0]['id'] if match else '')
")
    if [ -z "$user_id" ]; then
      echo "create-writer-accounts.sh: WARN $uname reported 409 but not found in user list, skipping" >&2
      rm -f "$create_resp"
      continue
    fi
  else
    echo "create-writer-accounts.sh: WARN failed to create $uname (HTTP $create_status), skipping" >&2
    cat "$create_resp" >&2
    rm -f "$create_resp"
    continue
  fi
  rm -f "$create_resp"

  entry=$(python3 -c "
import json
print(json.dumps({'id': '$user_id', 'username': '$uname', 'email': '$email'}))
")
  if [ "$first" -eq 1 ]; then first=0; else echo "," >> "$WRITER_ACCOUNTS_OUT.tmp"; fi
  printf '%s' "$entry" >> "$WRITER_ACCOUNTS_OUT.tmp"
done
echo "]" >> "$WRITER_ACCOUNTS_OUT.tmp"
mv "$WRITER_ACCOUNTS_OUT.tmp" "$WRITER_ACCOUNTS_OUT"

count=$(python3 -c "import json; print(len(json.load(open('$WRITER_ACCOUNTS_OUT'))))")
echo "create-writer-accounts.sh: done, $count writer accounts written to $WRITER_ACCOUNTS_OUT"
