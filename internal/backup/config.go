package backup

import "time"

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
