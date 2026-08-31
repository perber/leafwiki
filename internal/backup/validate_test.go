package backup

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestTestRemote_ReachableRemote_Succeeds(t *testing.T) {
	bare := initBareRemote(t)
	// Push something so the remote is non-empty.
	_, _ = newRepoWithRemote(t, bare)

	err := TestRemote(context.Background(), Config{
		RemoteURL: "file://" + bare,
		SSHKey:    testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("TestRemote on a reachable remote: %v", err)
	}
}

func TestTestRemote_EmptyRemote_TreatedAsSuccess(t *testing.T) {
	bare := initBareRemote(t) // freshly init'd, no commits

	err := TestRemote(context.Background(), Config{
		RemoteURL: "file://" + bare,
		SSHKey:    testSSHKeyPEM,
	})
	if err != nil {
		t.Fatalf("TestRemote on an empty remote should succeed, got: %v", err)
	}
}

func TestTestRemote_NonexistentRemote_Fails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.git")

	err := TestRemote(context.Background(), Config{
		RemoteURL: "file://" + missing,
		SSHKey:    testSSHKeyPEM,
	})
	if err == nil {
		t.Fatal("TestRemote on a nonexistent remote should fail")
	}
}

func TestTestRemote_NoRemoteURL_Fails(t *testing.T) {
	if err := TestRemote(context.Background(), Config{}); err == nil {
		t.Fatal("TestRemote with no RemoteURL should fail")
	}
}

func TestTestRemote_SSHRemoteWithoutKey_FailsBeforeDialing(t *testing.T) {
	err := TestRemote(context.Background(), Config{
		RemoteURL: "git@github.com:example/repo.git",
	})
	if err == nil {
		t.Fatal("TestRemote for an SSH remote with no key should fail")
	}
}

func TestTestRemote_CancelledContext_Fails(t *testing.T) {
	bare := initBareRemote(t)
	_, _ = newRepoWithRemote(t, bare)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	if err := TestRemote(ctx, Config{RemoteURL: "file://" + bare, SSHKey: testSSHKeyPEM}); err == nil {
		t.Fatal("TestRemote should fail when the context is already expired")
	}
}
