#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
initial_cwd="$(pwd)"

binary_name="leafwiki-mcp-stdio"
cmd_path="./cmd/leafwiki-mcp-stdio"
build_dir="${LEAFWIKI_MCP_STDIO_BUILD_DIR:-$repo_root/releases}"
version="${LEAFWIKI_MCP_STDIO_VERSION:-${LEAFWIKI_VERSION:-}}"
target_os="${LEAFWIKI_MCP_STDIO_GOOS:-${GOOS:-}}"
target_arch="${LEAFWIKI_MCP_STDIO_GOARCH:-${GOARCH:-}}"
output_path=""
dry_run=0
write_checksum=1

usage() {
  cat <<EOF
Usage: scripts/build-mcp-stdio.sh [options]

Builds the optional leafwiki-mcp-stdio proxy/sidecar binary with the local Go
toolchain. No Docker is required.

Options:
  --build-dir <path>  Directory for default output (default: ./releases)
  --output <path>     Exact output binary path
  --version <version> Version used in the default output name (default: latest git tag or v0.1.0)
  --os <os>           Target OS: darwin, linux, or windows (default: current Go OS)
  --arch <arch>       Target architecture: arm64 or amd64 (default: current Go arch)
  --no-checksum       Do not write a .sha256 file next to the binary
  --dry-run           Print the build plan without changing files
  -h, --help          Show this help

Environment overrides:
  LEAFWIKI_MCP_STDIO_BUILD_DIR, LEAFWIKI_MCP_STDIO_VERSION
  LEAFWIKI_MCP_STDIO_GOOS, LEAFWIKI_MCP_STDIO_GOARCH
EOF
}

fail() {
  printf 'Error: %s\n' "$1" >&2
  exit 1
}

log() {
  printf '%s\n' "$1"
}

quote_command() {
  local arg
  for arg in "$@"; do
    printf '%q ' "$arg"
  done
}

run() {
  printf '+ '
  quote_command "$@"
  printf '\n'
  if [[ "$dry_run" -eq 0 ]]; then
    "$@"
  fi
}

run_in_repo() {
  printf '+ cd '
  printf '%q' "$repo_root"
  printf ' && '
  quote_command "$@"
  printf '\n'
  if [[ "$dry_run" -eq 0 ]]; then
    (cd "$repo_root" && "$@")
  fi
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  command_exists "$1" || fail "$1 is required"
}

absolute_path() {
  case "$1" in
    /*)
      printf '%s\n' "$1"
      ;;
    *)
      printf '%s/%s\n' "$initial_cwd" "$1"
      ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --build-dir)
      [[ $# -ge 2 ]] || fail "--build-dir requires a path"
      build_dir="$2"
      shift 2
      ;;
    --output)
      [[ $# -ge 2 ]] || fail "--output requires a path"
      output_path="$2"
      shift 2
      ;;
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      version="$2"
      shift 2
      ;;
    --os)
      [[ $# -ge 2 ]] || fail "--os requires darwin, linux, or windows"
      target_os="$2"
      shift 2
      ;;
    --arch)
      [[ $# -ge 2 ]] || fail "--arch requires arm64 or amd64"
      target_arch="$2"
      shift 2
      ;;
    --no-checksum)
      write_checksum=0
      shift
      ;;
    --dry-run)
      dry_run=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      fail "unknown option: $1"
      ;;
  esac
done

if [[ "$dry_run" -eq 0 ]]; then
  require_command go
fi

if [[ -z "$version" ]]; then
  if command_exists git && git -C "$repo_root" describe --tags --abbrev=0 >/dev/null 2>&1; then
    version="$(git -C "$repo_root" describe --tags --abbrev=0)"
  else
    version="v0.1.0"
  fi
fi

if [[ -z "$target_os" ]]; then
  if command_exists go; then
    target_os="$(go env GOOS)"
  else
    case "$(uname -s)" in
      Darwin)
        target_os="darwin"
        ;;
      Linux)
        target_os="linux"
        ;;
      *)
        fail "could not infer target OS; pass --os darwin, --os linux, or --os windows"
        ;;
    esac
  fi
fi

if [[ -z "$target_arch" ]]; then
  if command_exists go; then
    target_arch="$(go env GOARCH)"
  else
    case "$(uname -m)" in
      arm64|aarch64)
        target_arch="arm64"
        ;;
      x86_64)
        target_arch="amd64"
        ;;
      *)
        fail "could not infer architecture; pass --arch arm64 or --arch amd64"
        ;;
    esac
  fi
fi

case "$target_os/$target_arch" in
  linux/amd64|linux/arm64|darwin/amd64|darwin/arm64|windows/amd64)
    ;;
  *)
    fail "unsupported target '$target_os/$target_arch'; supported targets match the release matrix"
    ;;
esac

build_dir="$(absolute_path "$build_dir")"
if [[ -z "$output_path" ]]; then
  extension=""
  if [[ "$target_os" == "windows" ]]; then
    extension=".exe"
  fi
  output_path="$build_dir/$binary_name-$version-$target_os-$target_arch$extension"
else
  output_path="$(absolute_path "$output_path")"
fi
output_dir="$(dirname -- "$output_path")"
checksum_path="$output_path.sha256"

if [[ "$dry_run" -eq 1 ]]; then
  log "Would build $binary_name $version for $target_os/$target_arch"
else
  log "Building $binary_name $version for $target_os/$target_arch"
fi
log "Output: $output_path"

run mkdir -p "$output_dir"
run_in_repo env \
  CGO_ENABLED=0 \
  GOOS="$target_os" \
  GOARCH="$target_arch" \
  go build \
  -trimpath \
  -ldflags="-s -w" \
  -o "$output_path" \
  "$cmd_path"

if [[ "$dry_run" -eq 0 && ! -s "$output_path" ]]; then
  fail "build did not produce binary at $output_path"
fi

if [[ "$write_checksum" -eq 1 ]]; then
  output_base="$(basename -- "$output_path")"
  checksum_base="$(basename -- "$checksum_path")"
  if command_exists sha256sum; then
    printf '+ cd %q && sha256sum %q > %q\n' "$output_dir" "$output_base" "$checksum_base"
    if [[ "$dry_run" -eq 0 ]]; then
      (cd "$output_dir" && sha256sum "$output_base" > "$checksum_base")
    fi
  elif command_exists shasum; then
    printf '+ cd %q && shasum -a 256 %q > %q\n' "$output_dir" "$output_base" "$checksum_base"
    if [[ "$dry_run" -eq 0 ]]; then
      (cd "$output_dir" && shasum -a 256 "$output_base" > "$checksum_base")
    fi
  elif [[ "$dry_run" -eq 0 ]]; then
    fail "sha256sum or shasum is required to write checksum"
  else
    log "Would write checksum to $checksum_path"
  fi
  log "Checksum: $checksum_path"
fi

log "Built $binary_name at $output_path"
