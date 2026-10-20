#!/bin/bash
# Fails when the Go and Node versions CI uses drift from the ones the release
# images build with. CI reads Go from go.mod's toolchain line and Node from
# .nvmrc; the Dockerfiles pin their base images separately, and Dependabot
# bumps only those. Compared: the full Go version (so the Dockerfiles pin a
# patch tag, never a floating one like 1.27-alpine), Node major.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

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

go_version="$(sed -n -E 's/^toolchain go([0-9]+\.[0-9]+\.[0-9]+).*/\1/p' go.mod)"
if [[ -z "$go_version" ]]; then
    fail go.mod "go.mod has no toolchain go<major>.<minor>.<patch> line; CI reads the Go version from it."
fi

# The first non-blank line, without a leading v: 26, v26 and 26.10.0 give 26.
node_version="$(awk 'NF { sub(/^v/, "", $1); sub(/[^0-9].*/, "", $1); print $1; exit }' .nvmrc 2>/dev/null || true)"
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

if [[ "$status" -ne 0 ]]; then
    exit "$status"
fi
echo "Toolchain versions match: Go $go_version, Node $node_version."
