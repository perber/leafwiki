package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestForcePush_RefusesWhenLocalBehind covers the data-loss path of the
// file-conflict state: the pull was refused, so local HEAD is still an ancestor
// of the remote tip, and a force push would drop the remote commits (and the
// pages they added). ForcePush must refuse, leave the remote alone and keep the
// conflict visible.
func TestForcePush_RefusesWhenLocalBehind(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitFilesToRemote(t, bareDir, map[string]string{
		"root/page.md":     "version B from remote\n",
		"root/incoming.md": "new page from remote\n",
	})
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on the file conflict")
	}
	remoteBefore := remoteHeadCommit(t, bareDir).Hash

	err := NewEnvManager(repo, nil).ForcePush()
	if !errors.Is(err, ErrForcePushRemoteAhead) {
		t.Fatalf("ForcePush error = %v, want ErrForcePushRemoteAhead", err)
	}

	if got := remoteHeadCommit(t, bareDir).Hash; got != remoteBefore {
		t.Errorf("remote head moved from %s to %s; the remote commits were overwritten", remoteBefore, got)
	}
	snap := repo.Status()
	if !snap.NeedsIntervention || snap.ConflictKind != ConflictKindFileConflict {
		t.Errorf("status = %+v, want the file conflict to stay visible", snap)
	}
}

// TestForcePush_DivergedStillOverwrites checks that ForcePush keeps its
// purpose for diverged history: the remote is replaced by local HEAD and the
// conflict state is cleared.
func TestForcePush_DivergedStillOverwrites(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitToRemote(t, bareDir, "root/remote-only.md", "from remote\n")
	commitDirectlyOnRepo(t, repo, rootDir, "local-only.md", "local only\n")
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on diverged history")
	}
	if kind := repo.Status().ConflictKind; kind != ConflictKindDiverged {
		t.Fatalf("ConflictKind = %q, want %q", kind, ConflictKindDiverged)
	}

	if err := repo.ForcePush(); err != nil {
		t.Fatalf("ForcePush failed: %v", err)
	}

	local, err := repo.headCommit()
	if err != nil {
		t.Fatalf("headCommit: %v", err)
	}
	if got := remoteHeadCommit(t, bareDir).Hash; got != local.Hash {
		t.Errorf("remote head = %s, want local head %s", got, local.Hash)
	}
	if snap := repo.Status(); snap.NeedsIntervention || snap.ConflictKind != "" {
		t.Errorf("status = %+v, want the conflict cleared", snap)
	}
}
