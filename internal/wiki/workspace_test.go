package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorkspace_AllowsDefaultRootUnderDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")

	if err := ValidateWorkspace(DefaultWorkspace(dataDir)); err != nil {
		t.Fatalf("ValidateWorkspace default root failed: %v", err)
	}
}

func TestValidateWorkspace_RejectsRootDirContainingDataDir(t *testing.T) {
	rootDir := filepath.Join(t.TempDir(), "wiki")
	dataDir := filepath.Join(rootDir, "data")

	err := ValidateWorkspace(Workspace{ID: "default", DataDir: dataDir, RootDir: rootDir})
	if err == nil {
		t.Fatalf("expected root dir containing data dir to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must not contain data dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWorkspace_RejectsRootDirInsideReservedDataDirState(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")
	rootDir := filepath.Join(dataDir, "assets", "pages")

	err := ValidateWorkspace(Workspace{ID: "default", DataDir: dataDir, RootDir: rootDir})
	if err == nil {
		t.Fatalf("expected root dir inside reserved app state to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must not be inside data dir app state") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWorkspace_RejectsRootDirSymlinkContainingDataDir(t *testing.T) {
	baseDir := t.TempDir()
	rootTarget := filepath.Join(baseDir, "wiki")
	dataDir := filepath.Join(rootTarget, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("mkdir data dir: %v", err)
	}
	rootLink := filepath.Join(baseDir, "root-link")
	if err := os.Symlink(rootTarget, rootLink); err != nil {
		t.Fatalf("symlink root target: %v", err)
	}

	err := ValidateWorkspace(Workspace{ID: "default", DataDir: dataDir, RootDir: rootLink})
	if err == nil {
		t.Fatalf("expected symlinked root containing data dir to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must not contain data dir") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWorkspace_RejectsRootDirSymlinkInsideReservedDataDirState(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "data")
	rootTarget := filepath.Join(dataDir, "assets", "pages")
	if err := os.MkdirAll(rootTarget, 0o755); err != nil {
		t.Fatalf("mkdir root target: %v", err)
	}
	rootLink := filepath.Join(baseDir, "root-link")
	if err := os.Symlink(rootTarget, rootLink); err != nil {
		t.Fatalf("symlink root target: %v", err)
	}

	err := ValidateWorkspace(Workspace{ID: "default", DataDir: dataDir, RootDir: rootLink})
	if err == nil {
		t.Fatalf("expected symlinked root inside app state to be rejected")
	}
	if !strings.Contains(err.Error(), "root dir must not be inside data dir app state") {
		t.Fatalf("unexpected error: %v", err)
	}
}
