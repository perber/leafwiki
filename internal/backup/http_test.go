package backup

import (
	"strings"
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func TestIsHTTPRemote(t *testing.T) {
	tests := []struct {
		remote string
		want   bool
	}{
		{"https://github.com/user/repo.git", true},
		{"http://gitea.internal/user/repo.git", true},
		{"HTTPS://github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", false},
		{"ssh://git@github.com/user/repo.git", false},
		{"file:///tmp/bare", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.remote, func(t *testing.T) {
			if got := isHTTPRemote(tc.remote); got != tc.want {
				t.Fatalf("isHTTPRemote(%q) = %v, want %v", tc.remote, got, tc.want)
			}
		})
	}
}

func TestBuildAuth_HTTPSRemote_ReturnsBasicAuth(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.RemoteURL = "https://github.com/user/repo.git"
	repo.cfg.HTTPUsername = "jochumdev"
	repo.cfg.HTTPPassword = "github_pat_secret"

	auth, err := repo.buildAuth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("expected *http.BasicAuth, got %T", auth)
	}
	if basic.Username != "jochumdev" || basic.Password != "github_pat_secret" {
		t.Fatalf("unexpected credentials: user=%q", basic.Username)
	}
}

// The password must never end up in a log line: go-git masks it in String(),
// and we rely on that when auth methods are logged.
func TestBuildAuth_HTTPSRemote_DoesNotLeakPasswordInString(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.RemoteURL = "https://github.com/user/repo.git"
	repo.cfg.HTTPUsername = "jochumdev"
	repo.cfg.HTTPPassword = "github_pat_secret"

	auth, err := repo.buildAuth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got := auth.String(); got == "" || strings.Contains(got, "github_pat_secret") {
		t.Fatalf("auth.String() leaks the password: %q", got)
	}
}

func TestBuildAuth_HTTPSRemote_MissingOneCredential_ReturnsError(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{"username only", "jochumdev", ""},
		{"password only", "", "github_pat_secret"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := baseRepo(t)
			repo.cfg.RemoteURL = "https://github.com/user/repo.git"
			repo.cfg.HTTPUsername = tc.username
			repo.cfg.HTTPPassword = tc.password

			if _, err := repo.buildAuth(); err == nil {
				t.Fatal("expected an error when only one of username/password is set")
			}
		})
	}
}

// No credentials at all is legal: they may be embedded in the remote URL, or
// the remote may not require auth. go-git treats a nil AuthMethod as "no auth".
func TestBuildAuth_HTTPSRemote_NoCredentials_ReturnsNilAuth(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.RemoteURL = "https://user:token@github.com/user/repo.git"

	auth, err := repo.buildAuth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if auth != nil {
		t.Fatalf("expected nil auth, got %#v", auth)
	}
}

// An HTTPS remote must not require an SSH key — that's the whole point of #1375.
func TestBuildAuth_HTTPSRemote_IgnoresMissingSSHKey(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.RemoteURL = "https://github.com/user/repo.git"
	repo.cfg.SSHKey = ""
	repo.cfg.SSHKeyPath = ""
	repo.cfg.HTTPUsername = "jochumdev"
	repo.cfg.HTTPPassword = "github_pat_secret"

	if _, err := repo.buildAuth(); err != nil {
		t.Fatalf("expected no error without an SSH key, got: %v", err)
	}
}

func TestBuildAuth_SSHRemote_ReturnsPublicKeys(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.RemoteURL = "git@github.com:user/repo.git"
	repo.cfg.SSHKey = testSSHKeyPEM

	auth, err := repo.buildAuth()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, ok := auth.(*ssh.PublicKeys); !ok {
		t.Fatalf("expected *ssh.PublicKeys, got %T", auth)
	}
}

// HTTP credentials must not be used for an SSH remote — the SSH key stays required.
func TestBuildAuth_SSHRemote_HTTPCredentialsDoNotSubstituteForKey(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.RemoteURL = "git@github.com:user/repo.git"
	repo.cfg.SSHKey = ""
	repo.cfg.SSHKeyPath = ""
	repo.cfg.HTTPUsername = "jochumdev"
	repo.cfg.HTTPPassword = "github_pat_secret"

	if _, err := repo.buildAuth(); err == nil {
		t.Fatal("expected an error: an SSH remote still needs an SSH key")
	}
}

func TestRedactRemote(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		want   string
	}{
		{"no credentials", "https://github.com/user/repo.git", "https://github.com/user/repo.git"},
		{"user and password", "https://jochumdev:github_pat_secret@github.com/user/repo.git", "https://jochumdev:xxxxx@github.com/user/repo.git"},
		{"scp-style ssh", "git@github.com:user/repo.git", "git@github.com:user/repo.git"},
		{"ssh url", "ssh://git@github.com/user/repo.git", "ssh://git@github.com/user/repo.git"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := redactRemote(tc.remote); got != tc.want {
				t.Fatalf("redactRemote(%q) = %q, want %q", tc.remote, got, tc.want)
			}
		})
	}
}
