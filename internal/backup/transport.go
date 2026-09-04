package backup

import "strings"

// RemoteTransport is how a git backup remote is reached.
type RemoteTransport string

const (
	// TransportUnknown is an empty remote (local-only) or an unrecognised scheme.
	TransportUnknown RemoteTransport = ""
	// TransportHTTP covers http:// and https:// remotes (basic auth / token).
	TransportHTTP RemoteTransport = "https"
	// TransportSSH covers git@host:path and ssh:// remotes (private key).
	TransportSSH RemoteTransport = "ssh"
)

// ClassifyRemote maps a remote URL to its transport. It is the single place that
// decides "HTTP(S) vs SSH vs neither" — credential construction, credential
// validation, the settings form's auth-mode field and the stale-credential
// pruning all defer to it so they can never disagree.
func ClassifyRemote(remoteURL string) RemoteTransport {
	l := strings.ToLower(strings.TrimSpace(remoteURL))
	switch {
	case l == "":
		return TransportUnknown
	case strings.HasPrefix(l, "http://"), strings.HasPrefix(l, "https://"):
		return TransportHTTP
	case strings.HasPrefix(l, "git@"), strings.HasPrefix(l, "ssh://"):
		return TransportSSH
	default:
		return TransportUnknown
	}
}
