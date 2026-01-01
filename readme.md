# 🌿 LeafWiki

**A self-hosted Dev Wiki built for developers — tree-based, fast, and powered by plain Markdown.**  

LeafWiki provides a personal, self-hosted documentation space — storing pages as plain Markdown files on your own infrastructure, without requiring an external database.

---

## Live Demo

A public demo of LeafWiki is available here:

🌐 **[demo.leafwiki.com](https://demo.leafwiki.com)**  

Login credentials are displayed on the demo site.  
The demo instance resets automatically every hour, so all changes are temporary.

---

## Preview

![LeafWiki](./preview.png)

---

**Mobile View:**

<p align="center">
  <img src="./mobile-editor.png" width="260" />
  <img src="./mobile-pageview.png" width="260" />
  <img src="./mobile-navigation.png" width="260" />
</p>

---

## What LeafWiki is good for today

LeafWiki focuses on personal and small-team documentation use cases today.  
Team features and collaboration may evolve based on real-world needs.

LeafWiki is currently well-suited for:
- Personal technical notes, documentation and ideas
- Project documentation maintained by one main contributor
- Runbooks, operational knowledge and engineering guides for small teams
- Structured content that benefit from explicit hierarchy and ordering

---

## Why Another Wiki?

Many existing wiki tools feel heavier than the problem they solve.  
They require databases, plugins, workflows or complex deployment setups.  

That adds friction before writing has even started.  
You have to pick structure, configure storage, and think about operations instead of capturing knowledge while the context is fresh.  

LeafWiki was designed around a few simple questions:
- Why require a complex database for Markdown content?
- Why should self-hosting a wiki require significant setup effort?
- Why can’t structure and navigation be handled explicitly while keeping files portable?

The result is a lightweight wiki engine with:
- Markdown files stored directly on disk
- Explicit tree-based structure
- A single Go binary or container deployment
- Minimal operational overhead


LeafWiki intentionally prioritizes writing flow, simplicity and long-term maintainability over feature complexity.

---

## Core principles

LeafWiki is built around a small set of clear principles:

- **Plain Markdown storage**  
  All content is stored as Markdown files on disk. This avoids vendor lock-in and keeps your data portable and transparent.

- **No external database required**  
  LeafWiki uses SQLite internally and does not require running or managing a separate database service.

- **Explicit structure management**  
  Page hierarchy and ordering are managed explicitly, allowing pages to be reordered without relying on the filesystem layout alone.

- **Self-hosted by design**
  LeafWiki is designed to run on a single server with minimal operational overhead.

---

### Data model

LeafWiki stores page content as Markdown files and uses a combination of JSON and SQLite for navigation, metadata, and search.
For details on the current model and its constraints, see [Known limitations](#known-limitations).

---

## What LeafWiki supports

- Built-in Markdown editor
- Tree-based navigation for structured content
- Public read-only access
- Support for diagrams via Mermaid
- Full-text search across page titles and content
- Image and asset support
- Dark mode and mobile-friendly UI
- Separation between admin and editor users
- Keyboard shortcuts for common actions (like saving with Ctrl+S, ...)


LeafWiki runs as a single Go binary, does not require an external database, and is designed to be self-hosted using Docker or as a standalone binary.
LeafWiki supports public read-only access for documentation use cases, while keeping editing and structure management restricted to authenticated users.

## What LeafWiki is not

LeafWiki does not aim to be a large enterprise documentation system.

It intentionally avoids complex workflows, real-time collaborative editing, and advanced permission models to maintain simplicity, predictability, and low operational overhead.

---

## Project Status

LeafWiki is stable for everyday use as a personal or primary-owner wiki.  
The core feature set — writing, structure, search, ... is actively maintained and production-ready.

Collaboration is currently limited and follows a *last-write-wins* approach.  
More advanced team-oriented capabilities are under development, with a focus on durability and predictable behavior.


**Current priorities:**  
- Versioning
- Operations metadata (created/updated info)
- Conflict handling for concurrent edits (optimistic locking)

Development is iterative and guided by real-world use.  
The platform will evolve cautiously toward team workflows while maintaining its principles of simplicity and low operational overhead.

> **LeafWiki** is actively developed and open to collaboration 🌿 

See the [CHANGELOG](CHANGELOG.md) for release details.

---

## Installation

LeafWiki is distributed as a single Go binary and can be run directly on the host or via Docker.
The sections below show a recommended quick start and a few common installation examples.

### Quick start

The easiest way to install LeafWiki is using the provided installation script:

```bash
curl -sL https://raw.githubusercontent.com/perber/leafwiki/main/install.sh -o install.sh && chmod +x ./install.sh && sudo ./install.sh --arch amd64
```

This installs LeafWiki as a system service on the target machine.
The service is started automatically after installation.
> The installation script has been tested on Ubuntu.
> Feedback for other distributions is welcome via GitHub issues.

#### Deployment examples 
- [Install LeafWiki with nginx on Ubuntu](docs/install/nginx.md)
- [Install LeafWiki on a Raspberry Pi](docs/install/raspberry.md)


#### Security notes

Sensitive information such as the JWT secret and administrator password appears in plain text in the systemd service file `/etc/systemd/system/leafwiki.service`.
Make sure that this file is accessible only to authorized users.

#### Installer script options

The installation script supports a small set of flags that control how LeafWiki is installed on the target system.
These options are only used during installation and do not affect the runtime behavior of LeafWiki.

| Flag               | Description                                                 | Default       |
|--------------------|-------------------------------------------------------------|---------------|
| `--arch`           | Target architecture for the binary (e.g. `amd64`, `arm64`)  |       -       |
| `--host`           | Host/IP address the server binds to                         | `127.0.0.1`   |
| `--port`           | Port the server listens on                                  | `8080`        |


### Docker

You can run LeafWiki as a container using Docker.

```bash
docker run -p 8080:8080 \
    -v ~/leafwiki-data:/app/data \
    ghcr.io/perber/leafwiki:latest \
    --jwt-secret=yoursecret \
    --admin-password=yourpassword
```

By default, the container binds to `0.0.0.0` so the wiki is reachable from your network.
The data directory inside the container is `/app/data`.

---

**Running as non-root user**
To avoid running the container as root, specify a user ID:

```bash
docker run -p 8080:8080 \
    -u 1000:1000 \
    -v ~/leafwiki-data:/app/data \
    ghcr.io/perber/leafwiki:latest \
    --jwt-secret=yoursecret \
    --admin-password=yourpassword
```

Make sure that the mounted data directory is writable by the specified user.

The data directory inside the container will be `/app/data`..

---

### Manual installation

Download the latest release binary from GitHub, make it executable, and start the server:

```
chmod +x leafwiki
./leafwiki --jwt-secret=yoursecret --admin-password=yourpassword
```

**Note:** By default, the server listens on `127.0.0.1`, which means it will only be accessible from localhost. If you want to access the server from other machines on your network, add `--host=0.0.0.0` to the command:

```
./leafwiki --jwt-secret=yoursecret --admin-password=yourpassword --host=0.0.0.0
```

Default port is `8080`, and the default data directory is `./data`.
You can change the data directory with the `--data-dir` flag.

The JWT secret is required for authentication and should be kept secure.


## Authentication and admin user

### Reset Admin Password
If you need to reset the admin password, you can do so by running:

```bash
./leafwiki reset-admin-password
```

## Runtime Configuration

LeafWiki can be configured using command-line flags or environment variables.
These options control how the server runs after installation.

### CLI Flags

| Flag                            | Description                                                            | Default       |
|---------------------------------|------------------------------------------------------------------------|---------------|
| `--jwt-secret`                  | Secret used for signing JWTs (required)                                | –             |
| `--host`                        | Host/IP address the server binds to                                    | `127.0.0.1`   |
| `--port`                        | Port the server listens on                                             | `8080`        |
| `--data-dir`                    | Directory where data is stored                                         | `./data`      |
| `--admin-password`              | Initial admin password *(used only if no admin exists)* (required)      | –             |
| `--public-access`               | Allow public read-only access                                          | `false`       |
| `--hide-link-metadata-section`  | Hide link metadata section                                             | `false`       |

> When using the official Docker image, `LEAFWIKI_HOST` defaults to `0.0.0.0` if neither a `--host` flag nor `LEAFWIKI_HOST` is provided, as the container entrypoint sets this automatically.

### Environment Variables

The same configuration options can also be provided via environment variables.
This is especially useful in containerized or production environments.

| Variable                               | Description                                                             | Default    |
|----------------------------------------|-------------------------------------------------------------------------|------------|
| `LEAFWIKI_HOST`                        | Host/IP address the server binds to                                     | `127.0.0.1`|
| `LEAFWIKI_PORT`                        | Port the server listens on                                              | `8080`     |
| `LEAFWIKI_DATA_DIR`                    | Path to the data storage directory                                      | `./data`   |
| `LEAFWIKI_ADMIN_PASSWORD`              | Initial admin password *(used only if no admin exists yet)* (required)  | –          |
| `LEAFWIKI_JWT_SECRET`                  | Secret used to sign JWT tokens *(required)*                             | –          |
| `LEAFWIKI_PUBLIC_ACCESS`               | Allow public read-only access                                           | `false`    |
| `LEAFWIKI_HIDE_LINK_METADATA_SECTION`  | Hide link metadata section                                              | `false`    |

These environment variables override the default values and are especially useful in containerized or production environments.

> When using the official Docker image, `LEAFWIKI_HOST` defaults to `0.0.0.0` if neither a `--host` flag nor `LEAFWIKI_HOST` is provided, as the container entrypoint sets this automatically.

## Migrating existing Markdown content (community tool)

If you already have an existing folder of Markdown files, you can bootstrap a LeafWiki navigation tree using a small community-contributed helper script.


> **Community Tool**  
> Useful for **initial migration** from existing Markdown structures into LeafWiki.  
> Optional, provided “as is”. Contributions are welcome

The script:
- scans a Markdown directory (default: `data/root`)
- normalizes folder/file names to LeafWiki conventions (lowercase, hyphens)
- ensures every folder has an `index.md`
- generates a `data/tree.json` so LeafWiki can navigate the structure

Location: `tools/generate-tree.py`

### preview changes (dry-run mode)
You can preview the changes without modifying any files using the `--preview` flag:

```bash
python3 tools/generate-tree.py --root data/root --preview
```

This will:
- show proposed renames, skipped files and added index.md files
- print a preview of the generated tree
- perform no changes.

### Generate tree.json and normalize Markdown structure
To actually apply the changes and generate `tree.json`, run:

```bash
python3 tools/generate-tree.py --root data/root --output data/tree.json
```

**Notes:**
- It will rename files/folders to enforce LeafWiki naming conventions (lowercase, hyphens)
- It will create blank `index.md` files in folders that lack them.
- This is intended as a one-time import step
- For ongoing structure management, use LeafWiki's UI.

> Contributed in [#523](https://github.com/perber/leafwiki/issues/523) - thanks to @nicgord

---

## Quick Start (Dev)

```
# 1. Clone the repo

git clone https://github.com/perber/leafwiki.git
cd leafwiki

# 2. Install frontend dependencies

cd ui/leafwiki-ui
npm install
npm run dev   # Starts Vite dev server on http://localhost:5173

# 3. In another terminal, start the backend

cd ../../cmd/leafwiki
go run main.go

# Note: The backend binds to 127.0.0.1 by default for security.
# If you need to access it from a different machine or network interface
# (e.g., testing on mobile or from another device), use:
# go run main.go --host=0.0.0.0
```

---

### Keyboard Shortcuts

| Action                     | Shortcut                                   |
|----------------------------|--------------------------------------------|
| Switch to Edit Mode        | `Ctrl + E` (or `Cmd + E`)                  |
| Switch to Search Pane      | `Ctrl + Shift + F` (or `Cmd + Shift + F`)  |
| Switch to Navigation Pane  | `Ctrl + Shift + E` (or `Cmd + Shift + E`)  |
| Save Page                  | `Ctrl + S` (or `Cmd + S`)                  |
| Bold Text                  | `Ctrl + B` (or `Cmd + B`)                  |
| Italic Text                | `Ctrl + I` (or `Cmd + I`)                  |
| Headline 1                 | `Ctrl + Alt + 1` (or `Cmd + Alt + 1`)      |
| Headline 2                 | `Ctrl + Alt + 2` (or `Cmd + Alt + 2`)      |
| Headline 3                 | `Ctrl + Alt + 3` (or `Cmd + Alt + 3`)      |

`Ctrl+V` / `Cmd+V` for pasting images or files is also supported in the editor.
`Esc` can be used to exit modals, dialogs or the edit mode.

More shortcuts may be added in future releases.

---

## Known limitations

LeafWiki focuses on simplicity with a well-defined scope.  
As a result, the following limitations apply today:

- **No built-in page history or versioning**  
  Saving changes overwrites the previous state. Versioning is a planned feature.

- **Basic concurrency handling**  
  Edits follow a last-write-wins model. Best suited for single maintainers or low-concurrency use.

- **Metadata not fully embedded in Markdown**  
  Page content is plain Markdown, but structure, metadata, user accounts, and search indexes are stored in SQLite.

- **Minimal access control**  
  No role-based permissions or fine-grained restrictions at this time.

---

### Available Builds

LeafWiki is available as a native binary for the following platforms:

- **Linux (x86_64 and ARM64)**
- **macOS (x86_64 and ARM64)**
- **Windows (x86_64)**
- **Raspberry Pi (tested with 64-bit OS)**

---

## Contributing

Contributions, discussions, and feedback are very welcome.  
If you have ideas, questions, or run into issues, feel free to open an issue or start a discussion.

## Stay in the loop

Follow the repository to get updates about new releases and ongoing development.

