# Docker Git Backup

LeafWiki supports automated Git backup of your wiki content to a remote Git repository.

## Overview

The backup feature pushes your `root/` (pages) and `assets/` (attachments) directories to a remote Git repository on a configurable schedule.

## Configuration

Enable git backup via environment variables:

| Variable | Description | Required |
|---|---|---|
| `LEAFWIKI_GIT_BACKUP` | Set to `"true"` to enable backup | Yes |
| `LEAFWIKI_GIT_BACKUP_REMOTE` | SSH remote URL (e.g. `git@github.com:user/wiki-backup.git`) | Yes |
| `LEAFWIKI_GIT_BACKUP_BRANCH` | Remote branch to push to (default: `main`) | No |
| `LEAFWIKI_GIT_BACKUP_AUTHOR_NAME` | Git commit author name (default: `LeafWiki Backup`) | No |
| `LEAFWIKI_GIT_BACKUP_AUTHOR_EMAIL` | Git commit author email (default: `backup@leafwiki.local`) | No |
| `LEAFWIKI_GIT_BACKUP_INTERVAL` | Backup interval in minutes (default: `60`) | No |

## SSH Key Authentication

The backup uses SSH to authenticate with your Git remote. Provide the private key via:

### Option 1: Docker Secret (Recommended for production)

```yaml
services:
  leafwiki:
    image: ghcr.io/perber/leafwiki:latest
    environment:
      LEAFWIKI_GIT_BACKUP: "true"
      LEAFWIKI_GIT_BACKUP_REMOTE: "git@github.com:user/wiki-backup.git"
      LEAFWIKI_GIT_BACKUP_BRANCH: "main"
      LEAFWIKI_GIT_BACKUP_INTERVAL: "60"
    secrets:
      - backup_ssh_key

secrets:
  backup_ssh_key:
    file: ./backup_key
```

The SSH key will be mounted at `/run/secrets/backup_ssh_key` inside the container.

### Option 2: File Mount

Mount your SSH key into the container:

```yaml
services:
  leafwiki:
    image: ghcr.io/perber/leafwiki:latest
    environment:
      LEAFWIKI_GIT_BACKUP: "true"
      LEAFWIKI_GIT_BACKUP_REMOTE: "git@github.com:user/wiki-backup.git"
      LEAFWIKI_GIT_BACKUP_SSH_KEY_PATH: "/secrets/backup_key"
    volumes:
      - ./backup_key:/secrets/backup_key:ro
```

## Docker Compose Example

```yaml
services:
  leafwiki:
    image: ghcr.io/perber/leafwiki:latest
    environment:
      LEAFWIKI_JWT_SECRET: "${JWT_SECRET}"
      LEAFWIKI_ADMIN_PASSWORD: "${ADMIN_PASSWORD}"
      LEAFWIKI_GIT_BACKUP: "true"
      LEAFWIKI_GIT_BACKUP_REMOTE: "git@github.com:user/wiki-backup.git"
      LEAFWIKI_GIT_BACKUP_BRANCH: "main"
      LEAFWIKI_GIT_BACKUP_INTERVAL: "60"
    secrets:
      - backup_ssh_key

secrets:
  backup_ssh_key:
    file: ./backup_key

volumes:
  - ./data:/app/data
```

## Repository Setup

Before enabling backup, ensure your remote repository is initialized:

```bash
# Create a bare repository on GitHub/GitLab
# The first time LeafWiki starts with git-backup enabled,
# it will create an initial commit with your existing content
```

## What Gets Backed Up

Only the following directories are staged and committed:
- `data/root/` — all wiki pages (Markdown files)
- `data/assets/` — uploaded attachments

The following are explicitly excluded (not backed up):
- `*.db` — SQLite database (search index, metadata)
- `*.db-shm`, `*.db-wal` — SQLite WAL files
- `*.tmp`, `.tmp-*` — temporary files

## Admin UI

Once enabled, a "Backup" section appears in the admin settings panel at `/settings/backup`. You can:
- View the current backup status
- See the last backup time and any errors
- Trigger an immediate backup with "Push now"

## Troubleshooting

### SSH Key Permissions

Ensure your SSH private key has correct permissions:
```bash
chmod 600 backup_key
```

### First Backup Fails

If the first backup fails, check:
1. SSH key is correctly mounted/readable
2. The remote repository exists and you have push access
3. SSH known_hosts is configured for the Git host

### Verify Backup is Working

Check the logs:
```bash
docker compose logs -f leafwiki | grep -i backup
```

Or use the admin API:
```bash
curl -u admin:PASSWORD http://localhost:8080/api/admin/backup/status
```