#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
script="$repo_root/scripts/install-macos.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$script" ]] || fail "missing scripts/install-macos.sh"

bash -n "$script"

help_output="$("$script" --help)"
[[ "$help_output" == *"--install-dir"* ]] || fail "help omits --install-dir"
[[ "$help_output" == *"--build-dir"* ]] || fail "help omits --build-dir"
[[ "$help_output" == *"--dry-run"* ]] || fail "help omits --dry-run"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

dry_run_output="$(
  "$script" \
    --dry-run \
    --version v9.9.9 \
    --arch arm64 \
    --install-dir "$tmp_dir/bin" \
    --build-dir "$tmp_dir/releases"
)"

[[ "$dry_run_output" == *"Would build LeafWiki v9.9.9 for darwin/arm64"* ]] || fail "dry-run omits target platform"
[[ "$dry_run_output" == *"$tmp_dir/bin/leafwiki"* ]] || fail "dry-run omits target install path"
[[ "$dry_run_output" == *"CGO_ENABLED=0"* ]] || fail "dry-run omits release Go build env"
[[ ! -e "$tmp_dir/bin/leafwiki" ]] || fail "dry-run created installed binary"

printf 'PASS: install-macos script checks\n'
