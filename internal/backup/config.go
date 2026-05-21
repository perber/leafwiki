package backup

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
	IntervalMinutes int     // how often to run the scheduled backup, default 60
}