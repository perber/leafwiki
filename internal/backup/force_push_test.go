package backup

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/test_utils"
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

// TestForcePush_RemoteBranchMissing_Pushes checks that ForcePush still creates
// the remote branch when it does not exist yet (nothing there to overwrite).
func TestForcePush_RemoteBranchMissing_Pushes(t *testing.T) {
	bareDir := initBareRemote(t)
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	for _, dir := range []string{rootDir, assetsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}
	repo, err := Init(Config{
		RootDir: rootDir, AssetsDir: assetsDir,
		AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main",
		RemoteURL: test_utils.FileURL(bareDir), SSHKey: testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	commitDirectlyOnRepo(t, repo, rootDir, "page.md", "local only\n")

	if err := repo.ForcePush(); err != nil {
		t.Fatalf("ForcePush: %v", err)
	}
	local, err := repo.headCommit()
	if err != nil {
		t.Fatalf("headCommit: %v", err)
	}
	if got := remoteHeadCommit(t, bareDir).Hash; got != local.Hash {
		t.Errorf("remote head = %s, want local head %s", got, local.Hash)
	}
}

// TestForcePush_UnbornLocal_RemoteExists_Refused: a fresh local backup that
// has never committed is "behind" any existing remote branch; force pushing
// would at best fail and at worst wipe the remote, so it must be refused.
func TestForcePush_UnbornLocal_RemoteExists_Refused(t *testing.T) {
	bareDir := initBareRemote(t)
	tmpDir := t.TempDir()
	rootDir := filepath.Join(tmpDir, "root")
	assetsDir := filepath.Join(tmpDir, "assets")
	for _, dir := range []string{rootDir, assetsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("MkdirAll %s: %v", dir, err)
		}
	}
	repo, err := Init(Config{
		RootDir: rootDir, AssetsDir: assetsDir,
		AuthorName: "T", AuthorEmail: "t@t.com", Branch: "main",
		RemoteURL: test_utils.FileURL(bareDir), SSHKey: testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if head, herr := repo.headCommit(); herr != nil || head != nil {
		t.Fatalf("precondition: expected unborn local HEAD, got head=%v err=%v", head, herr)
	}
	pushFirstCommitToRemote(t, bareDir, "main", "root/remote.md", "remote\n")
	remoteBefore := remoteHeadCommit(t, bareDir).Hash

	if err := repo.ForcePush(); !errors.Is(err, ErrForcePushRemoteAhead) {
		t.Fatalf("ForcePush error = %v, want ErrForcePushRemoteAhead", err)
	}
	if got := remoteHeadCommit(t, bareDir).Hash; got != remoteBefore {
		t.Errorf("remote head moved from %s to %s", remoteBefore, got)
	}
}

// TestForcePush_RepeatedRefusal_KeepsConflictState: clicking Force Push again
// after a refusal must neither overwrite the remote nor clear the visible
// file-conflict state (kind and details).
func TestForcePush_RepeatedRefusal_KeepsConflictState(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	commitToRemote(t, bareDir, "root/page.md", "version B from remote\n")
	if err := os.WriteFile(filepath.Join(rootDir, "page.md"), []byte("version C local\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on the file conflict")
	}
	before := repo.Status()
	remoteBefore := remoteHeadCommit(t, bareDir).Hash

	for i := 0; i < 3; i++ {
		if err := repo.ForcePush(); !errors.Is(err, ErrForcePushRemoteAhead) {
			t.Fatalf("ForcePush #%d error = %v, want ErrForcePushRemoteAhead", i+1, err)
		}
	}

	if got := remoteHeadCommit(t, bareDir).Hash; got != remoteBefore {
		t.Errorf("remote head moved from %s to %s", remoteBefore, got)
	}
	after := repo.Status()
	if after.NeedsIntervention != before.NeedsIntervention ||
		after.ConflictKind != ConflictKindFileConflict ||
		after.ConflictDetails != before.ConflictDetails ||
		after.LastError != before.LastError {
		t.Errorf("status changed by refused force push:\nbefore %+v\nafter  %+v", before, after)
	}
}

// TestForcePush_Diverged_NextCycleIsClean: after Force Push resolved a
// divergence, the following backup cycle must succeed normally (no stale
// ConflictKind, the new page reaches the remote).
func TestForcePush_Diverged_NextCycleIsClean(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)
	commitToRemote(t, bareDir, "root/remote-only.md", "from remote\n")
	commitDirectlyOnRepo(t, repo, rootDir, "local-only.md", "local only\n")
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on diverged history")
	}
	if err := repo.ForcePush(); err != nil {
		t.Fatalf("ForcePush: %v", err)
	}

	if err := os.WriteFile(filepath.Join(rootDir, "after.md"), []byte("after force push\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := repo.RunBackup(); err != nil {
		t.Fatalf("RunBackup after force push: %v", err)
	}
	snap := repo.Status()
	if snap.NeedsIntervention || snap.ConflictKind != "" || snap.LastError != "" {
		t.Errorf("status = %+v, want clean", snap)
	}
	if _, err := remoteHeadCommit(t, bareDir).File("root/after.md"); err != nil {
		t.Errorf("root/after.md missing on the remote after the next cycle: %v", err)
	}
	local, err := repo.headCommit()
	if err != nil {
		t.Fatalf("headCommit: %v", err)
	}
	if got := remoteHeadCommit(t, bareDir).Hash; got != local.Hash {
		t.Errorf("remote head = %s, want local head %s", got, local.Hash)
	}
}

// TestForcePush_RemoteMovedAfterCompare_Refused checks the lease: a commit
// pushed to the remote between ForcePush's comparison and its push must not be
// overwritten unseen.
func TestForcePush_RemoteMovedAfterCompare_Refused(t *testing.T) {
	bareDir := initBareRemote(t)
	repo, rootDir := newRepoWithRemote(t, bareDir)

	commitToRemote(t, bareDir, "root/remote-only.md", "from remote\n")
	commitDirectlyOnRepo(t, repo, rootDir, "local-only.md", "local only\n")
	if err := repo.RunBackup(); err == nil {
		t.Fatal("expected RunBackup to fail on diverged history")
	}

	repo.beforeForcePush = func() {
		commitToRemote(t, bareDir, "root/late.md", "pushed after the comparison\n")
	}
	err := repo.ForcePush()
	if !errors.Is(err, ErrForcePushRemoteAhead) {
		t.Fatalf("ForcePush error = %v, want ErrForcePushRemoteAhead", err)
	}
	if _, fileErr := remoteHeadCommit(t, bareDir).File("root/late.md"); fileErr != nil {
		t.Errorf("remote lost the late commit: %v", fileErr)
	}
	if kind := repo.Status().ConflictKind; kind != ConflictKindDiverged {
		t.Errorf("ConflictKind = %q, want the divergence to stay visible", kind)
	}

	// Without a concurrent push, the next force push goes through.
	repo.beforeForcePush = nil
	if err := repo.ForcePush(); err != nil {
		t.Fatalf("second ForcePush failed: %v", err)
	}
}
