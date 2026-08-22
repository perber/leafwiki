package backup

import (
	"crypto/subtle"
	"net/http"
	"net/http/cgi"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	testHTTPUser  = "leafwiki"
	testHTTPToken = "github_pat_testtoken"
)

// startGitHTTPServer serves a bare repo over the git smart HTTP protocol behind
// basic auth, using the git-http-backend CGI that ships with git itself.
// Returns the base URL of the served repository.
//
// The test is skipped when git (or its CGI backend) is unavailable, so the suite
// still runs on machines without a git installation.
func startGitHTTPServer(t *testing.T, bareDir string) string {
	t.Helper()

	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git binary not available, skipping git HTTP integration test")
	}
	execPathOut, err := exec.Command(gitBin, "--exec-path").Output()
	if err != nil {
		t.Skipf("could not determine git exec-path: %v", err)
	}
	backend := filepath.Join(strings.TrimSpace(string(execPathOut)), "git-http-backend")
	if _, err := os.Stat(backend); err != nil {
		t.Skipf("git-http-backend not available at %s, skipping", backend)
	}

	// git refuses to serve a repo it doesn't consider "exported" unless
	// GIT_HTTP_EXPORT_ALL is set, and it rejects pushes to a non-bare repo
	// over HTTP unless http.receivepack is enabled.
	if out, err := exec.Command(gitBin, "-C", bareDir, "config", "http.receivepack", "true").CombinedOutput(); err != nil {
		t.Fatalf("git config http.receivepack: %v: %s", err, out)
	}

	handler := &cgi.Handler{
		Path: backend,
		Env: []string{
			"GIT_PROJECT_ROOT=" + filepath.Dir(bareDir),
			"GIT_HTTP_EXPORT_ALL=1",
		},
	}

	authenticated := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(user), []byte(testHTTPUser)) != 1 ||
			subtle.ConstantTimeCompare([]byte(pass), []byte(testHTTPToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="git"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(authenticated)
	t.Cleanup(server.Close)

	return server.URL + "/" + filepath.Base(bareDir)
}

// newHTTPRepoWithRemote builds a Repository configured for an HTTP remote with
// basic auth and no SSH key at all.
func newHTTPRepoWithRemote(t *testing.T, remoteURL, username, password string) (*Repository, string) {
	t.Helper()
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("# Page\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo, err := Init(Config{
		RootDir:      rootDir,
		AssetsDir:    assetsDir,
		AuthorName:   "Test Author",
		AuthorEmail:  "test@example.com",
		Branch:       "main",
		RemoteURL:    remoteURL,
		HTTPUsername: username,
		HTTPPassword: password,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return repo, rootDir
}

// TestRunBackup_HTTPBasicAuth_PushesToRemote is the end-to-end proof for #1375:
// a backup authenticated with username + token over HTTP reaches the remote,
// with no SSH key configured anywhere.
func TestRunBackup_HTTPBasicAuth_PushesToRemote(t *testing.T) {
	bareDir := initBareRemote(t)
	remoteURL := startGitHTTPServer(t, bareDir)

	repo, rootDir := newHTTPRepoWithRemote(t, remoteURL, testHTTPUser, testHTTPToken)
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	assertRemoteHasFile(t, bareDir, "root/page.md")

	// A second cycle must push the follow-up commit too (auth is rebuilt per call).
	if err := os.WriteFile(filepath.Join(rootDir, "second.md"), []byte("# Second\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("second RunBackup: %v", err)
	}
	assertRemoteHasFile(t, bareDir, "root/second.md")
}

// TestInit_HTTPBasicAuth_WrongPasswordFails guards against the credentials being
// silently ignored: a bad token must surface as an error rather than a backup
// that quietly never reaches the remote. Init already talks to the remote (it
// adopts its history), so that is where a wrong token is rejected.
func TestInit_HTTPBasicAuth_WrongPasswordFails(t *testing.T) {
	bareDir := initBareRemote(t)
	remoteURL := startGitHTTPServer(t, bareDir)
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll root: %v", err)
	}

	_, err := Init(Config{
		RootDir:      rootDir,
		AssetsDir:    filepath.Join(tmpDir, "assets"),
		AuthorName:   "Test Author",
		AuthorEmail:  "test@example.com",
		Branch:       "main",
		RemoteURL:    remoteURL,
		HTTPUsername: testHTTPUser,
		HTTPPassword: "wrong-token",
	})
	if err == nil {
		t.Fatal("expected Init to fail with invalid credentials")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Fatalf("expected an authentication error, got: %v", err)
	}
}

// TestRunBackup_HTTPBasicAuth_CredentialsRevokedFails covers a token that stops
// working after the repo was set up: the backup must fail loudly.
func TestRunBackup_HTTPBasicAuth_CredentialsRevokedFails(t *testing.T) {
	bareDir := initBareRemote(t)
	remoteURL := startGitHTTPServer(t, bareDir)

	repo, rootDir := newHTTPRepoWithRemote(t, remoteURL, testHTTPUser, testHTTPToken)
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup: %v", err)
	}

	repo.cfg.HTTPPassword = "revoked-token"
	if err := os.WriteFile(filepath.Join(rootDir, "third.md"), []byte("# Third\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail with revoked credentials")
	}
	if status := repo.Status(); status.LastError == "" {
		t.Fatal("expected the failure to be recorded in the backup status")
	}
}

// assertRemoteHasFile checks that the bare remote's main branch contains path.
func assertRemoteHasFile(t *testing.T, bareDir, path string) {
	t.Helper()
	bare, err := gogit.PlainOpen(bareDir)
	if err != nil {
		t.Fatalf("PlainOpen bare: %v", err)
	}
	ref, err := bare.Reference(plumbing.NewBranchReferenceName("main"), true)
	if err != nil {
		t.Fatalf("remote has no main branch: %v", err)
	}
	commit, err := bare.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("CommitObject: %v", err)
	}
	if _, err := commit.File(path); err != nil {
		t.Fatalf("expected %s on the remote, got: %v", path, err)
	}
}
