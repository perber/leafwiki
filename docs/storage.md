# How LeafWiki stores data

LeafWiki keeps wiki **content on disk as Markdown** and uses **SQLite** for metadata, search, users, and other runtime state. Both live under a single **data directory** (default `./data`, or `/app/data` in the Docker image).

## Data directory layout

| Path | Purpose |
|------|---------|
| `root/` | Page content as `.md` files, mirroring the wiki tree |
| `assets/` | Uploaded images and other files referenced from pages |
| `wiki.db` | SQLite database (users, sessions, tree order, tags, search index, …) |
| `snapshots/` | Full backup ZIPs when snapshot backups are enabled (default location) |
| `.git/` | Local git repo used by Git Backup (only when `--git-backup` is enabled) |

Page identity is stored in each file's frontmatter (`leafwiki_id`), not in the filename. That lets pages survive renames and moves without breaking links.

## What lives where

**On disk (`root/`, `assets/`):**

- Human-readable Markdown and uploads
- Safe to back up with `cp -r` or rsync when LeafWiki is stopped
- Editable outside the app (trigger a resync from the admin UI or send `SIGUSR1` / `SIGHUP` after external edits)

**In SQLite (`wiki.db`):**

- User accounts, roles, and sessions
- Page tree structure and manual sort order
- Tags, backlinks metadata, and the full-text search index
- Revision history (when `--enable-revision` is on)
- Other app state that is not duplicated in Markdown files

You cannot fully reconstruct a running wiki from `root/` alone — you need the database too (or a snapshot that includes both).

## Related docs

- [Backup and restore](backup-restore.md)
- [Install with nginx](install/nginx.md)
