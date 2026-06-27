package backup

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
)

// --- hasFiles ---

func TestHasFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	got, err := hasFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false for empty dir")
	}
}

func TestHasFiles_DirWithFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "file.md", "content")
	got, err := hasFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true for dir with file")
	}
}

func TestHasFiles_OnlyEmptySubdirs(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, dir, "sub/nested")
	got, err := hasFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false for dir with only empty subdirs")
	}
}

func TestHasFiles_FileInSubdir(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, dir, "sub")
	writeFile(t, dir, "sub/deep.md", "content")
	got, err := hasFiles(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true for file in subdirectory")
	}
}

func TestHasFiles_NonExistentDir(t *testing.T) {
	_, err := hasFiles("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// --- hasStagedChanges ---

func TestHasStagedChanges_EmptyStatus(t *testing.T) {
	if hasStagedChanges(gogit.Status{}) {
		t.Error("expected false for empty status")
	}
}

func TestHasStagedChanges_OnlyUntracked(t *testing.T) {
	s := gogit.Status{
		"outside.txt": &gogit.FileStatus{Staging: gogit.Untracked, Worktree: gogit.Untracked},
	}
	if hasStagedChanges(s) {
		t.Error("expected false: untracked files must not trigger a commit")
	}
}

func TestHasStagedChanges_Added(t *testing.T) {
	s := gogit.Status{"new.md": &gogit.FileStatus{Staging: gogit.Added}}
	if !hasStagedChanges(s) {
		t.Error("expected true for Added file")
	}
}

func TestHasStagedChanges_Modified(t *testing.T) {
	s := gogit.Status{"page.md": &gogit.FileStatus{Staging: gogit.Modified}}
	if !hasStagedChanges(s) {
		t.Error("expected true for Modified file")
	}
}

func TestHasStagedChanges_Deleted(t *testing.T) {
	s := gogit.Status{"gone.md": &gogit.FileStatus{Staging: gogit.Deleted}}
	if !hasStagedChanges(s) {
		t.Error("expected true for Deleted file")
	}
}

func TestHasStagedChanges_MixUntrackedAndModified(t *testing.T) {
	s := gogit.Status{
		"outside.txt": &gogit.FileStatus{Staging: gogit.Untracked},
		"page.md":     &gogit.FileStatus{Staging: gogit.Modified},
	}
	if !hasStagedChanges(s) {
		t.Error("expected true: modified file alongside untracked")
	}
}

// --- Status ---

func TestStatus_SetSuccessClearsAllFields(t *testing.T) {
	s := &Status{}
	s.SetNeedsIntervention("conflict details")
	s.SetSuccess(time.Now())

	snap := s.Snapshot()
	if snap.LastError != "" {
		t.Errorf("expected empty LastError, got %q", snap.LastError)
	}
	if snap.NeedsIntervention {
		t.Error("expected NeedsIntervention to be cleared by SetSuccess")
	}
	if snap.ConflictDetails != "" {
		t.Errorf("expected empty ConflictDetails, got %q", snap.ConflictDetails)
	}
	if snap.LastBackupAt == nil {
		t.Error("expected LastBackupAt to be set")
	}
}

func TestStatus_SetNeedsInterventionSetsAllFields(t *testing.T) {
	s := &Status{}
	s.SetNeedsIntervention("some conflict")

	snap := s.Snapshot()
	if !snap.NeedsIntervention {
		t.Error("expected NeedsIntervention = true")
	}
	if snap.ConflictDetails != "some conflict" {
		t.Errorf("expected ConflictDetails %q, got %q", "some conflict", snap.ConflictDetails)
	}
	if snap.LastError != "some conflict" {
		t.Errorf("expected LastError %q, got %q", "some conflict", snap.LastError)
	}
}

func TestStatus_SnapshotZeroTimeReturnsNilPointer(t *testing.T) {
	s := &Status{}
	snap := s.Snapshot()
	if snap.LastBackupAt != nil {
		t.Error("expected nil LastBackupAt for zero-value Status")
	}
}

func TestStatus_ConcurrentAccess(t *testing.T) {
	s := &Status{}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.SetSuccess(time.Now())
		}()
		go func() {
			defer wg.Done()
			_ = s.Snapshot()
		}()
	}
	wg.Wait()
}

// --- helpers ---

func writeFile(t *testing.T, base, rel, content string) {
	t.Helper()
	p := filepath.Join(base, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatalf("writeFile MkdirAll %s: %v", rel, err)
	}
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeFile %s: %v", rel, err)
	}
}

func mkdirAll(t *testing.T, base, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(rel)), 0755); err != nil {
		t.Fatalf("mkdirAll %s: %v", rel, err)
	}
}
