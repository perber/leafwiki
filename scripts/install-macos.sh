#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/.." && pwd)"

binary_name="leafwiki"
install_dir="${LEAFWIKI_INSTALL_DIR:-/usr/local/bin}"
build_dir="${LEAFWIKI_BUILD_DIR:-$repo_root/releases}"
version="${LEAFWIKI_VERSION:-}"
arch="${LEAFWIKI_ARCH:-}"
dry_run=0
skip_npm_ci=0

usage() {
  cat <<EOF
Usage: scripts/install-macos.sh [options]

Builds a production LeafWiki binary for macOS from this checkout and installs it
as leafwiki.

Options:
  --install-dir <path>  Directory to install leafwiki into (default: /usr/local/bin)
  --build-dir <path>    Directory for the built binary (default: ./releases)
  --version <version>   Version string passed to the UI (default: latest git tag or v0.1.0)
  --arch <arch>         Target architecture: arm64 or amd64 (default: current Go arch)
  --skip-npm-ci         Reuse existing frontend dependencies
  --dry-run             Print the build/install plan without changing files
  -h, --help            Show this help

Environment overrides:
  LEAFWIKI_INSTALL_DIR, LEAFWIKI_BUILD_DIR, LEAFWIKI_VERSION, LEAFWIKI_ARCH
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
    --arch)
      [[ $# -ge 2 ]] || fail "--arch requires arm64 or amd64"
      arch="$2"
      shift 2
      ;;
    --skip-npm-ci)
      skip_npm_ci=1
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

if [[ "$(uname -s)" != "Darwin" ]]; then
  if [[ "$dry_run" -eq 1 ]]; then
    log "Dry run only: actual install requires macOS."
  else
    fail "this installer is for macOS only"
  fi
fi

if [[ "$dry_run" -eq 0 ]]; then
  require_command go
  require_command npm
  require_command git
  require_command install
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

ui_dir="$repo_root/ui/leafwiki-ui"
ui_dist="$ui_dir/dist"
embedded_dist="$repo_root/internal/http/dist"
output_binary="$build_dir/$binary_name-$version-darwin-$arch"
target_binary="$install_dir/$binary_name"

if [[ "$dry_run" -eq 1 ]]; then
  log "Would build LeafWiki $version for darwin/$arch"
else
  log "Building LeafWiki $version for darwin/$arch"
fi
log "Build output: $output_binary"
log "Install target: $target_binary"

if [[ "$skip_npm_ci" -eq 0 ]]; then
  run npm --prefix "$ui_dir" ci --ignore-scripts
fi

run env VITE_API_URL=/ APP_VERSION="$version" npm --prefix "$ui_dir" run build

run mkdir -p "$embedded_dist"
run find "$embedded_dist" -mindepth 1 ! -name .gitkeep -exec rm -rf '{}' +
run cp -R "$ui_dist/." "$embedded_dist/"
run touch "$embedded_dist/.gitkeep"

run mkdir -p "$build_dir"
run_in_repo env \
  CGO_ENABLED=0 \
  GOOS=darwin \
  GOARCH="$arch" \
  go build \
  -ldflags="-s -w -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production" \
  -o "$output_binary" \
  ./cmd/leafwiki/main.go

if [[ "$dry_run" -eq 1 ]]; then
  if [[ -d "$install_dir" && -w "$install_dir" ]]; then
    run install -m 0755 "$output_binary" "$target_binary"
  else
    run sudo install -m 0755 "$output_binary" "$target_binary"
  fi
  exit 0
fi

if [[ ! -x "$output_binary" ]]; then
  fail "build did not produce executable binary at $output_binary"
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

if [[ -w "$install_dir" ]]; then
  run install -m 0755 "$output_binary" "$target_binary"
else
  require_command sudo
  run sudo install -m 0755 "$output_binary" "$target_binary"
fi

if [[ ! -x "$target_binary" ]]; then
  fail "installed binary is not executable at $target_binary"
fi

log "Installed LeafWiki to $target_binary"
