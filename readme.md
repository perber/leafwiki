# 🌿 LeafWiki

**LeafWiki helps engineers, self-hosters, and small teams keep long-lived documentation structured, portable, and easy to operate.**  
Self-hosted. Single Go binary. SQLite-based. Markdown stored on disk.

If you want something lighter than a large wiki suite, but more structured than scattered notes, LeafWiki is built for that middle ground.

LeafWiki is a real wiki application built around Markdown, not a plain Markdown file browser. It provides structured navigation, editing, search, roles, and managed content workflows inside the app.

![LeafWiki](./assets/preview.png)

## Why it exists

LeafWiki is for documentation that needs to stay understandable over time: runbooks, internal docs, personal knowledge bases, and operational notes.

The goal is not to become an all-in-one workspace. The goal is to give you a wiki that is small enough to operate comfortably and structured enough to trust for long-lived documentation.

## Quick facts

- Single Go binary
- SQLite-based runtime storage
- Markdown-first editing and content portability
- Works with Docker or direct binary install
- Multi-platform builds for Linux, macOS, Windows, and ARM64
- Reverse-proxy friendly with `--base-path`
- Public read-only mode available
- Optional revision history and link refactoring behind feature flags

## Why it fits this workflow

- Explicit tree navigation instead of flat note feeds
- Markdown content that is easy to back up, move, and version
- Public read-only docs with authenticated editing
- Optimistic locking for concurrent edits
- Optional revision history and safe link refactoring
- Small operational footprint without external database setup

## Good fit

- Personal wikis and engineering notebooks
- Internal team documentation
- Runbooks, SOPs, and operational guides
- Homelab and self-hosted environments
- Teams that prefer explicit tree navigation over flat note feeds

## Probably not a fit

- Large organizations needing complex enterprise permissions or workflow engines
- Real-time collaborative editing
- Knowledge management setups that expect databases, automations, and approval flows everywhere
- Teams looking for a Confluence or Notion clone

LeafWiki is intentionally narrower than those systems. That focus is part of the value.

---

## Live Demo

A public demo of LeafWiki is available here:

🌐 **[demo.leafwiki.com](https://demo.leafwiki.com)**  

Try: `Ctrl+E` to edit, `Ctrl+S` to save, `Ctrl+Shift+F` to open the search.  

Login credentials are displayed on the demo site.  
The demo instance resets automatically every hour, so all changes are temporary.

---

**Mobile View:**

Mobile-friendly UI for reading (and editing) docs & runbooks on the go.

<p align="center">
  <img src="./assets/mobile-editor.png" width="260" />
  <img src="./assets/mobile-pageview.png" width="260" />
  <img src="./assets/mobile-navigation.png" width="260" />
</p>

---

## Keep internal links intact

Renaming or moving pages often breaks internal documentation.

LeafWiki can rewrite internal links when:
- a page is renamed
- a page is moved in the tree

This helps keep long-lived documentation coherent as the structure changes.

> Revision history and link refactoring are currently available behind feature flags: `--enable-revision` and `--enable-link-refactor`.

---

## Import existing Markdown

LeafWiki includes a built-in Markdown importer for editors and admins.

What to expect:
- ZIP-based import workflow
- review the generated import plan before running it
- best results when the source already has a reasonably clean folder structure
- linked Markdown pages and local assets can be imported together
- Obsidian-style wiki links can be rewritten during import

It is intended as a pragmatic migration helper, not a fully automatic migration system for every source format.

---

## What LeafWiki supports

- Fast writing flow with keyboard shortcuts
- Built-in Markdown editor with live preview
- Full-text search across page titles and content
- Images, files, Mermaid diagrams, and practical Markdown extensions
- Admin, editor, and viewer roles
- Branding options such as logo, favicon, and site name
- Dark mode and mobile-friendly UI

Revision history and link refactoring are currently available behind feature flags: `--enable-revision` and `--enable-link-refactor`.

LeafWiki's editor and live preview support standard Markdown plus practical documentation features such as tables, task lists, footnotes, shoutouts, Mermaid diagrams, embedded audio/video, and a sanitized subset of inline HTML.

## What LeafWiki is not

- Not a full Confluence replacement
- Not real-time collaborative editing
- Not a workflow, approval, or document-control platform
- Not a database-heavy documentation stack

LeafWiki is designed to stay focused, predictable, and easy to operate.

---

## Installation

LeafWiki is distributed as a single Go binary and can be run directly on the host or via Docker.
Start with the quickest path below. The more detailed deployment and configuration options follow after that.

### Quick start

If you already run self-hosted apps with containers, start here:

```bash
docker run -p 8080:8080 \
    -v ~/leafwiki-data:/app/data \
    ghcr.io/perber/leafwiki:latest \
    --jwt-secret=yoursecret \
    --admin-password=yourpassword \
    --allow-insecure=true
```

The container stores data in `/app/data` and binds to `0.0.0.0` by default.
For plain HTTP setups, `--allow-insecure=true` is required for login cookies to work.
If you serve LeafWiki behind HTTPS or a reverse proxy that forwards HTTPS headers, omit `--allow-insecure=true`.

### Other deployment options

- Use the installer if you want a system service on a Linux host
- Use Docker Compose if you prefer a compose-based setup
- Use the binary if you want the smallest and most direct install

**Running as non-root user**
To avoid running the container as root, specify a user ID:

```bash
docker run -p 8080:8080 \
    -u 1000:1000 \
    -v ~/leafwiki-data:/app/data \
    ghcr.io/perber/leafwiki:latest \
    --jwt-secret=yoursecret \
    --admin-password=yourpassword \
    --allow-insecure=true
```

Make sure that the mounted data directory is writable by the specified user.

### Quick start with the installer

The easiest way to install LeafWiki is using the provided installation script:

```bash
sudo /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/perber/leafwiki/main/install.sh)"
```

This installs LeafWiki as a system service on the target machine.
The service is started automatically after installation.
> The installation script has been tested on Ubuntu.
> Feedback for other distributions is welcome via GitHub issues.

#### Deployment examples
- [Install LeafWiki with nginx on Ubuntu](docs/install/nginx.md)
- [Install LeafWiki on a Raspberry Pi](docs/install/raspberry.md)


#### Security notes
In interactive mode, environment variables appear in plain text in file `/etc/leafwiki/.env`.
Make sure that this file is accessible only to authorized users.

#### Installer script options

**Non-interactive mode**

The script supports non-interactive mode for automated deployments. Use the `--non-interactive` flag and provide configuration via an `.env` file.

An `.env.example` file is included showing all available environment variables. Copy and customize it as needed:

```bash
cp .env.example .env
# Edit .env with your configuration
sudo ./install.sh --non-interactive --env-file ./.env
```

---

### Docker Compose

```yaml
services:
  leafwiki:
    image: ghcr.io/perber/leafwiki:latest
    container_name: leafwiki
    user: 1000:1000  # Run as non-root (specify your {UID}:{GID})
    ports:
      - "8080:8080"
    environment:
      - LEAFWIKI_JWT_SECRET=yourSecret
      - LEAFWIKI_ADMIN_PASSWORD=yourPassword
      - LEAFWIKI_ALLOW_INSECURE=true  # WARNING: Enables HTTP by disabling Secure/HttpOnly cookies; for HTTPS deployments, omit this variable. HTTPS is the preferred method.
    volumes: 
      - ~/leafwiki-data:/app/data
    restart: unless-stopped
```

Make sure the mounted data directory (`~/leafwiki-data`) is writable by the user specified in the `user` field.

### Quick start with a binary

Download the latest release binary from GitHub, make it executable, and start the server:

```bash
chmod +x leafwiki
./leafwiki --jwt-secret=yoursecret --admin-password=yourpassword --allow-insecure=true
```

**Note:** By default, the server listens on `127.0.0.1`, which means it will only be accessible from localhost. If you want to access the server from other machines on your network, add `--host=0.0.0.0` to the command:

```bash
./leafwiki --jwt-secret=yoursecret --admin-password=yourpassword --host=0.0.0.0 --allow-insecure=true
```

Default port is `8080`, and the default data directory is `./data`.
You can change the data directory with the `--data-dir` flag.

The JWT secret is required for authentication and should be kept secure.
For plain HTTP setups such as localhost testing or direct LAN access, `--allow-insecure=true` is required so the browser accepts login and CSRF cookies.
When LeafWiki is served over HTTPS, leave `--allow-insecure` disabled.

### Operations notes

- Default bind address is `127.0.0.1` unless you set `--host` or use the official Docker image
- Default data directory is `./data` on direct binary installs and `/app/data` in the container
- `--public-access` allows public read-only access while keeping editing authenticated
- `--disable-auth` is only appropriate for trusted internal networks or local development

These defaults are intentionally conservative so a fresh install does not become network-exposed by accident.


## Authentication and admin user

### Reset Admin Password
If you need to reset the admin password:

```bash
./leafwiki reset-admin-password
```

## Runtime Configuration

LeafWiki can be configured using command-line flags or environment variables.
These options control how the server runs after installation.

If you are just getting started, the most important options are usually:

- `--jwt-secret`
- `--admin-password`
- `--host`
- `--data-dir`
- `--public-access`
- `--base-path`
- `--allow-insecure`

### CLI Flags

| Flag                            | Description                                                            | Default       | Available since   |
|---------------------------------|------------------------------------------------------------------------|---------------|-------------------|
| `--jwt-secret`                  | Secret used for signing JWTs (required)                                | –             | –                 |
| `--host`                        | Host/IP address the server binds to                                    | `127.0.0.1`   | –                 |
| `--port`                        | Port the server listens on                                             | `8080`        | –                 |
| `--data-dir`                    | Directory where data is stored                                         | `./data`      | –                 |
| `--admin-password`              | Initial admin password *(used only if no admin exists)* (required)     | –             | –                 |
| `--public-access`               | Allow public read-only access                                          | `false`       | –                 |
| `--hide-link-metadata-section`  | Hide link metadata section                                             | `false`       | –                 |
| `--inject-code-in-header`       | Raw HTML/JS code injected into <head> tag (e.g., analytics, custom CSS)| `""`          | v0.6.0            |
| `--custom-stylesheet`           | Path to a `.css` file inside the data dir, served publicly as `/custom.css` or `${base-path}/custom.css` | `""`          | v0.8.5      |
| `--allow-insecure`              | ⚠️ Allows insecure HTTP usage for auth cookies (required for plain HTTP) | `false`       | v0.7.0            |
| `--access-token-timeout`        | Access token timeout duration (e.g. 24h, 15m)                          | `15m`         | v0.7.0            |
| `--refresh-token-timeout`       | Refresh token timeout duration (e.g. 168h, 7d)                         | `7d`          | v0.7.0            |
| `--disable-auth`                | ⚠️ Disable authentication & authorization (internal networks only!)    | `false`       | v0.7.0            |
| `--base-path`                   | URL prefix when served behind a reverse proxy (e.g. /wiki)              | `""`        | v0.8.2            |
| `--max-asset-upload-size`       | Maximum size for asset uploads (e.g. `50MiB`, `50MB`, `52428800`)      | `50MiB`       | v0.8.5            |
| `--enable-revision`             | Enable revision history / page history                                  | `false`       | v0.9.0            |
| `--enable-link-refactor`        | Enable link refactoring dialog and rewrite flow                         | `false`       | v0.9.0            |
| `--max-revision-history`        | Maximum revisions kept per page; `0` means unlimited                    | `100`         | v0.9.0            |


> When using the official Docker image, `LEAFWIKI_HOST` defaults to `0.0.0.0` if neither a `--host` flag nor `LEAFWIKI_HOST` is provided, as the container entrypoint sets this automatically.

### Environment Variables

The same configuration options can also be provided via environment variables.
This is especially useful in containerized or production environments.

| Variable                               | Description                                                             | Default    | Available since |
|----------------------------------------|-------------------------------------------------------------------------|------------|-----------------|
| `LEAFWIKI_HOST`                        | Host/IP address the server binds to                                     | `127.0.0.1`| -               |
| `LEAFWIKI_PORT`                        | Port the server listens on                                              | `8080`     | -               |
| `LEAFWIKI_DATA_DIR`                    | Path to the data storage directory                                      | `./data`   | -               |
| `LEAFWIKI_ADMIN_PASSWORD`              | Initial admin password *(used only if no admin exists yet)* (required)  | –          | -               |
| `LEAFWIKI_JWT_SECRET`                  | Secret used to sign JWT tokens *(required)*                             | –          | -               |
| `LEAFWIKI_PUBLIC_ACCESS`               | Allow public read-only access                                           | `false`    | -               |
| `LEAFWIKI_HIDE_LINK_METADATA_SECTION`  | Hide link metadata section                                              | `false`    | -               |
| `LEAFWIKI_INJECT_CODE_IN_HEADER`       | Raw HTML/JS code injected into <head> tag (e.g., analytics, custom CSS) | `""`       | v0.6.0          |
| `LEAFWIKI_CUSTOM_STYLESHEET`           | Path to a `.css` file inside the data dir, served publicly as `/custom.css` or `${LEAFWIKI_BASE_PATH}/custom.css` | `""`       | v0.8.5   |
| `LEAFWIKI_ALLOW_INSECURE`              | ⚠️ Allows insecure HTTP usage for auth cookies (required for plain HTTP) | `false`    | v0.7.0          |
| `LEAFWIKI_ACCESS_TOKEN_TIMEOUT`        | Access token timeout duration (e.g. 24h, 15m)                           | `15m`      | v0.7.0          |
| `LEAFWIKI_REFRESH_TOKEN_TIMEOUT`       | Refresh token timeout duration (e.g. 168h, 7d)                          | `7d`       | v0.7.0          |
| `LEAFWIKI_DISABLE_AUTH`                | ⚠️ Disable authentication & authorization (internal networks only!)     | `false`    | v0.7.0          |
| `LEAFWIKI_BASE_PATH`                   | URL prefix when served behind a reverse proxy (e.g. /wiki)              | `""`       | v0.8.2          |
| `LEAFWIKI_MAX_ASSET_UPLOAD_SIZE`       | Maximum size for asset uploads (e.g. `50MiB`, `50MB`, `52428800`)       | `50MiB`    | v0.8.5          |
| `LEAFWIKI_ENABLE_REVISION`             | Enable revision history / page history                                  | `false`    | v0.9.0          |
| `LEAFWIKI_ENABLE_LINK_REFACTOR`        | Enable link refactoring dialog and rewrite flow                         | `false`    | v0.9.0          |
| `LEAFWIKI_MAX_REVISION_HISTORY`        | Maximum revisions kept per page; `0` means unlimited                    | `100`      | v0.9.0          |


These environment variables override the default values and are especially useful in containerized or production environments.

> When using the official Docker image, `LEAFWIKI_HOST` defaults to `0.0.0.0` if neither a `--host` flag nor `LEAFWIKI_HOST` is provided, as the container entrypoint sets this automatically.

### Custom Stylesheet

The custom stylesheet feature is available since `v0.8.5`.

To use it, place a `.css` file inside your configured data directory and pass its path via `--custom-stylesheet` or `LEAFWIKI_CUSTOM_STYLESHEET`.

Example:

```bash
./leafwiki \
  --data-dir=./data \
  --custom-stylesheet=custom.css \
  --jwt-secret=yoursecret \
  --admin-password=yourpassword
```

With the example above:

- The file must exist at `./data/custom.css`
- Without a base path, it is served as `/custom.css`
- With `--base-path=/wiki`, it is served as `/wiki/custom.css`
- The stylesheet endpoint is publicly accessible

### Security Overview - Since v0.7.0

LeafWiki includes several built-in security mechanisms enabled by default:

- **Secure, HttpOnly cookies** for session handling
- **Session-based authentication** backed by a local database
- **CSRF protection** for all state-changing requests
- **Rate limiting** on authentication-related endpoints
- **Role-based access** (admin, editor, viewer)

These defaults are intended to be safe for normal deployments. If you weaken them with `--disable-auth` or `--allow-insecure`, do so only in trusted environments.

---

### Security warning: `--disable-auth`

`--disable-auth` completely disables authentication and authorization.

Only use it for:
- local development
- trusted internal networks
- isolated environments protected elsewhere

Do not use it on public or internet-facing deployments.

Safe local example:

```bash
./leafwiki --disable-auth --host=127.0.0.1
```

For most setups, prefer:
- Authentication enabled (default)
- `--public-access` for read-only public access
- Viewer role for read-only access

---

## Quick Start (Dev)

```bash
git clone https://github.com/perber/leafwiki.git
cd leafwiki

cd ui/leafwiki-ui
npm install
npm run dev

cd ../../cmd/leafwiki
go run main.go --jwt-secret=yoursecret --allow-insecure=true --admin-password=yourpassword
```

Vite starts on `http://localhost:5173`.
The backend binds to `127.0.0.1` by default. Use `--host=0.0.0.0` only if you intentionally need network access.

---

### Keyboard Shortcuts

| Action                     | Shortcut                                   |
|----------------------------|--------------------------------------------|
| Switch to Edit Mode        | `Ctrl + E` (or `Cmd + E`)                  |
| Save Page                  | `Ctrl + S` (or `Cmd + S`)                  |
| Switch to Search Pane      | `Ctrl + Shift + F` (or `Cmd + Shift + F`)  |
| Switch to Navigation Pane  | `Ctrl + Shift + E` (or `Cmd + Shift + E`)  |
| Go to Page                 | `Ctrl + Alt + P` (or `Cmd + Option + P`)   |
| Bold Text                  | `Ctrl + B` (or `Cmd + B`)                  |
| Italic Text                | `Ctrl + I` (or `Cmd + I`)                  |
| Headline 1                 | `Ctrl + Alt + 1` (or `Cmd + Alt + 1`)      |
| Headline 2                 | `Ctrl + Alt + 2` (or `Cmd + Alt + 2`)      |
| Headline 3                 | `Ctrl + Alt + 3` (or `Cmd + Alt + 3`)      |

`Ctrl+V` / `Cmd+V` for pasting images or files is also supported in the editor.
`Esc` can be used to exit modals, dialogs or the edit mode.

More shortcuts may be added in future releases.

---

### Available Builds

LeafWiki is available as a native binary for the following platforms:

- **Linux (x86_64 and ARM64)**
- **macOS (x86_64 and ARM64)**
- **Windows (x86_64)**
- **Raspberry Pi (tested with 64-bit OS)**

---

## Support LeafWiki

If LeafWiki is useful to you or your team, consider sponsoring the project on GitHub.

Sponsorship helps fund maintenance, bug fixes, documentation, and improvements like tags and properties.

👉 https://github.com/sponsors/perber

---

## Contributing

Contributions, discussions, and feedback are very welcome.  
If you have ideas, questions, or run into issues, feel free to open an issue or start a discussion.

## Stay in the loop

Follow the repository to get updates about new releases and ongoing development.
