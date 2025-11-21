# 🌿 LeafWiki

**A lightweight, tree-based wiki that keeps your content in plain Markdown - fast, clean, and self-hosted**  

> *Run your knowledge base in 5 minutes - no database, just a single Go binary.*

LeafWiki is designed for teams and individuals who want a clean, fast, and self-hosted knowledge base - with full control over structure, content, and deployment.

---

![LeafWiki](./preview.gif)

---

## 🌿 Growing with Purpose

LeafWiki has reached a **stable and usable foundation**, and it’s already being used by developers to organize and document their knowledge.  

The project isn’t *feature-complete yet*.  
Right now, the focus is on **refining what’s there**, improving **stability**, and evolving the platform carefully toward a truly developer-friendly knowledge base.

**Current priorities:**  
- Stability and reliability
- UX/UI and usability improvements
- Testing, CI, and quality assurance

The editor and admin features are already in place, but collaboration is still basic - currently following a “last-write-wins” approach.
Team-ready features like versioning, ... are planned for future iterations.

LeafWiki and of course I are looking for **contributors** who want to help bring this vision to life - by refining existing features, fixing bugs, or experimenting with new ideas. 

> **LeafWiki** is stable, growing and open for collaboration 🌿 

## 🧪 Live Demo

> **Test it yourself – no setup required!**  
> A public demo of LeafWiki is available here:  

🌐 **[demo.leafwiki.com](https://demo.leafwiki.com)**  

> Login credentials are displayed on the demo site's login page.  
> ⚙️ The demo instance resets automatically every hour,   
> so feel free to experiment - all changes are temporary.  

---

## ✨ Features

- 🧾 Markdown-first with live editor + preview
- 🌲 True tree-structured pages (nested folders)
- 🔒 Role-based access (admin / editor)
- 🧠 No database required
- 📂 Per-page assets with upload support
- 🖼️ Embed images and files via Markdown or paste-to-upload (CTRL+V)
- ⚙️ Single statically-linked Go binary (no dependencies)
- 🚀 Easily self-hosted (Docker or standalone)
- 🔁 Session auth with JWT tokens + refresh
- 🔍 Search functionality for page titles and content
- 📱 Mobile-friendly design
- 🌐 Public pages (viewable without login)
- 🖨️ Print view support
- 🧩 Mermaid.js diagram support
- 🎨 Resizable sidebar

---

### Available Builds

LeafWiki now builds and runs natively on:
- **Linux (x86_64 and ARM64)**
- **Windows (x86_64)**
- **Raspberry Pi (tested with 64-bit OS)**

---

## 💭 Why Another Wiki?

After trying out tools like Wiki.js, Confluence, and DokuWiki, I wanted something simpler: no database, easy to host, Markdown-based, and truly Git-friendly.

- Why use a database just to store Markdown?
- Why should setup be a weekend project?
- Why can't a wiki just be file-based and fast?

**LeafWiki** was born out of that frustration - and the desire to have:

- 🧾 Clean Markdown files, organized in folders
- 🧠 A real tree structure, not a flat list
- ⚙️ A single binary with no external dependencies
- 🛠️ Something teams can actually self-host without DevOps pain

It’s not trying to be everything - just a solid, minimal wiki for people who want **clarity over complexity.**

---

## 💡 LeafWiki vs. Markdown Readers

LeafWiki is often compared to simple Markdown viewers or file browsers - but it’s built for a very different purpose.

|                 | Markdown Reader             | LeafWiki                                                         |
|-----------------|-----------------------------| -----------------------------------------------------------------|
| **Purpose**     | Read static `.md` files     | Create, organize, and serve a structured wiki                    |
| **Data source** | Reads directly from disk    | Stores pages as Markdown, but manages structure internally       |
| **Structure**   | Flat folder view            | True tree of pages & subpages                                    |
| **Editing**     | External text editor        | Built-in Markdown editor with live preview                       |
| **Uploads**     | Not supported               | Per-page assets (images, files)                                  |
| **Search**      | File names only             | Indexed full-text search (titles + content)                      |
| **Hosting**     | Static / local              | Self-hosted Go web app                                           |
| **Access**      | No authentication           | Role-based access (admin / editor)                               |

> LeafWiki doesn’t just *display* Markdown files - it **organizes, edits, and serves** them.  
> It’s not a viewer, but a small wiki engine that happens to store everything as Markdown.

---

## 🛠️ Installation (Production)

You can install LeafWiki in several environments.  
Below are some examples to get you started.

### Setup Examples
- [Install LeafWiki with nginx on Ubuntu](docs/install/nginx.md)
- [Install LeafWiki on a Raspberry Pi](docs/install/raspberry.md)

### Quick install
```bash
curl -sL https://raw.githubusercontent.com/perber/leafwiki/main/install.sh -o install.sh && chmod +x ./install.sh && sudo ./install.sh --arch amd64
```

This command performs a quick installation of LeafWiki on the target machine.

### ⚙️ Flags

| Flag               | Description                                                 | Default       |
|--------------------|-------------------------------------------------------------|---------------|
| `--arch`           | Target architecture for the binary (e.g. `amd64`, `arm64`)  |       -       |
| `--host`           | Host/IP address the server binds to                         | `0.0.0.0`     |
| `--port`           | Port the server listens on                                  | `8080`        |

#### ⚠️ Attention - Security

Sensitive information (JWT token, administrator password) appears in plain text in the *_systemd_* file `/etc/systemd/system/leafwiki.service`.
Make sure that this file is accessible only to authorized users.

 >The installation script has been tested on Ubuntu. 
 >If you test the script on other distributions or architectures, please provide feedback by opening an issue on the GitHub repository - this will help us improve compatibility and documentation.

---

### Docker installation

You can run LeafWiki in a Docker container:

```bash
docker run -p 8080:8080 ghcr.io/perber/leafwiki:latest --jwt-secret=yoursecret
```

By default, the container runs as root. You can specify a custom data directory using the `--data-dir` option.

If you want to avoid running as root, add `-u 1000:1000`:

```bash
docker run -p 8080:8080 -u 1000:1000 \
  ghcr.io/perber/leafwiki:latest --jwt-secret=yoursecret
```

By default the data directory inside the container will be `app/data/`

---

### Manual installation

```
# Download the latest release from GitHub
chmod +x leafwiki
./leafwiki --jwt-secret=yoursecret
```

Default port is `8080`, and the default data directory is `./data`.
You can change the data directory with the `--data-dir` flag.

> ✅ Native ARM64 builds are available in the [Releases](https://github.com/perber/leafwiki/releases) section.

### Default admin user

The first time you run LeafWiki, it will create an admin user with the default password `admin`.

You can change this password later in the admin settings or by using the CLI:

```bash
./leafwiki --admin-password=newpassword --jwt-secret=yoursecret
```

> Note: `--admin-password` (or the `LEAFWIKI_ADMIN_PASSWORD` env var) is only used on first startup, when no admin user exists yet.


### Reset Admin Password
If you need to reset the admin password, you can do so by running:

```bash
./leafwiki reset-admin-password
```

### ⚙️ CLI Flags

| Flag               | Description                                                 | Default       |
|--------------------|-------------------------------------------------------------|---------------|
| `--jwt-secret`     | Secret used for signing JWTs (required)                     | –             |
| `--host`           | Host/IP address the server binds to                         | `0.0.0.0`     |
| `--port`           | Port the server listens on                                  | `8080`        |
| `--data-dir`       | Directory where data is stored                              | `./data`      |
| `--admin-password` | Initial admin password (used only if no admin exists)       | `admin`       |
| `--public-access`  | Allow public access to the wiki (no auth required)          | `false`       |
   

### 🌱 Environment Variables

Instead of CLI flags, you can also configure LeafWiki using environment variables:

| Variable                 | Description                                                  | Default    |
|--------------------------|--------------------------------------------------------------|------------|
| `LEAFWIKI_HOST`          | Host/IP address the server binds to                          | `0.0.0.0`  |
| `LEAFWIKI_PORT`          | Port the server listens on                                   | `8080`     |
| `LEAFWIKI_DATA_DIR`      | Path to the data storage directory                           | `./data`   |
| `LEAFWIKI_ADMIN_PASSWORD`| Initial admin password *(used only if no admin exists yet)*  | `admin`    |
| `LEAFWIKI_JWT_SECRET`    | Secret used to sign JWT tokens *(required)*                  | –          |
| `LEAFWIKI_PUBLIC_ACCESS` | Allow public access to the wiki (no auth required)           | `false`    |

These environment variables override the default values and are especially useful in containerized or production environments.

Binding to localhost behind a reverse proxy
-------------------------------------------

If you run a reverse proxy (nginx, Caddy, Traefik) in front of LeafWiki, you may want the Go server to only listen on the loopback interface. You can do that with the `--host` flag or the `LEAFWIKI_HOST` env var:

```bash
# bind to localhost only
LEAFWIKI_HOST=127.0.0.1 ./leafwiki --jwt-secret=yoursecret

# or with the CLI flag
./leafwiki --host 127.0.0.1 --jwt-secret=yoursecret
```

When bound to `127.0.0.1`, the server will only listen on localhost and won't be directly reachable from the public internet; the reverse proxy can forward requests from the external interface.


## 🚀 Quick Start (Dev)

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
```


## 🗺️ Roadmap

### Planned
- [ ] Global Keyboard Shortcuts (e.g. Ctrl+F for search, ...)
- [ ] Further UI/UX improvements
- [ ] Your ideas are welcome! Open an issue or start a discussion!

----

### ✅ v0.1.0 – MVP
- [x] Tree-based page structure
- [x] Markdown file creation
- [x] Slug + file path mapping
- [x] Move / rename / delete logic
- [x] Markdown editor with preview
- [x] File/image uploads per page
- [x] Simple page title search
- [x] Asset management (images, files)
- [x] Basic JWT auth (session-based)

### ✅ v0.2.0 – Improved Editor Experience
- [x] Use CodeMirror for Markdown editing
- [x] Add Toolbar with common actions like bold, italic, links, etc.
- [x] Allow Undo/Redo actions

### ✅ v0.3.4 – Improved Asset Handling
- [x] Allow uploading multiple files at once
- [x] Allow renaming of uploaded files
- [x] Fix caching issues with uploaded assets
- [x] Fix syntax highlighting in preview
- [x] Fix favicon not displayed
- [x] ARM64 support for Raspberry Pi and other ARM devices (thanks @nahaktarun)

### ✅ v0.4.6 – Ready for Dogfooding
- [x] Add Search functionality for page titles and content
- [x] Add Mobile optimizations for better usability
- [x] Allow Public Pages (viewable pages without login)
- [x] Add shortcuts in the editor (e.g. Ctrl+S to save, Ctrl+B for bold, Ctrl+Z for undo, ...)
- [x] Smaller improvements and bugfixes in the UI
- [x] Added "Create & Edit" option to dialog to allow creating structure before editing
- [x] Warn user about unsaved changes when navigating away (via `beforeunload` and `react-router`)
- [x] Updated the tree view design – it now has a more documentation-style look
- [x] Print view support for pages (print-friendly layout)

### ✅ v0.4.7 – Stabilize
- [x] Several dependencies updates
- [x] Allow to configure `--host` to bind to specific IP (e.g. `--host 127.0.0.1`) - thanks @magnus-madsen for the suggestion!

### ✅ v0.4.8 – UX Improvements
- [x] Several dependencies updates
- [x] Not Found page now suggests creating a new page - thanks @magnus-madsen for the suggestion!
- [x] links to non-existing pages now show a create page dialog - thanks @magnus-madsen for the suggestion!
- [x] smaller UI improvements and bugfixes (e.g. green save button, ...)

### ✅ v0.4.9 – Mermaid Support, UX Improvements & Easier Installation

A special thanks for this release goes out to @Hugo-Galley.
He improved the documentation and the onboarding experience a lot! 

- [x] Mermaid.js diagram support
- [x] Copy page functionality added
- [x] Installation script added for binary - thanks to @Hugo-Galley
- [x] Improved docker builds with multi-arch support (amd64 + arm64)
- [x] Several UI/UX improvements and bugfixes
- [x] Stability improvements and dependency updates

### ✅ v0.4.10 – Clipboard Image/File Uploads, Resizable Scrollbar and other UX Improvements

- [x] Docker images now have labels and annotations - thanks @Hugo-Galley
- [x] Installer now has a welcome message - thanks @Hugo-Galley
- [x] Allow to upload files by using **CTRL+V** in the codemirror editor
- [x] Improve position for tooltip in the treeview
- [x] Add toggle to **show & hide the preview**
- [x] Add **resizable sidebar** - thanks @magnus-madsen for the suggestion!
- [x] Various bug fixes and UX/UI improvements
- [x] better e2e test coverage
- [x] Dependency updates across the project

---

## 🧠 Philosophy

- **Simple to run**: No container, no DB, just Go
- **Simple to host**: You know where your data is
- **Simple to trust**: Markdown is portable & future-proof


Your knowledge should live in files you own - not in a database you maintain.

---

## 🙋 Contributing

Contributions, discussions and feedback are very welcome.  
This project is still early – feel free to open issues or ideas!

## 📬 Stay in the Loop

> More updates coming soon.  
> Watch the repo or drop a star ⭐ if you’re curious!
