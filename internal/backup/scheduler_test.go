package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScheduler_TriggerNow(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create scheduler with a long interval so it won't fire naturally
	scheduler := NewScheduler(repo, 10*time.Minute)
	defer scheduler.Stop()

	// Give it a moment to process the initial run
	time.Sleep(100 * time.Millisecond)

	// TriggerNow should not block
	scheduler.TriggerNow()

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)

	// Verify that status was updated (backup ran)
	if repo.status.LastBackupAt.IsZero() {
		t.Error("expected LastBackupAt to be set after TriggerNow")
	}
}

func TestScheduler_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	scheduler := NewScheduler(repo, 10*time.Minute)

	// Stop should not block
	scheduler.Stop()

	// Verify we can call Stop multiple times safely
	scheduler.Stop()
}

func TestScheduler_RunsOnStart(t *testing.T) {
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")

	err := os.MkdirAll(rootDir, 0755)
	if err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	err = os.MkdirAll(assetsDir, 0755)
	if err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}

	cfg := Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create scheduler with very long interval
	scheduler := NewScheduler(repo, 10*time.Hour)
	defer scheduler.Stop()

	// Wait a moment for the initial run
	time.Sleep(100 * time.Millisecond)

	// Verify that LastBackupAt was set (scheduler ran immediately on start)
	if repo.status.LastBackupAt.IsZero() {
		t.Error("expected scheduler to run immediately on start")
	}
}