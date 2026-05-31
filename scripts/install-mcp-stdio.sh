#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
initial_cwd="$(pwd)"

binary_name="leafwiki-mcp-stdio"
build_script="$script_dir/build-mcp-stdio.sh"
install_dir="${LEAFWIKI_MCP_STDIO_INSTALL_DIR:-/usr/local/bin}"
build_dir="${LEAFWIKI_MCP_STDIO_BUILD_DIR:-$repo_root/releases}"
version="${LEAFWIKI_MCP_STDIO_VERSION:-${LEAFWIKI_VERSION:-}}"
target_os="${LEAFWIKI_MCP_STDIO_GOOS:-${GOOS:-}}"
target_arch="${LEAFWIKI_MCP_STDIO_GOARCH:-${GOARCH:-}}"
dry_run=0
write_checksum=1

usage() {
  cat <<EOF
Usage: scripts/install-mcp-stdio.sh [options]

Builds the optional leafwiki-mcp-stdio proxy/sidecar binary with the local Go
toolchain and installs it as leafwiki-mcp-stdio.

Options:
  --install-dir <path> Directory to install leafwiki-mcp-stdio into (default: /usr/local/bin)
  --build-dir <path>   Directory for the built binary (default: ./releases)
  --version <version>  Version used in the build output name (default: latest git tag or v0.1.0)
  --os <os>            Target OS: darwin or linux (default: current Go OS)
  --arch <arch>        Target architecture: arm64 or amd64 (default: current Go arch)
  --no-checksum        Do not write a .sha256 file next to the built binary
  --dry-run            Print the build/install plan without changing files
  -h, --help           Show this help

Environment overrides:
  LEAFWIKI_MCP_STDIO_INSTALL_DIR, LEAFWIKI_MCP_STDIO_BUILD_DIR
  LEAFWIKI_MCP_STDIO_VERSION, LEAFWIKI_MCP_STDIO_GOOS, LEAFWIKI_MCP_STDIO_GOARCH
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

run_build_script() {
  local args=(
    --version "$version"
    --os "$target_os"
    --arch "$target_arch"
    --build-dir "$build_dir"
  )
  if [[ "$write_checksum" -eq 0 ]]; then
    args+=(--no-checksum)
  fi

  if [[ "$dry_run" -eq 1 ]]; then
    printf '+ '
    quote_command "$build_script" --dry-run "${args[@]}"
    printf '\n'
    "$build_script" --dry-run "${args[@]}"
  else
    run "$build_script" "${args[@]}"
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-dir)
      [[ $# -ge 2 ]] || fail "--install-dir requires a path"
      install_dir="$2"
      shift 2
      ;;
    --build-dir)
      [[ $# -ge 2 ]] || fail "--build-dir requires a path"
      build_dir="$2"
      shift 2
      ;;
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      version="$2"
      shift 2
      ;;
    --os)
      [[ $# -ge 2 ]] || fail "--os requires darwin or linux"
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

[[ -x "$build_script" ]] || fail "missing executable build script at $build_script"

if [[ "$dry_run" -eq 0 ]]; then
  require_command install
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
        fail "could not infer target OS; pass --os darwin or --os linux"
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
  darwin/amd64|darwin/arm64|linux/amd64|linux/arm64)
    ;;
  *)
    fail "unsupported install target '$target_os/$target_arch'; use darwin/linux on amd64/arm64"
    ;;
esac

install_dir="$(absolute_path "$install_dir")"
build_dir="$(absolute_path "$build_dir")"
built_binary="$build_dir/$binary_name-$version-$target_os-$target_arch"
target_binary="$install_dir/$binary_name"

if [[ "$dry_run" -eq 1 ]]; then
  log "Would build and install $binary_name $version for $target_os/$target_arch"
else
  log "Building and installing $binary_name $version for $target_os/$target_arch"
fi
log "Build output: $built_binary"
log "Install target: $target_binary"

run_build_script

if [[ "$dry_run" -eq 0 && ! -s "$built_binary" ]]; then
  fail "build did not produce binary at $built_binary"
fi

if [[ ! -d "$install_dir" ]]; then
  parent_dir="$(dirname -- "$install_dir")"
  if [[ -d "$parent_dir" && -w "$parent_dir" ]]; then
    run mkdir -p "$install_dir"
  else
    require_command sudo
    run sudo mkdir -p "$install_dir"
  fi
fi

if [[ -d "$install_dir" && -w "$install_dir" ]]; then
  run install -m 0755 "$built_binary" "$target_binary"
else
  require_command sudo
  run sudo install -m 0755 "$built_binary" "$target_binary"
fi

if [[ "$dry_run" -eq 0 && ! -x "$target_binary" ]]; then
  fail "installed binary is not executable at $target_binary"
fi

log "Installed $binary_name to $target_binary"
