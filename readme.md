# 🌿 LeafWiki

**A lightweight, tree-based Markdown wiki – no database, just a single Go binary.**

LeafWiki is designed for teams and individuals who want a clean, fast, and self-hosted knowledge base — with full control over structure, content, and deployment.

---

![LeafWiki](./preview.gif)

---

## 📦 Status

> **MVP released – actively developed** 
> LeafWiki is already powering real documentation workflows — but it’s still in early development.
> It's **not yet production-ready** for mission-critical teams.
>
> Expect active development, some rough edges, and fast iteration.
> 
> Want to try it and help shape it? You’re more than welcome.

LeafWiki now builds and runs natively on:
- **Linux (x86_64 and ARM64)**
- **Windows (x86_64)**
- **Raspberry Pi (tested with 64-bit OS)**

---

## ✨ Features

- 🧾 Markdown-first with live editor + preview
- 🌲 True tree-structured pages (nested folders)
- 🔒 Role-based access (admin / editor)
- 🧠 no DB required
- 📂 Per-page assets with upload support
- 🖼️ Embed images and files with Markdown
- ⚙️ Single statically-linked Go binary (no dependencies)
- 🚀 Easily self-hosted (Docker or standalone)
- 🔁 Session auth with JWT tokens + refresh
- 🔍 Search functionality for page titles and content
- 📱 Mobile-friendly design
- 🌐 Public pages (viewable without login)

---

## 💭 Why Another Wiki?

After trying out tools like Wiki.js, Confluence, and DokuWiki, I wanted something simpler: no database, easy to host, Markdown-based, and truly Git-friendly.

- Why use a database just to store Markdown?
- Why should setup be a weekend project?
- Why can't a wiki just be file-based and fast?

**LeafWiki** was born out of that frustration — and the desire to have:

- 🧾 Clean Markdown files, organized in folders
- 🧠 A real tree structure, not a flat list
- ⚙️ A single binary with no external dependencies
- 🛠️ Something teams can actually self-host without DevOps pain

It’s not trying to be everything — just a solid, minimal wiki for people who want **clarity over complexity.**

---

## 🛠️ Installation (Production)

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
| `--port`           | Port the server listens on                                  | `8080`        |
| `--data-dir`       | Directory where data is stored                              | `./data`      |
| `--admin-password` | Initial admin password (used only if no admin exists)       | `admin`       |
| `--public-access`  | Allow public access to the wiki (no auth required)          | `false`       |
   

### 🌱 Environment Variables

Instead of CLI flags, you can also configure LeafWiki using environment variables:

| Variable                 | Description                                                  | Default    |
|--------------------------|--------------------------------------------------------------|------------|
| `LEAFWIKI_PORT`          | Port the server listens on                                   | `8080`     |
| `LEAFWIKI_DATA_DIR`      | Path to the data storage directory                           | `./data`   |
| `LEAFWIKI_ADMIN_PASSWORD`| Initial admin password *(used only if no admin exists yet)*  | `admin`    |
| `LEAFWIKI_JWT_SECRET`    | Secret used to sign JWT tokens *(required)*                  | –          |
| `LEAFWIKI_PUBLIC_ACCESS` | Allow public access to the wiki (no auth required)           | `false`    |

These environment variables override the default values and are especially useful in containerized or production environments.


## 🚀 Quick Start (Dev)

```
# 1. Clone the repo

git clone https://github.com/perber/leafwiki.git
cd leafwiki

# 2. Install frontend dependencies

cd ui/leafwiki-ui
npm install
npm run build-watch // This will generate the UI files into the `./dist` folder at the root and continue watching for changes.


# 3. In another terminal, start the backend
The backend requires the generate UI files to be present.  
To generate the UI files, run: `npm run build-watch` in the `ui/leafwiki-ui` directory.

cd ../../cmd/leafwiki
go run main.go
```


## 🗺️ Roadmap

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

### ✅ v0.4.4 – Ready for Dogfooding
- [x] Add Search functionality for page titles and content
- [x] Add Mobile optimizations for better usability
- [x] Allow Public Pages (viewable pages without login)
- [x] Add shortcuts in the editor (e.g. Ctrl+S to save, Ctrl+B for bold, Ctrl+Z for undo, ...)
- [x] Smaller improvements and bugfixes in the UI
- [x] Added "Create & Edit" option to dialog to allow creating structure before editing
- [x] Warn user about unsaved changes when navigating away (via `beforeunload` and `react-router`)
- [x] Updated the tree view design – it now has a more documentation-style look


### Upcoming Features in Version 0.5.0
- [ ] Static pages (Required for SEO and public pages)
- [ ] Dogfooding (using LeafWiki to document LeafWiki)
- [ ] Showcase release

### 🧪 Future Ideas
- [ ] Automatic import of existing Markdown files
- [ ] Optimistic locking (conflict resolution)
- [ ] Versioning (history)
- [ ] Git integration
- [ ] Automatic update of links

---

## 🧠 Philosophy

- **Simple to run**: No container, no DB, just Go
- **Simple to host**: You know where your data is
- **Simple to trust**: Markdown is portable & future-proof

---

## 🙋 Contributing

Contributions, discussions and feedback are very welcome.  
This project is still early – feel free to open issues or ideas!

## 💬 Chat on Discord

We now have an official [Discord server](https://discord.gg/gX7Sw3PjUP)
→ ask questions, get help, contribute, or just say hi.

Main channels:
- `#welcome` – Say hi, introduce yourself
- `#general` – General discussion about LeafWiki (ideas, feedback, off-topic, ...)
- `#support` – Help with issues, questions, or troubleshooting
- `#release-announcements` – Updates on new releases, features, and improvements
- `#questions` – Any questions about the code, structure, roadmap, or contributing

---

## 📬 Stay in the Loop

> More updates coming soon.  
> Watch the repo or drop a star ⭐ if you’re curious!
