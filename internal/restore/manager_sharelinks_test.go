package restore

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/sharing"
	snapshotSvc "github.com/perber/wiki/internal/snapshot"
	"github.com/perber/wiki/internal/test_utils"
)

// anyPage accepts every page ID, so share links can be created without a tree.
type anyPage struct{}

func (anyPage) FindPageByID(id string) (*tree.PageNode, error) {
	return &tree.PageNode{ID: id}, nil
}

func newShareLinkService(t *testing.T, dataDir string) *sharing.ShareLinkService {
	t.Helper()
	store, err := sharing.NewShareLinkStore(dataDir, nil)
	if err != nil {
		t.Fatalf("NewShareLinkStore failed: %v", err)
	}
	svc := sharing.NewShareLinkService(store, anyPage{}, nil)
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(svc.Close, t) })
	return svc
}

func createShareLink(t *testing.T, svc *sharing.ShareLinkService, pageID string) string {
	t.Helper()
	_, token, err := svc.Create(context.Background(), sharing.CreateShareLinkInput{PageID: pageID, CreatedBy: "editor-1"})
	if err != nil {
		t.Fatalf("Create share link failed: %v", err)
	}
	return token
}

// A snapshot captures share_links.db and a live restore swaps it in: links
// from the snapshot work afterwards (same tokens), links created only on the
// live instance are gone, as with every other state store (ADR-0015).
func TestManager_Restore_HappyPath_PreservesShareLinks(t *testing.T) {
	src := t.TempDir()
	test_utils.WriteFile(t, src, "root/welcome.md", "# Snapshot content\n")
	createRealUsersDB(t, src, "snapshot-admin", "snapshot-admin@example.com", "snapshot-password-123")
	snapshotLinks := newShareLinkService(t, src)
	snapshotToken := createShareLink(t, snapshotLinks, "snapshot-page")

	snapshotMgr := snapshotSvc.NewManager(snapshotSvc.Config{
		BackupsDir:   filepath.Join(src, "backups"),
		RootDir:      filepath.Join(src, "root"),
		UsersDBPath:  filepath.Join(src, "users.db"),
		StateDBPaths: []string{filepath.Join(src, "share_links.db")},
		WikiVersion:  "v1.0.0",
	})
	if err := snapshotMgr.RunOnce(context.Background()); err != nil {
		t.Fatalf("failed to build fixture snapshot: %v", err)
	}
	entries, err := snapshotMgr.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("expected 1 fixture snapshot, got %v (err=%v)", entries, err)
	}

	dataDir := t.TempDir()
	createRealUsersDB(t, dataDir, "live-admin", "live-admin@example.com", "live-password-123")
	liveLinks := newShareLinkService(t, dataDir)
	liveToken := createShareLink(t, liveLinks, "live-page")

	manager := NewManager(Config{
		SnapshotManager: snapshotMgr,
		DataDir:         dataDir,
		WikiVersion:     "v1.0.0",
		WriteGate:       NewWriteGate(),
		Stores:          []LiveStore{{File: "share_links.db", Store: liveLinks}},
	})
	if err := manager.TriggerRestore(entries[0].ID); err != nil {
		t.Fatalf("TriggerRestore failed: %v", err)
	}
	if status := waitForRestoreDone(t, manager); status.Error != "" {
		t.Fatalf("expected successful restore, got error: %s", status.Error)
	}

	grant, err := liveLinks.Resolve(context.Background(), snapshotToken)
	if err != nil {
		t.Fatalf("snapshot link must resolve after restore: %v", err)
	}
	if grant.PageID != "snapshot-page" {
		t.Errorf("grant.PageID = %q, want snapshot-page", grant.PageID)
	}
	if _, err := liveLinks.Resolve(context.Background(), liveToken); err == nil {
		t.Error("a link that only existed on the live instance must be gone after restore")
	}
}
