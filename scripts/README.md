# Scripts

This directory contains local helper scripts for building, installing, release
prep, and lightweight script validation. Most scripts are intended to be run
from the repository root, although they resolve the repo root from their own
location where practical.

When in doubt, run the relevant script with `--help` or `--dry-run` first.

## Common Flows

### Install both local macOS executables

Use this when you want the main LeafWiki server and the MCP STDIO sidecar in
the same local bin directory.

```bash
./scripts/install-all-macos.sh --dry-run --install-dir "$HOME/.local/bin"
./scripts/install-all-macos.sh --install-dir "$HOME/.local/bin"
```

This installs:

- `leafwiki`
- `leafwiki-mcp-stdio`
- `run-mcp.sh`

It delegates to `install-macos.sh` and `install-mcp-stdio.sh`, so the same build
and install behavior applies. It also installs the MCP wrapper script next to
the binaries so clients can spawn it from a stable path.

### Install LeafWiki from this checkout on macOS

Use this when you want a local production-style `leafwiki` binary built from
the current working tree.

```bash
./scripts/install-macos.sh --dry-run
./scripts/install-macos.sh
```

Default install target: `/usr/local/bin/leafwiki`.

This script builds the frontend, copies it into `internal/http/dist`, builds the
Go server with production frontend embedding enabled, and installs the binary.
It may use `sudo` if the install directory is not writable.

### Build the MCP STDIO sidecar without installing it

Use this when you only want the `leafwiki-mcp-stdio` binary artifact.

```bash
./scripts/build-mcp-stdio.sh
```

Default output:

```text
releases/leafwiki-mcp-stdio-<version>-<os>-<arch>
releases/leafwiki-mcp-stdio-<version>-<os>-<arch>.sha256
```

This is the script form of the local sidecar build. The `Makefile` also has
`make build-sidecar`, but this script supports explicit output paths,
cross-target arguments, dry-run mode, and checksum generation.

### Install the MCP STDIO sidecar

Use this when your MCP client needs to spawn `leafwiki-mcp-stdio` from a stable
path.

```bash
./scripts/install-mcp-stdio.sh --dry-run
./scripts/install-mcp-stdio.sh
```

Default install target: `/usr/local/bin/leafwiki-mcp-stdio`.

This script calls `build-mcp-stdio.sh`, then installs the built binary. It
supports macOS and Linux install targets on `amd64` and `arm64`. It may use
`sudo` if the install directory is not writable.

### Run LeafWiki as an MCP STDIO server command

Use this when an MCP client wants a single command that starts LeafWiki and then
speaks MCP over STDIO.

```bash
./scripts/run-mcp.sh
```

Example MCP client command:

```bash
/path/to/leafwiki/scripts/run-mcp.sh --root-dir /path/to/wiki
```

The script starts `leafwiki` with MCP enabled, waits for `/api/health`, then
runs `leafwiki-mcp-stdio` against the computed `/mcp` endpoint. LeafWiki server
logs are written to a log file so stdout remains reserved for MCP JSON-RPC from
the stdio proxy.

## Script Reference

| Script | Purpose | Typical command | Notes |
| --- | --- | --- | --- |
| `install-all-macos.sh` | Build and install both local macOS executables plus `run-mcp.sh`: `leafwiki`, `leafwiki-mcp-stdio`, and the MCP wrapper. | `./scripts/install-all-macos.sh --install-dir "$HOME/.local/bin"` | Delegates to `install-macos.sh` and `install-mcp-stdio.sh`, then installs the wrapper script; use this for the normal local all-in-one install. |
| `install-macos.sh` | Build and install the main `leafwiki` executable from this checkout on macOS. | `./scripts/install-macos.sh` | Builds the UI, updates ignored frontend build output, builds the server with production embedding, and installs to `/usr/local/bin` by default. |
| `build-mcp-stdio.sh` | Build the optional `leafwiki-mcp-stdio` MCP STDIO sidecar/proxy. | `./scripts/build-mcp-stdio.sh` | Writes a release-style binary and `.sha256` under `releases/` by default. No Docker required. |
| `install-mcp-stdio.sh` | Build and install the MCP STDIO sidecar/proxy. | `./scripts/install-mcp-stdio.sh` | Calls `build-mcp-stdio.sh`, then installs `leafwiki-mcp-stdio` to `/usr/local/bin` by default. |
| `run-mcp.sh` | Start `leafwiki` with MCP enabled, then run `leafwiki-mcp-stdio` against it. | `./scripts/run-mcp.sh --root-dir ./wiki` | Intended as an MCP client command. It keeps LeafWiki logs out of stdout. |
| `changelog.sh` | Generate categorized release notes from commits between two tags. | `./scripts/changelog.sh v0.10.0 v0.11.0` | Writes `current_release_changelog.md` in the current working directory. Used by the release workflow. |
| `test-install.sh` | Validate root `install.sh` configuration handling without performing a real system install. | `./scripts/test-install.sh` | Uses fake `systemctl`/`wget` and `LEAFWIKI_INSTALL_VALIDATE_ONLY=1`. This tests the Linux installer at repo root, not the macOS installer. |
| `test-install-macos.sh` | Lightweight checks for `install-macos.sh`. | `./scripts/test-install-macos.sh` | Checks syntax, help text, and dry-run planning without installing. |
| `test-build-mcp-stdio.sh` | Lightweight checks for `build-mcp-stdio.sh`. | `./scripts/test-build-mcp-stdio.sh` | Checks syntax, help text, and dry-run planning without building. |
| `test-install-mcp-stdio.sh` | Lightweight checks for `install-mcp-stdio.sh`. | `./scripts/test-install-mcp-stdio.sh` | Checks syntax, help text, and dry-run planning without installing. |
| `test-run-mcp.sh` | Lightweight checks for `run-mcp.sh`. | `./scripts/test-run-mcp.sh` | Checks syntax, help text, dry-run planning, and stdout hygiene with fake binaries. |

## Build And Install Scripts

### `install-all-macos.sh`

Builds and installs both local macOS executables from the current checkout:
`leafwiki` and `leafwiki-mcp-stdio`. It also installs `run-mcp.sh` next to
those binaries for MCP clients that need a single STDIO command.

Useful commands:

```bash
./scripts/install-all-macos.sh --help
./scripts/install-all-macos.sh --dry-run --install-dir "$HOME/.local/bin"
./scripts/install-all-macos.sh --install-dir "$HOME/.local/bin"
./scripts/install-all-macos.sh --install-dir "$HOME/.local/bin" --skip-npm-ci
```

Important side effects:

- Runs the main app install flow from `install-macos.sh`.
- Runs the sidecar install flow from `install-mcp-stdio.sh`.
- Installs both binaries into the selected install directory.
- May use `sudo` if the install directory is not writable.

Use `--build-dir` to send both build outputs to the same directory, or
`--server-build-dir` and `--mcp-build-dir` when you need to split them.

### `install-macos.sh`

Builds the main LeafWiki application for macOS and installs it as `leafwiki`.
This is for local macOS installs from the current checkout. It is separate from
the root-level `install.sh`, which downloads published Linux release binaries
and installs a `systemd` service.

Useful commands:

```bash
./scripts/install-macos.sh --help
./scripts/install-macos.sh --dry-run
./scripts/install-macos.sh --install-dir "$HOME/.local/bin"
./scripts/install-macos.sh --skip-npm-ci
```

Important side effects:

- Runs `npm ci --ignore-scripts` unless `--skip-npm-ci` is set.
- Runs the production Vite build for `ui/leafwiki-ui`.
- Copies `ui/leafwiki-ui/dist` into `internal/http/dist`.
- Writes the built binary under `releases/` by default.
- Installs `leafwiki` into the selected install directory.

### `build-mcp-stdio.sh`

Builds the optional MCP STDIO sidecar/proxy. Use this when you want a binary
artifact but do not want to install it.

Useful commands:

```bash
./scripts/build-mcp-stdio.sh --help
./scripts/build-mcp-stdio.sh --dry-run
./scripts/build-mcp-stdio.sh --os darwin --arch arm64
./scripts/build-mcp-stdio.sh --output /tmp/leafwiki-mcp-stdio --no-checksum
```

Supported release targets match the current release matrix:

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`

The script sets `CGO_ENABLED=0`, uses `go build -trimpath -ldflags="-s -w"`,
and writes a `.sha256` file unless `--no-checksum` is passed.

### `install-mcp-stdio.sh`

Builds and installs the optional MCP STDIO sidecar/proxy. Use this when an MCP
client needs a stable executable path such as `/usr/local/bin/leafwiki-mcp-stdio`.

Useful commands:

```bash
./scripts/install-mcp-stdio.sh --help
./scripts/install-mcp-stdio.sh --dry-run
./scripts/install-mcp-stdio.sh --install-dir "$HOME/.local/bin"
./scripts/install-mcp-stdio.sh --os linux --arch amd64
```

Install targets are limited to macOS and Linux:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`

For Windows, use `build-mcp-stdio.sh` to create the `.exe` artifact instead of
using this install script.

### `run-mcp.sh`

Starts a LeafWiki server process with MCP enabled, waits for readiness, then
runs `leafwiki-mcp-stdio` connected to that server. This is the script to use as
the command in MCP clients that only support spawning a STDIO process.

Default server command shape:

```bash
leafwiki \
  --enable-mcp \
  --host 127.0.0.1 \
  --port 8080 \
  --data-dir ./data \
  --root-dir ./wiki \
  --jwt-secret p4lyOlQU643BRUc2HBiCrr55L6ygh4pJlVQ8z5LEnfT \
  --admin-password admin \
  --allow-insecure \
  --disable-request-log
```

Default stdio proxy command shape:

```bash
leafwiki-mcp-stdio --endpoint http://127.0.0.1:8080/mcp
```

Useful commands:

```bash
./scripts/run-mcp.sh --help
./scripts/run-mcp.sh --dry-run
./scripts/run-mcp.sh --root-dir "$PWD/wiki"
./scripts/run-mcp.sh --api-key "lwk_<id>_<secret>"
./scripts/run-mcp.sh --leafwiki-bin "$HOME/.local/bin/leafwiki" --mcp-stdio-bin "$HOME/.local/bin/leafwiki-mcp-stdio"
```

Important behavior:

- All wrapper diagnostics go to stderr.
- LeafWiki server stdout/stderr are redirected to `--server-log`.
- The MCP client's stdin/stdout are inherited by `leafwiki-mcp-stdio`.
- When `leafwiki-mcp-stdio` exits, the wrapper stops the LeafWiki server.
- When the wrapper receives `SIGINT` or `SIGTERM`, it stops
  `leafwiki-mcp-stdio` first, then stops the LeafWiki server it started.
- `SIGKILL` cannot be trapped, so no shell wrapper can clean up children after
  `kill -9`.
- `--server-arg` and `--stdio-arg` can be repeated for flags not modeled by the
  wrapper.

## Release Helper

### `changelog.sh`

Generates a categorized changelog from commit subjects between two tags.

```bash
./scripts/changelog.sh <previous_tag> <current_tag>
```

Example:

```bash
./scripts/changelog.sh v0.10.0 v0.11.0
```

The script expects commit subjects to roughly follow Conventional Commits
prefixes such as `feat:`, `fix:`, `docs:`, `refactor:`, `test:`, and `chore:`.
It writes `current_release_changelog.md` in the current working directory.

## Validation Scripts

The `test-*.sh` scripts are lightweight checks for script behavior. They are not
full integration tests for the application.

Run all script checks:

```bash
./scripts/test-install.sh
./scripts/test-install-macos.sh
./scripts/test-build-mcp-stdio.sh
./scripts/test-install-mcp-stdio.sh
./scripts/test-install-all-macos.sh
./scripts/test-run-mcp.sh
```

What they cover:

- Bash syntax with `bash -n`.
- Expected `--help` options for newer scripts.
- Dry-run behavior that must not create install targets.
- Root `install.sh` validation-only behavior for Linux install config.

The MCP STDIO sidecar itself is covered by Go tests in:

```bash
go test ./cmd/leafwiki-mcp-stdio ./internal/wiki/mcpstdio
```

## Environment Overrides

The build/install scripts can be configured with flags or environment variables.
Flags should be preferred in one-off commands because they are visible in shell
history and CI logs.

| Script | Environment variables |
| --- | --- |
| `install-all-macos.sh` | `LEAFWIKI_INSTALL_DIR`, `LEAFWIKI_BUILD_DIR`, `LEAFWIKI_VERSION`, `LEAFWIKI_ARCH`, `LEAFWIKI_MCP_STDIO_INSTALL_DIR`, `LEAFWIKI_MCP_STDIO_BUILD_DIR`, `LEAFWIKI_MCP_STDIO_VERSION`, `LEAFWIKI_MCP_STDIO_GOARCH`, `GOARCH` |
| `install-macos.sh` | `LEAFWIKI_INSTALL_DIR`, `LEAFWIKI_BUILD_DIR`, `LEAFWIKI_VERSION`, `LEAFWIKI_ARCH` |
| `build-mcp-stdio.sh` | `LEAFWIKI_MCP_STDIO_BUILD_DIR`, `LEAFWIKI_MCP_STDIO_VERSION`, `LEAFWIKI_MCP_STDIO_GOOS`, `LEAFWIKI_MCP_STDIO_GOARCH`, `LEAFWIKI_VERSION`, `GOOS`, `GOARCH` |
| `install-mcp-stdio.sh` | `LEAFWIKI_MCP_STDIO_INSTALL_DIR`, `LEAFWIKI_MCP_STDIO_BUILD_DIR`, `LEAFWIKI_MCP_STDIO_VERSION`, `LEAFWIKI_MCP_STDIO_GOOS`, `LEAFWIKI_MCP_STDIO_GOARCH`, `LEAFWIKI_VERSION`, `GOOS`, `GOARCH` |
| `run-mcp.sh` | `LEAFWIKI_RUN_MCP_LEAFWIKI_BIN`, `LEAFWIKI_RUN_MCP_STDIO_BIN`, `LEAFWIKI_RUN_MCP_HOST`, `LEAFWIKI_RUN_MCP_PORT`, `LEAFWIKI_RUN_MCP_BASE_PATH`, `LEAFWIKI_RUN_MCP_ROOT_DIR`, `LEAFWIKI_RUN_MCP_DATA_DIR`, `LEAFWIKI_RUN_MCP_JWT_SECRET`, `LEAFWIKI_RUN_MCP_ADMIN_PASSWORD`, `LEAFWIKI_RUN_MCP_ENDPOINT`, `LEAFWIKI_RUN_MCP_API_KEY`, `LEAFWIKI_RUN_MCP_SERVER_LOG`, plus LeafWiki and sidecar fallback vars listed by `./scripts/run-mcp.sh --help` |

## Notes

- Prefer `--dry-run` before installing into `/usr/local/bin`.
- The macOS main-app installer is local-build oriented. It does not install a
  launch service.
- The root-level `install.sh` is Linux/systemd oriented and downloads published
  release binaries. It is not the same workflow as `scripts/install-macos.sh`.
- The MCP STDIO sidecar is a client-side bridge for MCP clients that spawn a
  local STDIO process. It does not start or manage the LeafWiki server.
