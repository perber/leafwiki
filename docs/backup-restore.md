# Backup and restore

LeafWiki offers three backup strategies, from simplest to most integrated.

## 1. Copy the data directory (full backup)

The most complete offline backup is a copy of the entire data directory **while LeafWiki is stopped**:

```bash
systemctl stop leafwiki   # or stop your container/process
cp -a /path/to/data /path/to/data-backup-$(date +%F)
systemctl start leafwiki
```

This includes Markdown pages, assets, the SQLite database, and any snapshot ZIPs already on disk.

## 2. Snapshot backups (built-in, recommended)

Since v0.12.0, LeafWiki can create **full snapshot ZIPs** that bundle `root/`, `assets/`, and the SQLite database. Snapshots are enabled by default.

### Configuration

| Flag / env | Default | Description |
|------------|---------|-------------|
| `--snapshot` / `LEAFWIKI_SNAPSHOT` | `true` | Enable snapshot backups |
| `--snapshot-interval` / `LEAFWIKI_SNAPSHOT_INTERVAL` | `24h` | Auto snapshot interval; `0` = manual only |
| `--snapshot-retention` / `LEAFWIKI_SNAPSHOT_RETENTION` | `10` | Keep the N most recent snapshots; `<= 0` = keep all |
| `--snapshot-dir` / `LEAFWIKI_SNAPSHOT_DIR` | `<data-dir>/snapshots` | Where ZIP files are stored |

### From the admin UI

Admins can open **Snapshot** settings to:

- Trigger a snapshot immediately
- Download existing snapshot ZIPs
- Upload a snapshot ZIP to restore (replaces the live data directory)
- Delete old snapshots

Restoring from the UI takes the wiki briefly offline while data is swapped.

### From the CLI (offline restore)

To restore onto a **stopped** instance (e.g. disaster recovery on a fresh machine):

```bash
./leafwiki --data-dir ./data restore-snapshot /path/to/snapshot.zip
```

Then start LeafWiki normally. The data directory must exist and LeafWiki must not be running against it during restore.

## 3. Git Backup (content only, experimental)

`--git-backup` pushes `root/` and `assets/` to a remote Git repository over SSH on a schedule. It does **not** include the SQLite database.

Use Git Backup for off-site **content** history; pair it with snapshot backups or data-directory copies for a full restore. See the [Git Backup section in the README](../README.md#git-backup-v0113-experimental) for SSH key and remote setup.

## Which method to use

| Goal | Method |
|------|--------|
| Full restore after hardware failure | Snapshot ZIP or `cp -a` of the data dir |
| Scheduled automated full backups | Snapshots (`--snapshot-interval`) |
| Version history of page Markdown in Git | Git Backup |
| Quick manual export before an upgrade | Trigger snapshot in the UI, then download the ZIP |

## Related docs

- [How LeafWiki stores data](storage.md)
- [Install with nginx](install/nginx.md)
