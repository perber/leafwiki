package backup

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	sshcrypto "golang.org/x/crypto/ssh"
)

// buildAuth builds the transport authentication for cfg's remote: HTTP(S)
// remotes use basic auth (username + password/token), everything else
// (git@..., ssh://...) uses the SSH private key. Shared by *Repository's git
// operations and by TestRemote's pre-save connectivity check so both construct
// credentials identically.
func buildAuth(cfg Config) (transport.AuthMethod, error) {
	if isHTTPRemote(cfg.RemoteURL) {
		return buildHTTPAuth(cfg)
	}
	return buildSSHAuth(cfg)
}

// buildHTTPAuth builds HTTP basic auth from config. On GitHub and friends the
// password is a personal access token, which is exactly the point of this auth
// mode: a fine-grained token can be scoped to a single repository, whereas an
// SSH key is bound to the whole account.
func buildHTTPAuth(cfg Config) (transport.AuthMethod, error) {
	if cfg.HTTPUsername == "" && cfg.HTTPPassword == "" {
		// No explicit credentials: the remote URL may already embed them, or the
		// repository may be publicly writable (self-hosted setups behind a VPN).
		slog.Debug("buildHTTPAuth: no HTTP credentials configured, connecting without basic auth")
		return nil, nil
	}
	if cfg.HTTPUsername == "" || cfg.HTTPPassword == "" {
		return nil, fmt.Errorf("HTTP basic auth requires both a username and a password/token")
	}
	slog.Debug("buildHTTPAuth: using HTTP basic auth", "username", cfg.HTTPUsername)
	return &githttp.BasicAuth{
		Username: cfg.HTTPUsername,
		Password: cfg.HTTPPassword,
	}, nil
}

// buildSSHAuth builds SSH authentication from config.
func buildSSHAuth(cfg Config) (ssh.AuthMethod, error) {
	var privateKey []byte
	var err error

	// Try SSHKey string first
	switch {
	case cfg.SSHKey != "":
		slog.Debug("buildSSHAuth: using inline SSH key")
		privateKey = []byte(cfg.SSHKey)
	case cfg.SSHKeyPath != "":
		slog.Debug("buildSSHAuth: reading SSH key from file", "path", cfg.SSHKeyPath)
		privateKey, err = os.ReadFile(cfg.SSHKeyPath)
		if err != nil {
			slog.Debug("buildSSHAuth: failed to read SSH key file", "path", cfg.SSHKeyPath, "error", err)
			return nil, fmt.Errorf("failed to read SSH key: %w", err)
		}
		slog.Debug("buildSSHAuth: SSH key file read successfully", "path", cfg.SSHKeyPath, "size", len(privateKey))
	default:
		slog.Debug("buildSSHAuth: no SSH key configured (neither inline nor path)")
		return nil, fmt.Errorf("no SSH key provided")
	}

	// Parse the private key using x/crypto/ssh
	signer, err := sshcrypto.ParsePrivateKey(privateKey)
	if err != nil {
		slog.Error("failed to parse SSH key", "error", err, "path", cfg.SSHKeyPath)
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}
	slog.Debug("buildSSHAuth: SSH key parsed successfully", "keyType", signer.PublicKey().Type())

	auth := &ssh.PublicKeys{
		User:   "git",
		Signer: signer,
	}

	// Use known hosts file for MITM protection.
	// Priority: explicit config path → ~/.ssh/known_hosts → InsecureIgnoreHostKey (warn).
	// If the configured path is unreadable we fail hard rather than silently downgrading.
	if cfg.SSHKnownHostsPath != "" {
		cb, err := ssh.NewKnownHostsCallback(cfg.SSHKnownHostsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load known_hosts file %q: %w", cfg.SSHKnownHostsPath, err)
		}
		auth.HostKeyCallback = cb
		slog.Debug("buildSSHAuth: using configured known_hosts", "path", cfg.SSHKnownHostsPath)
	} else if defaultPath, ok := defaultKnownHostsPath(); ok {
		cb, err := ssh.NewKnownHostsCallback(defaultPath)
		if err != nil {
			slog.Warn("buildSSHAuth: default known_hosts found but could not be loaded; SSH host key verification disabled (MITM risk)",
				"path", defaultPath, "error", err)
			auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
		} else {
			auth.HostKeyCallback = cb
			slog.Info("buildSSHAuth: using default known_hosts for SSH host key verification", "path", defaultPath)
		}
	} else {
		slog.Warn("buildSSHAuth: no known_hosts file configured or found at ~/.ssh/known_hosts; SSH connections will not verify host keys (MITM risk) — set --git-backup-ssh-known-hosts to fix")
		auth.HostKeyCallback = sshcrypto.InsecureIgnoreHostKey()
	}
	return auth, nil
}

// defaultKnownHostsPath returns the path to the user's default known_hosts file
// and whether it actually exists on disk.
func defaultKnownHostsPath() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	p := filepath.Join(home, ".ssh", "known_hosts")
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}
