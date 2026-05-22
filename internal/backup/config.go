package backup

import "time"

type Config struct {
	Enabled        bool
	RootDir        string   // path to LeafWiki root/ content directory
	AssetsDir      string   // path to LeafWiki assets/ directory
	AuthorName     string
	AuthorEmail    string
	RemoteURL      string   // SSH remote, e.g. git@github.com:user/repo.git
	Branch         string   // remote branch to push to, default "main"
	SSHKeyPath     string   // path to private key file (optional if SSHKey set)
	SSHKey         string   // raw PEM private key (env var preferred)
	SSHKnownHosts  string   // known_hosts content for MITM protection (optional)
	IntervalMinutes int     // how often to run the scheduled backup, default 60
}

// Duration returns the interval as a time.Duration.
func (c *Config) Duration() time.Duration {
	return time.Duration(c.IntervalMinutes) * time.Minute
}