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
		RootDir:       rootDir,
		AssetsDir:     assetsDir,
		AuthorName:    "Test Author",
		AuthorEmail:   "test@example.com",
		Branch:        "main",
		IntervalMinutes: 10,
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create scheduler with a long interval so it won't fire naturally
	scheduler := NewScheduler(repo, &cfg)
	defer scheduler.Stop()

	// Add a file to back up so there's something to commit
	if err := os.WriteFile(filepath.Join(rootDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for the initial run to complete using a channel
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !repo.Status().LastBackupAt.IsZero() {
				goto afterInitialRun
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial run")
		}
	}

afterInitialRun:
	// TriggerNow should not block
	scheduler.TriggerNow()

	// Wait for TriggerNow to be processed
	timeout2 := time.After(2 * time.Second)
	ticker2 := time.NewTicker(50 * time.Millisecond)
	defer ticker2.Stop()

	initialBackup := repo.Status().LastBackupAt
	for {
		select {
		case <-ticker2.C:
			if !repo.Status().LastBackupAt.IsZero() && !repo.Status().LastBackupAt.Equal(initialBackup) {
				return // Success
			}
		case <-timeout2:
			t.Fatal("timeout waiting for TriggerNow")
		}
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
		RootDir:       rootDir,
		AssetsDir:     assetsDir,
		AuthorName:    "Test Author",
		AuthorEmail:   "test@example.com",
		Branch:        "main",
		IntervalMinutes: 10,
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	scheduler := NewScheduler(repo, &cfg)

	// Stop should block until goroutine finishes
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
		RootDir:       rootDir,
		AssetsDir:     assetsDir,
		AuthorName:    "Test Author",
		AuthorEmail:   "test@example.com",
		Branch:        "main",
		IntervalMinutes: 600,
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create scheduler with very long interval
	scheduler := NewScheduler(repo, &cfg)
	defer scheduler.Stop()

	// Add a file to back up so there's something to commit
	if err := os.WriteFile(filepath.Join(rootDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for the initial run to complete using a channel
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !repo.Status().LastBackupAt.IsZero() {
				return // Success
			}
		case <-timeout:
			t.Fatal("timeout waiting for initial run")
		}
	}
}