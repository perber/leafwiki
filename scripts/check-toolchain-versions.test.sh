#!/bin/bash
# Tests check-toolchain-versions.sh against fixture repositories in a temp
# dir: a copy of the script runs from <fixture>/scripts, so it reads the
# fixture's go.mod, .nvmrc and Dockerfiles instead of the real ones.
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

GO_FROM='FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS backend'
NODE_FROM='FROM node:26-alpine@sha256:0b36e8c136b94cd4fcf02188228e76c31ad5872eef3fec8cbd2eee500cfd9e80 AS frontend'

FAILURES=0
# expect <name> <want ok|fail> <go.mod toolchain line> <.nvmrc content (printf)> <Dockerfile body (printf)>
expect() {
    local name="$1" want="$2" toolchain="$3" nvmrc="$4" dockerfile="$5"
    local dir="$WORK/$name" got
    mkdir -p "$dir/scripts"
    cp "$ROOT/scripts/check-toolchain-versions.sh" "$dir/scripts/"
    printf 'module example.com/x\n\ngo 1.26.0\n\n%s\n' "$toolchain" > "$dir/go.mod"
    printf '%b' "$nvmrc" > "$dir/.nvmrc"
    printf '%b' "$dockerfile" > "$dir/Dockerfile"
    printf '%b' "$dockerfile" > "$dir/Dockerfile.builder"
    # Run from another directory: the script must find its repository itself.
    if (cd "$WORK" && "$dir/scripts/check-toolchain-versions.sh") >/dev/null 2>&1; then
        got="ok"
    else
        got="fail"
    fi
    if [[ "$got" == "$want" ]]; then
        echo "PASS $name -> $got"
    else
        echo "FAIL $name -> $got, want $want"
        FAILURES=$((FAILURES + 1))
    fi
}

BOTH="$NODE_FROM\n$GO_FROM\n"
expect matching                ok   'toolchain go1.27.1' '26\n'      "$BOTH"
expect toolchain-without-patch ok   'toolchain go1.27'   '26\n'      "$BOTH"
expect nvmrc-v-prefix          ok   'toolchain go1.27.1' 'v26\n'     "$BOTH"
expect nvmrc-full-version      ok   'toolchain go1.27.1' '26.10.0'   "$BOTH"
expect nvmrc-crlf              ok   'toolchain go1.27.1' '26\r\n'    "$BOTH"
expect image-patch-tag         ok   'toolchain go1.27.1' '26\n'      "$NODE_FROM\nFROM golang:1.27.1-alpine AS backend\n"
expect platform-flag           ok   'toolchain go1.27.1' '26\n'      "$NODE_FROM\nFROM --platform=\$BUILDPLATFORM golang:1.27-alpine AS backend\n"
expect lowercase-from          ok   'toolchain go1.27.1' '26\n'      "from node:26-alpine AS frontend\nfrom golang:1.27-alpine AS backend\n"
expect docker-io-prefix        ok   'toolchain go1.27.1' '26\n'      "FROM docker.io/library/node:26-alpine\nFROM docker.io/library/golang:1.27-alpine\n"
expect nvmrc-leading-blank     ok   'toolchain go1.27.1' '\n\n26\n'  "$BOTH"
expect go-drift                fail 'toolchain go1.27.1' '26\n'      "$NODE_FROM\nFROM golang:1.26-alpine AS backend\n"
expect second-go-stage-drift   fail 'toolchain go1.27.1' '26\n'      "$BOTH""FROM golang:1.25-alpine AS tools\n"
expect node-drift              fail 'toolchain go1.27.1' '24\n'      "$BOTH"
expect no-toolchain-line       fail ''                   '26\n'      "$BOTH"
expect nvmrc-lts-alias         fail 'toolchain go1.27.1' 'lts/*\n'   "$BOTH"
expect nvmrc-empty             fail 'toolchain go1.27.1' ''          "$BOTH"
expect no-go-image             fail 'toolchain go1.27.1' '26\n'      "$NODE_FROM\n"

# Missing files must fail, never print the success line.
dir="$WORK/missing"
mkdir -p "$dir/scripts"
cp "$ROOT/scripts/check-toolchain-versions.sh" "$dir/scripts/"
for file in go.mod .nvmrc Dockerfile; do
    rm -f "$dir/go.mod" "$dir/.nvmrc" "$dir/Dockerfile" "$dir/Dockerfile.builder"
    printf 'module example.com/x\n\ngo 1.26.0\n\ntoolchain go1.27.1\n' > "$dir/go.mod"
    printf '26\n' > "$dir/.nvmrc"
    printf '%b' "$BOTH" > "$dir/Dockerfile"
    printf '%b' "$BOTH" > "$dir/Dockerfile.builder"
    rm "$dir/$file"
    if out="$("$dir/scripts/check-toolchain-versions.sh" 2>&1)" || [[ "$out" == *"versions match"* ]]; then
        echo "FAIL missing $file -> ok, want fail"
        FAILURES=$((FAILURES + 1))
    else
        echo "PASS missing $file -> fail"
    fi
done

if [[ "$FAILURES" -ne 0 ]]; then
    echo "$FAILURES failure(s)"
    exit 1
fi
