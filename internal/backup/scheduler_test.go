package backup

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func waitForBackup(t *testing.T, repo *Repository, timeout time.Duration) time.Time {
	t.Helper()
	deadline := time.After(timeout)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			last := repo.Status().LastBackupAt
			if last != nil && !last.IsZero() {
				return *last
			}
		case <-deadline:
			t.Fatal("timeout waiting for backup")
		}
	}
}

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
		Interval:    10 * time.Minute,
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Add a file BEFORE starting the scheduler so there's something to back up
	if err := os.WriteFile(filepath.Join(rootDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create scheduler with a long interval so it won't fire naturally
	scheduler := NewScheduler(repo)
	defer scheduler.Stop()

	// Wait for the initial run to complete
	initialBackup := waitForBackup(t, repo, 2*time.Second)

	// TriggerNow should not block
	scheduler.TriggerNow()

	// Wait for TriggerNow to be processed
	timeout2 := time.After(2 * time.Second)
	tick2 := time.NewTicker(50 * time.Millisecond)
	defer tick2.Stop()

	for {
		select {
		case <-tick2.C:
			if last := repo.Status().LastBackupAt; last != nil && !last.IsZero() && !last.Equal(initialBackup) {
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
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
		Interval:    10 * time.Minute,
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	scheduler := NewScheduler(repo)

	// Stop should block until goroutine finishes
	scheduler.Stop()

	// Verify we can call Stop multiple times safely
	scheduler.Stop()
}

func makeRepo(t *testing.T, interval time.Duration) *Repository {
	t.Helper()
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatalf("failed to create root dir: %v", err)
	}
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}
	repo, err := Init(Config{
		RootDir:     rootDir,
		AssetsDir:   assetsDir,
		AuthorName:  "Test Author",
		AuthorEmail: "test@example.com",
		Branch:      "main",
		Interval:    interval,
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	return repo
}

func TestScheduler_NegativeInterval_ManualOnly(t *testing.T) {
	repo := makeRepo(t, -5*time.Minute)
	scheduler := NewScheduler(repo)
	defer scheduler.Stop()

	if repo.cfg.Interval != 0 {
		t.Errorf("expected cfg.Interval to be clamped to 0, got %v", repo.cfg.Interval)
	}
	if scheduler.ticker != nil {
		t.Error("expected no ticker in manual-only mode")
	}
}

func TestScheduler_SubMinuteInterval_ClampedToMinimum(t *testing.T) {
	repo := makeRepo(t, 10*time.Second)
	scheduler := NewScheduler(repo)
	defer scheduler.Stop()

	if repo.cfg.Interval != minInterval {
		t.Errorf("expected cfg.Interval to be clamped to %v, got %v", minInterval, repo.cfg.Interval)
	}
	if scheduler.ticker == nil {
		t.Error("expected ticker to be running at minimum interval")
	}
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
		Interval:    600 * time.Minute,
	}

	repo, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Add a file BEFORE starting the scheduler so there's something to back up
	if err := os.WriteFile(filepath.Join(rootDir, "test.txt"), []byte("content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create scheduler with very long interval
	scheduler := NewScheduler(repo)
	defer scheduler.Stop()

	// Wait for the initial run to complete
	waitForBackup(t, repo, 2*time.Second)
}
