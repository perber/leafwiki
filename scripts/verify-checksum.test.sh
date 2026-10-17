#!/bin/bash
# Tests verify_checksum() in install.sh and update.sh against a local HTTP
# server. The scripts run their install steps at top level, so the function
# is extracted with sed instead of sourcing the whole script.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
SERVER_PID=""
cleanup() {
    [[ -n "$SERVER_PID" ]] && kill "$SERVER_PID" 2>/dev/null
    rm -rf "$WORK"
}
trap cleanup EXIT

mkdir -p "$WORK/srv" "$WORK/dl"
echo "leafwiki binary" > "$WORK/srv/good"
sha256sum "$WORK/srv/good" | awk '{print $1 "  good"}' > "$WORK/srv/good.sha256"
echo "leafwiki binary" > "$WORK/srv/legacy"
sha256sum "$WORK/srv/legacy" | awk '{print $1 "  releases/legacy"}' > "$WORK/srv/legacy.sha256"
echo "leafwiki binary" > "$WORK/srv/tampered"
echo "0000000000000000000000000000000000000000000000000000000000000000  tampered" > "$WORK/srv/tampered.sha256"
echo "leafwiki binary" > "$WORK/srv/nochecksum"

PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1])')
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$WORK/srv" >/dev/null 2>&1 &
SERVER_PID=$!
for _ in $(seq 50); do
    curl -s "http://127.0.0.1:$PORT/" >/dev/null && break
    sleep 0.1
done
BASE="http://127.0.0.1:$PORT"

FAILURES=0
expect() {
    local script="$1" name="$2" want="$3"
    local fn got
    fn="$(sed -n '/^verify_checksum()/,/^}/p' "$ROOT/$script")"
    if [[ -z "$fn" ]]; then
        echo "FAIL $script: verify_checksum() not found"
        FAILURES=$((FAILURES + 1))
        return
    fi
    cp "$WORK/srv/$name" "$WORK/dl/$name"
    if bash -c "$fn"$'\n'"verify_checksum \"\$1\" \"\$2\"" _ "$WORK/dl/$name" "$BASE/$name" 2>/dev/null; then
        got="ok"
    else
        got="fail"
    fi
    if [[ "$got" == "$want" ]]; then
        echo "PASS $script: $name -> $got"
    else
        echo "FAIL $script: $name -> $got, want $want"
        FAILURES=$((FAILURES + 1))
    fi
}

for script in install.sh update.sh; do
    expect "$script" good ok
    expect "$script" legacy ok
    expect "$script" tampered fail
    expect "$script" nochecksum fail
done

if [[ "$FAILURES" -ne 0 ]]; then
    echo "$FAILURES failure(s)"
    exit 1
fi
