package main

import (
	"time"

	"github.com/urfave/cli/v3"
)

// backupOptions holds the options for git backup, snapshots and restore.
type backupOptions struct {
	gitBackup              bool
	gitBackupAuthorName    string
	gitBackupAuthorEmail   string
	gitBackupRemote        string
	gitBackupBranch        string
	gitBackupSSHKeyPath    string
	gitBackupSSHKey        string
	gitBackupSSHKnownHosts string
	gitBackupHTTPUsername  string
	gitBackupHTTPPassword  string
	gitBackupInterval      time.Duration
	snapshot               bool
	snapshotInterval       time.Duration
	snapshotRetention      int
	snapshotDir            string
	restoreUploadMaxSize   string
}

// Flags declares them. Adding an option here is the whole change: the flag,
// its default, its environment variable and its help text are one literal,
// and --help is generated from it.
func (o *backupOptions) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.BoolFlag{
			Name:        "git-backup",
			Destination: &o.gitBackup,
			Category:    catGitBackup,
			Usage:       "enable git backup to a remote repository",
			Sources:     envBoolVars("LEAFWIKI_GIT_BACKUP"),
		},
		&cli.StringFlag{
			Name:        "git-backup-author-name",
			Destination: &o.gitBackupAuthorName,
			Category:    catGitBackup,
			Usage:       "git commit author name for backups",
			Value:       "LeafWiki Backup",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_AUTHOR_NAME"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "git-backup-author-email",
			Destination: &o.gitBackupAuthorEmail,
			Category:    catGitBackup,
			Usage:       "git commit author email for backups",
			Value:       "backup@leafwiki.local",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_AUTHOR_EMAIL"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "git-backup-remote",
			Destination: &o.gitBackupRemote,
			Category:    catGitBackup,
			Usage:       "git remote URL (SSH or HTTP(S)) for backups; leave unset for local-only backups",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_REMOTE"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "git-backup-branch",
			Destination: &o.gitBackupBranch,
			Category:    catGitBackup,
			Usage:       "git branch to push to",
			Value:       "main",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_BRANCH"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "git-backup-ssh-key-path",
			Destination: &o.gitBackupSSHKeyPath,
			Category:    catGitBackup,
			Usage:       "path to SSH private key for git backup",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_SSH_KEY_PATH"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        gitBackupSSHKeyFlagName,
			Destination: &o.gitBackupSSHKey,
			Category:    catGitBackup,
			Usage:       "raw SSH private key for git backup (env var preferred)",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_SSH_KEY"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "git-backup-ssh-known-hosts",
			Destination: &o.gitBackupSSHKnownHosts,
			Category:    catGitBackup,
			Usage:       "known_hosts file for SSH host key verification (MITM protection)",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_SSH_KNOWN_HOSTS"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        "git-backup-http-username",
			Destination: &o.gitBackupHTTPUsername,
			Category:    catGitBackup,
			Usage:       "username for HTTP(S) basic auth on http(s):// remotes",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_HTTP_USERNAME"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:        gitBackupHTTPPasswordFlagName,
			Destination: &o.gitBackupHTTPPassword,
			Category:    catGitBackup,
			Usage:       "password or access token for HTTP(S) basic auth (env var preferred)",
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_HTTP_PASSWORD"),
			Config:      trimmed,
		},
		&cli.DurationFlag{
			Name:        "git-backup-interval",
			Destination: &o.gitBackupInterval,
			Category:    catGitBackup,
			Usage:       "git backup interval (e.g. 60m, 2h); 0 = manual-only, no automatic scheduling",
			Value:       60 * time.Minute,
			Sources:     envVars("LEAFWIKI_GIT_BACKUP_INTERVAL"),
			DefaultText: "60m",
		},
		&cli.BoolFlag{
			Name:        "snapshot",
			Destination: &o.snapshot,
			Category:    catSnapshots,
			Usage:       "full backup snapshots (ZIP incl. the SQLite database); disable with --snapshot=false",
			Value:       true,
			DefaultText: "true",
			Sources:     envBoolVars("LEAFWIKI_SNAPSHOT"),
		},
		&cli.DurationFlag{
			Name:        "snapshot-interval",
			Destination: &o.snapshotInterval,
			Category:    catSnapshots,
			Usage:       "snapshot interval (e.g. 24h, 6h); 0 = manual-only, no automatic scheduling",
			Value:       24 * time.Hour,
			Sources:     envVars("LEAFWIKI_SNAPSHOT_INTERVAL"),
			DefaultText: "24h",
		},
		&cli.IntFlag{
			Name:        "snapshot-retention",
			Destination: &o.snapshotRetention,
			Category:    catSnapshots,
			Usage:       "number of most recent snapshots to keep; <= 0 = keep all",
			Value:       10,
			Sources:     envVars("LEAFWIKI_SNAPSHOT_RETENTION"),
		},
		&cli.StringFlag{
			Name:        "snapshot-dir",
			Destination: &o.snapshotDir,
			Category:    catSnapshots,
			Usage:       "directory to store snapshot ZIPs in",
			DefaultText: "<data-dir>/snapshots",
			Sources:     envVars("LEAFWIKI_SNAPSHOT_DIR"),
			Config:      trimmed,
		},
		&cli.StringFlag{
			Name:             "restore-upload-max-size",
			Destination:      &o.restoreUploadMaxSize,
			Category:         catSnapshots,
			Usage:            "maximum size of an uploaded backup ZIP to restore from (e.g. 500MiB, 500MB, 524288000)",
			Value:            "500MiB",
			Sources:          envVars("LEAFWIKI_RESTORE_UPLOAD_MAX_SIZE"),
			Validator:        validateByteSizeValue,
			ValidateDefaults: true,
			Config:           trimmed,
		},
	}
}
