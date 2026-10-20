#!/bin/bash
# Fails when the Go and Node versions CI uses drift from the ones the release
# images build with. CI reads Go from go.mod's toolchain line and Node from
# .nvmrc; the Dockerfiles pin their base images separately, and Dependabot
# bumps only those. Compared: the full Go version (so the Dockerfiles pin a
# patch tag, never a floating one like 1.27-alpine), Node major. The
# committed mise config takes Node from .nvmrc and Go as a bootstrap only.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=lib/toolchain-versions.sh
. scripts/lib/toolchain-versions.sh

DOCKERFILES=(Dockerfile Dockerfile.builder)
status=0

fail() {
    echo "::error file=$1::$2"
    status=1
}

# Prints the tag version of every `FROM <image>:<version>-...` line, one per
# line, e.g. 1.27.3 for `FROM golang:1.27.3-alpine@sha256:...`. Accepts any case
# of FROM, flags such as --platform=... and a docker.io/library/ prefix.
image_versions() {
    awk -v image="$2" '
        { sub(/\r$/, "") }
        tolower($1) == "from" {
            i = 2
            while (i <= NF && $i ~ /^--/) i++
            ref = $i
            sub(/^docker\.io\//, "", ref)
            sub(/^library\//, "", ref)
            if (index(ref, image ":") != 1) next
            tag = substr(ref, length(image) + 2)
            if (match(tag, /^[0-9][0-9.]*/)) print substr(tag, 1, RLENGTH)
        }
    ' "$1"
}

go_version="$(toolchain_go_version || true)"
if [[ -z "$go_version" ]]; then
    fail go.mod "go.mod has no toolchain go<major>.<minor>.<patch> line; CI reads the Go version from it."
fi

node_version="$(toolchain_node_major || true)"
if [[ -z "$node_version" ]]; then
    fail .nvmrc ".nvmrc is missing or has no Node major version; CI reads the Node version from it."
fi

# check <dockerfile> <image> <label> <want> <source> <cut>: every FROM line
# of <image> must match <want> after cut -d. -f<cut>.
check() {
    local dockerfile="$1" image="$2" label="$3" want="$4" source="$5" fields="$6"
    local versions version
    [[ -n "$want" ]] || return 0
    versions="$(image_versions "$dockerfile" "$image")"
    if [[ -z "$versions" ]]; then
        fail "$dockerfile" "No $image image in $dockerfile, but $source has $label $want."
        return 0
    fi
    while IFS= read -r version; do
        version="$(cut -d. -f"$fields" <<< "$version")"
        if [[ "$version" != "$want" ]]; then
            fail "$dockerfile" "$label $version in $dockerfile, but $source has $want. Update both together."
        fi
    done <<< "$versions"
}

for dockerfile in "${DOCKERFILES[@]}"; do
    if [[ ! -f "$dockerfile" ]]; then
        fail "$dockerfile" "$dockerfile is missing."
        continue
    fi
    check "$dockerfile" golang Go "$go_version" "go.mod's toolchain line" 1-3
    check "$dockerfile" node Node "$node_version" .nvmrc 1
done

# The committed mise configs carry no versions of their own: Node comes from
# .nvmrc and Go from go.mod's toolchain line. Deliberately strict and
# syntax-blind, so no TOML spelling (aliases, backend prefixes, dotted keys,
# inline tables) gets around it: after comments are stripped, any line naming
# node fails, except the setting that makes mise read .nvmrc; a line naming
# go or golang passes only as exactly go = "latest" (a bootstrap) or the
# toolchain version. .tool-versions must not name either. Per-user overrides
# (mise.local.toml, .config/) are left alone.
# mise_violations <file>: prints "<line number>: <line>" per violation.
mise_violations() {
    awk -v strict="$([[ "$1" == .tool-versions ]] && echo 1)" -v want="$go_version" '
        { sub(/\r$/, ""); sub(/#.*/, ""); line = tolower($0); bare = $0; gsub(/[ \t]/, "", bare) }
        !strict && line ~ /^[ \t]*idiomatic_version_file_enable_tools[ \t]*=/ { next }
        line ~ /(^|[^a-z0-9_])node/ { print FNR ": " $0; next }
        line ~ /(^|[^a-z0-9_])go(lang)?([^a-z0-9_]|$)/ {
            if (strict || (bare != "go=\"latest\"" && bare != "go=\047latest\047" &&
                bare != "go=\"" want "\"" && bare != "go=\047" want "\047")) print FNR ": " $0
        }
    ' "$1"
}

for file in mise.toml .mise.toml mise/config.toml .mise/config.toml .tool-versions; do
    [[ -f "$file" ]] || continue
    while IFS= read -r violation; do
        fail "$file" "$file:$violation: Node comes from .nvmrc and Go from go.mod; the only allowed entry is go = \"latest\" or go = \"$go_version\"."
    done < <(mise_violations "$file")
done

if [[ "$status" -ne 0 ]]; then
    exit "$status"
fi
echo "Toolchain versions match: Go $go_version, Node $node_version."
