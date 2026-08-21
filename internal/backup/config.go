package backup

import "time"

type Config struct {
	Enabled           bool
	RootDir           string // path to LeafWiki root/ content directory (live data)
	AssetsDir         string // path to LeafWiki assets/ directory (live data)
	Path              string // optional relative subdirectory inside the git repo (monorepo), e.g. docs/wiki
	AuthorName        string
	AuthorEmail       string
	RemoteURL         string        // SSH remote, e.g. git@github.com:user/repo.git
	Branch            string        // remote branch to push to, default "main"
	SSHKeyPath        string        // path to private key file (optional if SSHKey set)
	SSHKey            string        // raw PEM private key (env var preferred)
	SSHKnownHostsPath string        // path to known_hosts file for MITM protection (optional)
	Interval          time.Duration // how often to run the scheduled backup; 0 = manual-only
}
