package backup

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	Enabled           bool
	RootDir           string // path to LeafWiki root/ content directory
	AssetsDir         string // path to LeafWiki assets/ directory
	AuthorName        string
	AuthorEmail       string
	RemoteURL         string        // SSH remote (git@github.com:user/repo.git) or HTTPS remote (https://github.com/user/repo.git)
	Branch            string        // remote branch to push to, default "main"
	SSHKeyPath        string        // path to private key file (optional if SSHKey set)
	SSHKey            string        // raw PEM private key (env var preferred)
	SSHKnownHostsPath string        // path to known_hosts file for MITM protection (optional)
	HTTPUsername      string        // username for HTTP(S) basic auth
	HTTPPassword      string        // password or access token for HTTP(S) basic auth (env var preferred)
	Interval          time.Duration // how often to run the scheduled backup; 0 = manual-only
}

// Defaults applied to a settings-supplied Config when the corresponding field
// is left empty. They mirror the CLI flag defaults in
// cmd/leafwiki/flags_backup.go so an ENV-configured and a UI-configured backup
// behave identically.
const (
	DefaultBranch      = "main"
	DefaultAuthorName  = "LeafWiki Backup"
	DefaultAuthorEmail = "backup@leafwiki.local"

	// MinSettingsInterval / MaxSettingsInterval bound the sync interval an
	// admin may set through the settings UI. The ENV/flag path still allows
	// 0 (manual-only) and sub-minute values; the UI deliberately does not.
	MinSettingsInterval = 2 * time.Minute
	MaxSettingsInterval = 24 * time.Hour
)

// WithSettingsDefaults returns a copy of c with empty optional fields filled in
// from the Default* constants. Credentials and the remote URL are never
// defaulted — those must be supplied explicitly.
func (c Config) WithSettingsDefaults() Config {
	if strings.TrimSpace(c.Branch) == "" {
		c.Branch = DefaultBranch
	}
	if strings.TrimSpace(c.AuthorName) == "" {
		c.AuthorName = DefaultAuthorName
	}
	if strings.TrimSpace(c.AuthorEmail) == "" {
		c.AuthorEmail = DefaultAuthorEmail
	}
	return c
}

// ValidateForSettings checks a Config that came from the admin settings UI.
// Unlike the ENV/flag path it requires a remote (a UI-configured backup always
// pushes somewhere — that is the whole point of the "test connection" step) and
// clamps the interval to [MinSettingsInterval, MaxSettingsInterval].
//
// Call WithSettingsDefaults first so empty optional fields don't trip the
// checks here.
func (c Config) ValidateForSettings() error {
	if strings.TrimSpace(c.RemoteURL) == "" {
		return fmt.Errorf("a remote repository URL is required")
	}
	if c.Interval < MinSettingsInterval || c.Interval > MaxSettingsInterval {
		return fmt.Errorf("sync interval must be between %s and %s", MinSettingsInterval, MaxSettingsInterval)
	}
	if strings.TrimSpace(c.AuthorEmail) == "" || strings.TrimSpace(c.AuthorName) == "" {
		return fmt.Errorf("commit author name and email are required")
	}
	return ValidateRemoteCredentials(c.RemoteURL, c.SSHKey, c.SSHKeyPath, c.HTTPUsername, c.HTTPPassword)
}

// RedactRemoteURL masks any credentials embedded in a remote URL
// (https://user:token@host/…) for safe logging and display. Non-URL remotes
// (git@host:path) are returned unchanged.
func RedactRemoteURL(remote string) string { return redactRemote(remote) }

// ValidateRemoteCredentials checks that the remote URL is a supported transport
// (SSH or HTTP(S)) and that credentials matching that transport are present:
// HTTP(S) remotes need a username + password/token (or credentials embedded in
// the URL), SSH remotes need a private key. An empty remote is local-only and
// needs nothing. This is the shared core of both the ENV/flag validation in
// cmd/leafwiki and Config.ValidateForSettings.
func ValidateRemoteCredentials(remote, sshKey, sshKeyPath, httpUsername, httpPassword string) error {
	if strings.TrimSpace(remote) == "" {
		return nil
	}
	lower := strings.ToLower(remote)

	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		if httpUsername == "" && httpPassword == "" {
			if parsed, err := url.Parse(remote); err == nil && parsed.User != nil {
				return nil
			}
			return fmt.Errorf("an HTTP(S) remote requires both a username and a password/token")
		}
		if httpUsername == "" || httpPassword == "" {
			return fmt.Errorf("HTTP(S) basic auth requires both a username and a password/token; got only one of them")
		}
		return nil

	case strings.HasPrefix(lower, "git@"), strings.HasPrefix(lower, "ssh://"):
		if sshKey == "" && sshKeyPath == "" {
			return fmt.Errorf("an SSH private key (inline or a key file path) is required for an SSH remote")
		}
		return nil

	default:
		return fmt.Errorf("the remote must be an SSH URL (git@host:user/repo.git or ssh://...) or an HTTP(S) URL")
	}
}
