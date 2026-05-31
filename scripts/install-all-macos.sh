#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"
initial_cwd="$(pwd)"

leafwiki_installer="$script_dir/install-macos.sh"
mcp_stdio_installer="$script_dir/install-mcp-stdio.sh"
run_mcp_script="$script_dir/run-mcp.sh"
install_dir="${LEAFWIKI_INSTALL_DIR:-${LEAFWIKI_MCP_STDIO_INSTALL_DIR:-/usr/local/bin}}"
server_build_dir="${LEAFWIKI_BUILD_DIR:-$repo_root/releases}"
mcp_build_dir="${LEAFWIKI_MCP_STDIO_BUILD_DIR:-$repo_root/releases}"
version="${LEAFWIKI_VERSION:-${LEAFWIKI_MCP_STDIO_VERSION:-}}"
arch="${LEAFWIKI_ARCH:-${LEAFWIKI_MCP_STDIO_GOARCH:-${GOARCH:-}}}"
dry_run=0
skip_npm_ci=0
write_checksum=1

usage() {
  cat <<EOF
Usage: scripts/install-all-macos.sh [options]

Builds and installs both local macOS executables from this checkout:
  - leafwiki
  - leafwiki-mcp-stdio
  - run-mcp.sh

Options:
  --install-dir <path>       Directory to install both binaries into (default: /usr/local/bin)
  --build-dir <path>         Shared build output directory for both binaries (default: ./releases)
  --server-build-dir <path>  Build output directory for leafwiki only
  --mcp-build-dir <path>     Build output directory for leafwiki-mcp-stdio only
  --version <version>        Version passed to both installers (default: latest git tag or v0.1.0)
  --arch <arch>              Target architecture: arm64 or amd64 (default: current Go arch)
  --skip-npm-ci              Reuse existing frontend dependencies for the main leafwiki build
  --no-checksum              Do not write a .sha256 file for the MCP STDIO sidecar build
  --dry-run                  Print the build/install plan without changing files
  -h, --help                 Show this help

Environment overrides:
  LEAFWIKI_INSTALL_DIR, LEAFWIKI_BUILD_DIR, LEAFWIKI_VERSION, LEAFWIKI_ARCH
  LEAFWIKI_MCP_STDIO_INSTALL_DIR, LEAFWIKI_MCP_STDIO_BUILD_DIR
  LEAFWIKI_MCP_STDIO_VERSION, LEAFWIKI_MCP_STDIO_GOARCH
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
  "$@"
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
    --install-dir)
      [[ $# -ge 2 ]] || fail "--install-dir requires a path"
      install_dir="$2"
      shift 2
      ;;
    --build-dir)
      [[ $# -ge 2 ]] || fail "--build-dir requires a path"
      server_build_dir="$2"
      mcp_build_dir="$2"
      shift 2
      ;;
    --server-build-dir)
      [[ $# -ge 2 ]] || fail "--server-build-dir requires a path"
      server_build_dir="$2"
      shift 2
      ;;
    --mcp-build-dir)
      [[ $# -ge 2 ]] || fail "--mcp-build-dir requires a path"
      mcp_build_dir="$2"
      shift 2
      ;;
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      version="$2"
      shift 2
      ;;
    --arch)
      [[ $# -ge 2 ]] || fail "--arch requires arm64 or amd64"
      arch="$2"
      shift 2
      ;;
    --skip-npm-ci)
      skip_npm_ci=1
      shift
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

[[ -x "$leafwiki_installer" ]] || fail "missing executable installer at $leafwiki_installer"
[[ -x "$mcp_stdio_installer" ]] || fail "missing executable installer at $mcp_stdio_installer"
[[ -x "$run_mcp_script" ]] || fail "missing executable wrapper at $run_mcp_script"

if [[ "$(uname -s)" != "Darwin" ]]; then
  if [[ "$dry_run" -eq 1 ]]; then
    log "Dry run only: actual install requires macOS."
  else
    fail "this installer is for macOS only"
  fi
fi

if [[ -z "$version" ]]; then
  if command_exists git && git -C "$repo_root" describe --tags --abbrev=0 >/dev/null 2>&1; then
    version="$(git -C "$repo_root" describe --tags --abbrev=0)"
  else
    version="v0.1.0"
  fi
fi

if [[ -z "$arch" ]]; then
  if command_exists go; then
    arch="$(go env GOARCH)"
  else
    case "$(uname -m)" in
      arm64|aarch64)
        arch="arm64"
        ;;
      x86_64)
        arch="amd64"
        ;;
      *)
        fail "could not infer architecture; pass --arch arm64 or --arch amd64"
        ;;
    esac
  fi
fi

case "$arch" in
  arm64|amd64)
    ;;
  *)
    fail "unsupported architecture '$arch'; expected arm64 or amd64"
    ;;
esac

install_dir="$(absolute_path "$install_dir")"
server_build_dir="$(absolute_path "$server_build_dir")"
mcp_build_dir="$(absolute_path "$mcp_build_dir")"
run_mcp_target="$install_dir/run-mcp.sh"

leafwiki_args=(
  --install-dir "$install_dir"
  --build-dir "$server_build_dir"
  --version "$version"
  --arch "$arch"
)
if [[ "$skip_npm_ci" -eq 1 ]]; then
  leafwiki_args+=(--skip-npm-ci)
fi
if [[ "$dry_run" -eq 1 ]]; then
  leafwiki_args+=(--dry-run)
fi

mcp_stdio_args=(
  --install-dir "$install_dir"
  --build-dir "$mcp_build_dir"
  --version "$version"
  --os darwin
  --arch "$arch"
)
if [[ "$write_checksum" -eq 0 ]]; then
  mcp_stdio_args+=(--no-checksum)
fi
if [[ "$dry_run" -eq 1 ]]; then
  mcp_stdio_args+=(--dry-run)
fi

if [[ "$dry_run" -eq 1 ]]; then
  log "Would install LeafWiki, leafwiki-mcp-stdio, and run-mcp.sh $version for darwin/$arch"
else
  log "Installing LeafWiki, leafwiki-mcp-stdio, and run-mcp.sh $version for darwin/$arch"
fi
log "Install directory: $install_dir"
log "LeafWiki build directory: $server_build_dir"
log "MCP STDIO build directory: $mcp_build_dir"
log "run-mcp.sh install target: $run_mcp_target"

run "$leafwiki_installer" "${leafwiki_args[@]}"
run "$mcp_stdio_installer" "${mcp_stdio_args[@]}"

if [[ "$dry_run" -eq 1 ]]; then
  if [[ -d "$install_dir" && -w "$install_dir" ]]; then
    printf '+ '
    quote_command install -m 0755 "$run_mcp_script" "$run_mcp_target"
    printf '\n'
  else
    printf '+ '
    quote_command sudo install -m 0755 "$run_mcp_script" "$run_mcp_target"
    printf '\n'
  fi
else
  require_command install
  if [[ ! -d "$install_dir" ]]; then
    parent_dir="$(dirname -- "$install_dir")"
    if [[ -d "$parent_dir" && -w "$parent_dir" ]]; then
      run mkdir -p "$install_dir"
    else
      require_command sudo
      run sudo mkdir -p "$install_dir"
    fi
  fi

  if [[ -w "$install_dir" ]]; then
    run install -m 0755 "$run_mcp_script" "$run_mcp_target"
  else
    require_command sudo
    run sudo install -m 0755 "$run_mcp_script" "$run_mcp_target"
  fi
fi

if [[ "$dry_run" -eq 0 ]]; then
  [[ -x "$install_dir/leafwiki" ]] || fail "leafwiki was not installed at $install_dir/leafwiki"
  [[ -x "$install_dir/leafwiki-mcp-stdio" ]] || fail "leafwiki-mcp-stdio was not installed at $install_dir/leafwiki-mcp-stdio"
  [[ -x "$run_mcp_target" ]] || fail "run-mcp.sh was not installed at $run_mcp_target"
  log "Installed LeafWiki binaries and run-mcp.sh to $install_dir"
else
  log "Dry run complete for LeafWiki binaries and run-mcp.sh into $install_dir"
fi
