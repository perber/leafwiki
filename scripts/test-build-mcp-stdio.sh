#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
script="$repo_root/scripts/build-mcp-stdio.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$script" ]] || fail "missing scripts/build-mcp-stdio.sh"

bash -n "$script"

help_output="$("$script" --help)"
[[ "$help_output" == *"--output"* ]] || fail "help omits --output"
[[ "$help_output" == *"--os"* ]] || fail "help omits --os"
[[ "$help_output" == *"--arch"* ]] || fail "help omits --arch"
[[ "$help_output" == *"--dry-run"* ]] || fail "help omits --dry-run"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

dry_run_output="$(
  "$script" \
    --dry-run \
    --version v9.9.9 \
    --os darwin \
    --arch arm64 \
    --output "$tmp_dir/leafwiki-mcp-stdio"
)"

[[ "$dry_run_output" == *"Would build leafwiki-mcp-stdio v9.9.9 for darwin/arm64"* ]] || fail "dry-run omits target platform"
[[ "$dry_run_output" == *"$tmp_dir/leafwiki-mcp-stdio"* ]] || fail "dry-run omits output path"
[[ "$dry_run_output" == *"CGO_ENABLED=0"* ]] || fail "dry-run omits release Go build env"
[[ "$dry_run_output" == *"./cmd/leafwiki-mcp-stdio"* ]] || fail "dry-run omits sidecar command package"
[[ ! -e "$tmp_dir/leafwiki-mcp-stdio" ]] || fail "dry-run created output binary"

printf 'PASS: build-mcp-stdio script checks\n'
