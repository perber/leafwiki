#!/bin/sh
# Prints the toolchain versions and compares them with .nvmrc and go.mod's
# toolchain line. Exits 1 when go, node or npm is missing or Node's major
# version differs from .nvmrc: npm ci and the frontend build expect that
# version, so carrying on would fail later with a less obvious error. mise is
# optional and only reported.
set -u

cd "$(dirname "$0")/.."
# shellcheck source=lib/toolchain-versions.sh
. scripts/lib/toolchain-versions.sh
status=0

want_node="$(toolchain_node_major)"
want_go="$(toolchain_go_version)"

problem() {
    echo "  ! $1"
    status=1
}

# `go env` inside the module switches to (and if needed downloads) the
# toolchain from go.mod, so this shows the Go that builds LeafWiki.
if command -v go >/dev/null 2>&1; then
    echo "checking Go (may download the go.mod toolchain)..."
    if have_go="$(go env GOVERSION 2>/dev/null)"; then
        have_go="${have_go#go}"
        echo "go    $have_go (go.mod: $want_go)"
        if [ "$have_go" != "$want_go" ]; then
            echo "  note: CI builds with $want_go; GOTOOLCHAIN=local or a newer local Go keeps $have_go."
        fi
    else
        echo "go    found, but 'go env' failed"
        problem "Go could not switch to $want_go from go.mod (offline, or GOTOOLCHAIN=local with an older Go?)."
    fi
else
    echo "go    not found"
    problem "Install Go 1.21 or newer (mise install, or https://go.dev/dl/); go.mod selects the exact version."
fi

if command -v node >/dev/null 2>&1; then
    have_node="$(node --version)"
    echo "node  ${have_node#v} (.nvmrc: $want_node)"
    have_major="${have_node#v}"
    have_major="${have_major%%.*}"
    if [ "$have_major" != "$want_node" ]; then
        problem "Node $want_node is expected: run 'mise install', or 'nvm use' / 'fnm use'."
    fi
else
    echo "node  not found"
    problem "Install Node $want_node: 'mise install', or 'nvm install' / 'fnm install' (they read .nvmrc)."
fi

if command -v npm >/dev/null 2>&1; then
    echo "npm   $(npm --version)"
else
    echo "npm   not found"
    problem "npm ships with Node; reinstall Node $want_node."
fi

if command -v mise >/dev/null 2>&1; then
    echo "mise  $(mise --version)"
else
    echo "mise  not found (optional; nvm or fnm with .nvmrc work too)"
fi

if [ ! -d ui/leafwiki-ui/node_modules ] || [ ! -d e2e/node_modules ]; then
    echo "  note: npm dependencies are not installed yet; run 'make setup'."
fi

if [ "$status" -eq 0 ]; then
    echo "Toolchain OK."
fi
exit "$status"
