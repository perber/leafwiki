#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
script="$repo_root/scripts/install-all-macos.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

[[ -f "$script" ]] || fail "missing scripts/install-all-macos.sh"

bash -n "$script"

help_output="$("$script" --help)"
[[ "$help_output" == *"--install-dir"* ]] || fail "help omits --install-dir"
[[ "$help_output" == *"--build-dir"* ]] || fail "help omits --build-dir"
[[ "$help_output" == *"--skip-npm-ci"* ]] || fail "help omits --skip-npm-ci"
[[ "$help_output" == *"--dry-run"* ]] || fail "help omits --dry-run"
[[ "$help_output" == *"run-mcp.sh"* ]] || fail "help omits run-mcp.sh"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT
mkdir -p "$tmp_dir/bin"

dry_run_output="$(
  "$script" \
    --dry-run \
    --version v9.9.9 \
    --arch arm64 \
    --install-dir "$tmp_dir/bin" \
    --build-dir "$tmp_dir/releases"
)"

[[ "$dry_run_output" == *"Would install LeafWiki, leafwiki-mcp-stdio, and run-mcp.sh v9.9.9 for darwin/arm64"* ]] || fail "dry-run omits aggregate target"
[[ "$dry_run_output" == *"scripts/install-macos.sh"* ]] || fail "dry-run omits main installer"
[[ "$dry_run_output" == *"scripts/install-mcp-stdio.sh"* ]] || fail "dry-run omits MCP STDIO installer"
[[ "$dry_run_output" == *"$tmp_dir/bin/leafwiki"* ]] || fail "dry-run omits leafwiki install target"
[[ "$dry_run_output" == *"$tmp_dir/bin/leafwiki-mcp-stdio"* ]] || fail "dry-run omits sidecar install target"
[[ "$dry_run_output" == *"$tmp_dir/bin/run-mcp.sh"* ]] || fail "dry-run omits run-mcp.sh install target"
[[ ! -e "$tmp_dir/bin/leafwiki" ]] || fail "dry-run installed leafwiki"
[[ ! -e "$tmp_dir/bin/leafwiki-mcp-stdio" ]] || fail "dry-run installed leafwiki-mcp-stdio"
[[ ! -e "$tmp_dir/bin/run-mcp.sh" ]] || fail "dry-run installed run-mcp.sh"

printf 'PASS: install-all-macos script checks\n'
