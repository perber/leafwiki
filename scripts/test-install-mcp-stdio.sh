#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
script="$repo_root/scripts/install-mcp-stdio.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$script" ]] || fail "missing scripts/install-mcp-stdio.sh"

bash -n "$script"

help_output="$("$script" --help)"
[[ "$help_output" == *"--install-dir"* ]] || fail "help omits --install-dir"
[[ "$help_output" == *"--build-dir"* ]] || fail "help omits --build-dir"
[[ "$help_output" == *"--version"* ]] || fail "help omits --version"
[[ "$help_output" == *"--dry-run"* ]] || fail "help omits --dry-run"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
mkdir -p "$tmp_dir/bin"

dry_run_output="$(
  "$script" \
    --dry-run \
    --version v9.9.9 \
    --os darwin \
    --arch arm64 \
    --build-dir "$tmp_dir/releases" \
    --install-dir "$tmp_dir/bin"
)"

[[ "$dry_run_output" == *"Would build and install leafwiki-mcp-stdio v9.9.9 for darwin/arm64"* ]] || fail "dry-run omits target platform"
[[ "$dry_run_output" == *"scripts/build-mcp-stdio.sh"* ]] || fail "dry-run omits build script"
[[ "$dry_run_output" == *"$tmp_dir/bin/leafwiki-mcp-stdio"* ]] || fail "dry-run omits install target"
[[ ! -e "$tmp_dir/bin/leafwiki-mcp-stdio" ]] || fail "dry-run installed binary"

printf 'PASS: install-mcp-stdio script checks\n'
