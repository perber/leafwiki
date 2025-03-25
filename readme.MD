# 🌿 LeafWiki

**A lightweight, tree-based Markdown wiki – no database, no Docker, just a single Go binary.**

LeafWiki is designed for teams and individuals who want a clean, fast, and self-hosted knowledge base — with full control over structure, content, and deployment.

---

## 📦 Status

> LeafWiki is currently under active development.  
The current version focuses on backend tree structure and file logic.

---

## ✨ Features

- 📁 Tree-structured pages (not flat)
- 📝 Markdown files directly on disk
- 🔄 Built for Git-based versioning (planned)
- 🚫 No database, no Docker, no nonsense
- ⚙️ Single binary written in Go
- ➕ Pages, folders, slugs, and move operations supported
- 🧠 Simple API for future UI integrations

---

## 💭 Why Another Wiki?

After working with tools like Wiki.js, Confluence, and DokuWiki, I often found myself asking the same questions:

- Why do I need a database for a bunch of Markdown files?
- Why is the setup so complex for something that should be simple?
- Why can't a wiki just be Git-friendly, file-based and fast?

**LeafWiki** was born out of that frustration — and the desire to have:

- 🧾 Clean Markdown files, organized in folders
- 🧠 A real tree structure, not a flat list
- ⚙️ A single binary with no external dependencies
- 🛠️ Something teams can actually self-host without DevOps pain

It’s not trying to be everything — just a solid, minimal wiki for people who want **clarity over complexity.**

---

## 🚀 Quick Start (Dev)

```bash
git clone https://github.com/yourname/leafwiki.git
cd cmd

go run main.go
```

## 🗺️ Roadmap

### ✅ v0.1 – Core
- [x] Tree-based page structure
- [x] Markdown file creation
- [x] Slug + file path mapping
- [x] Move / rename / delete logic

### 🧪 v0.2 – MVP
- [ ] Markdown editor with preview
- [ ] File/image uploads per page
- [ ] Reference/link detection
- [ ] Simple search
- [ ] Basic auth (opt-in)

### 🛠️ v0.3+
- [ ] Git integration
- [ ] Access control & roles
- [ ] Public / private content support

---

## 🧠 Philosophy

- **Simple to run**: No container, no DB, just Go
- **Simple to host**: You know where your data is
- **Simple to trust**: Markdown is portable & future-proof

---

## 📄 License

> License will be defined before public release.  
> Open for feedback and suggestions.

---

## 🙋 Contributing

Contributions, discussions and feedback are very welcome.  
This project is still early – feel free to open issues or ideas!

---

## 📬 Stay in the Loop

> More updates coming soon.  
> Watch the repo or drop a star ⭐ if you’re curious!