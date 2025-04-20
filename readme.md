# 🌿 LeafWiki

**A lightweight, tree-based Markdown wiki – no database, no Docker, just a single Go binary.**

LeafWiki is designed for teams and individuals who want a clean, fast, and self-hosted knowledge base — with full control over structure, content, and deployment.

---

![Leafwiki](./preview.gif)

---

## 📦 Status

> **MVP released – actively developed** 
LeafWiki is usable and already powers knowledge workflows, but is still in early public stages.  
Expect improvements, polishing, and community feedback over the next releases.

---

## ✨ Features

- 🧾 Markdown-first with live editor + preview
- 🌲 True tree-structured pages (nested folders)
- 🔒 Role-based access (admin / editor)
- 🧠 Built for Git – no DB required
- 📂 Per-page assets with upload support
- 🖼️ Embed images and files with Markdown
- ⚙️ Single statically-linked Go binary (no dependencies)
- 🚀 Easily self-hosted (Docker or standalone)
- 🔁 Session auth with JWT tokens + refresh

---

## 💭 Why Another Wiki?

After trying out tools like Wiki.js, Confluence, and DokuWiki, I wanted something simpler: no database, easy to host, Markdown-based, and truly Git-friendly.

- Why use a database just to store Markdown?
- Why should setup be a weekend project?
- Why can't a wiki just be Git-friendly, file-based and fast?

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
wget https://github.com/yourname/leafwiki/releases/latest/download/leafwiki-linux-amd64
chmod +x leafwiki-linux-amd64
./leafwiki-linux-amd64 --jwt-secret=yoursecret
```

Default port is `:8080`, and the default data directory is `./data`.
You can change the data directory with the `--data-dir` flag.


### ⚙️ CLI Flags

| Flag               | Description                                 | Default       |
|--------------------|---------------------------------------------|---------------|
| `--jwt-secret`     | Secret used for signing JWTs (required)     | –             |
| `--port`           | Port the server listens on                  | `:8080`       |
| `--data-dir`       | Directory where data is stored              | `./data`      |


## 🚀 Quick Start (Dev)

```
# 1. Clone the repo

git clone https://github.com/yourname/leafwiki.git
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

### ✅ v0.1 – MVP
- [x] Tree-based page structure
- [x] Markdown file creation
- [x] Slug + file path mapping
- [x] Move / rename / delete logic
- [x] Markdown editor with preview
- [x] File/image uploads per page
- [x] Simple page title search
- [x] Asset management (images, files)
- [x] Basic JWT auth (session-based)


### 🧪 Future Ideas

- [ ] Optimistic locking (conflict resolution)
- [ ] Versioning (history)
- [ ] Upload multiple files
- [ ] Syntax Highlighting
- [ ] Full-text search
- [ ] TOC on page
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

---

## 📬 Stay in the Loop

> More updates coming soon.  
> Watch the repo or drop a star ⭐ if you’re curious!