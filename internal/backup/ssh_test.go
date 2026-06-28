package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func baseRepo(t *testing.T) *Repository {
	t.Helper()
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("MkdirAll rootDir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("MkdirAll assetsDir: %v", err)
	}
	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "T",
		AuthorEmail: "t@t.com",
		Branch:      "main",
	}
	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("baseRepo: Init failed: %v", err)
	}
	return repo
}

func TestBuildSSHAuth_NoKeyConfigured_ReturnsError(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.SSHKey = ""
	repo.cfg.SSHKeyPath = ""

	_, err := repo.buildSSHAuth()
	if err == nil {
		t.Fatal("expected error when neither SSHKey nor SSHKeyPath is set")
	}
}

func TestBuildSSHAuth_InvalidPEM_ReturnsError(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.SSHKey = "not a valid PEM key"

	_, err := repo.buildSSHAuth()
	if err == nil {
		t.Fatal("expected error for invalid PEM data")
	}
}

func TestBuildSSHAuth_SSHKeyPathNotFound_ReturnsError(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.SSHKeyPath = "/nonexistent/path/id_ed25519"

	_, err := repo.buildSSHAuth()
	if err == nil {
		t.Fatal("expected error when SSHKeyPath does not exist")
	}
}

func TestBuildSSHAuth_ValidInlineKey_ReturnsAuth(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.SSHKey = testSSHKeyPEM

	auth, err := repo.buildSSHAuth()
	if err != nil {
		t.Fatalf("expected no error for valid inline key, got: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
}

func TestBuildSSHAuth_ValidKeyFile_ReturnsAuth(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(keyFile, []byte(testSSHKeyPEM), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := baseRepo(t)
	repo.cfg.SSHKeyPath = keyFile

	auth, err := repo.buildSSHAuth()
	if err != nil {
		t.Fatalf("expected no error for valid key file, got: %v", err)
	}
	if auth == nil {
		t.Fatal("expected non-nil auth")
	}
}

func TestBuildSSHAuth_KnownHostsPathMissing_ReturnsError(t *testing.T) {
	repo := baseRepo(t)
	repo.cfg.SSHKey = testSSHKeyPEM
	repo.cfg.SSHKnownHostsPath = "/nonexistent/known_hosts"

	_, err := repo.buildSSHAuth()
	if err == nil {
		t.Fatal("expected error when SSHKnownHostsPath is configured but file does not exist")
	}
}
