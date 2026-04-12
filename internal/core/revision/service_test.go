package revision

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

func newRevisionTestService(t *testing.T) (*Service, *tree.TreeService, string) {
	t.Helper()
	storageDir := t.TempDir()
	treeService := tree.NewTreeService(storageDir)
	if err := treeService.LoadTree(); err != nil {
		t.Fatalf("LoadTree failed: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewService(storageDir, treeService, logger), treeService, storageDir
}

func createRevisionTestPage(t *testing.T, treeService *tree.TreeService, title, slug, content string) string {
	t.Helper()
	kind := tree.NodeKindPage
	id, err := treeService.CreateNode("tester", nil, title, slug, &kind)
	if err != nil {
		t.Fatalf("CreateNode failed: %v", err)
	}
	if err := treeService.UpdateNode("tester", *id, title, slug, &content); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}
	return *id
}

func writeLiveAsset(t *testing.T, storageDir, pageID, name, content string) {
	t.Helper()
	dir := filepath.Join(storageDir, "assets", pageID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll asset dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile asset failed: %v", err)
	}
}

func TestRecordContentUpdateHappyPathAndNoop(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	rev, created, err := service.RecordContentUpdate(pageID, "tester", "first")
	if err != nil {
		t.Fatalf("RecordContentUpdate failed: %v", err)
	}
	if !created || rev == nil {
		t.Fatalf("expected revision to be created")
	}
	if rev.Type != RevisionTypeContentUpdate || rev.ParentID != "" {
		t.Fatalf("unexpected revision: %#v", rev)
	}
	if rev.AssetManifestHash == "" || rev.ContentHash == "" {
		t.Fatalf("expected hashes on revision: %#v", rev)
	}
	if rev.PageCreatedAt.IsZero() || rev.PageUpdatedAt.IsZero() {
		t.Fatalf("expected page metadata timestamps on revision")
	}

	rev2, created2, err := service.RecordContentUpdate(pageID, "tester", "second")
	if err != nil {
		t.Fatalf("RecordContentUpdate second call failed: %v", err)
	}
	if created2 {
		t.Fatalf("expected second content update to be skipped")
	}
	if rev2.ID != rev.ID {
		t.Fatalf("expected same revision on noop, got %q vs %q", rev2.ID, rev.ID)
	}
}

func TestRecordAssetStructureDeleteAndRestoreHappyPath(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	parentKind := tree.NodeKindSection
	parentID, err := treeService.CreateNode("tester", nil, "Docs", "docs", &parentKind)
	if err != nil {
		t.Fatalf("CreateNode(parent) failed: %v", err)
	}
	kind := tree.NodeKindPage
	pageIDPtr, err := treeService.CreateNode("tester", parentID, "Child", "child", &kind)
	if err != nil {
		t.Fatalf("CreateNode(page) failed: %v", err)
	}
	pageID := *pageIDPtr
	content := "hello"
	if err := treeService.UpdateNode("tester", pageID, "Child", "child", &content); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	assetRev, created, err := service.RecordAssetChange(pageID, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}
	if !created || assetRev.Type != RevisionTypeAssetUpdate {
		t.Fatalf("unexpected asset revision: %#v created=%v", assetRev, created)
	}
	if assetRev.ParentID != *parentID {
		t.Fatalf("asset revision parent id = %q, want %q", assetRev.ParentID, *parentID)
	}

	if _, _, err := service.RecordStructureChange(pageID, "tester", "structure"); err != nil {
		t.Fatalf("RecordStructureChange failed: %v", err)
	}

	if _, _, err := service.RecordDelete(pageID, "tester", "delete"); err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}

	rootParent := "root"
	if err := service.RestorePage(pageID, "tester", &rootParent); err != nil {
		t.Fatalf("RestorePage failed: %v", err)
	}

	page, err := treeService.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if page.Content != content {
		t.Fatalf("restored content = %q, want %q", page.Content, content)
	}
	assetBytes, err := os.ReadFile(filepath.Join(storageDir, "assets", pageID, "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile restored asset failed: %v", err)
	}
	if string(assetBytes) != "asset-a" {
		t.Fatalf("restored asset = %q", string(assetBytes))
	}
	latest, err := service.GetLatestRevision(pageID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != RevisionTypeRestore {
		t.Fatalf("latest revision = %#v", latest)
	}
}

func TestRestorePageFailedPaths(t *testing.T) {
	service, treeService, _ := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")

	if err := service.RestorePage("missing", "tester", nil); err == nil {
		t.Fatalf("expected missing trash restore to fail")
	}

	contentRev, _, err := service.RecordContentUpdate(pageID, "tester", "content")
	if err != nil {
		t.Fatalf("RecordContentUpdate failed: %v", err)
	}
	if err := service.store.SaveTrashEntry(&TrashEntry{PageID: pageID, DeletedAt: time.Now().UTC(), DeletedBy: "tester", Title: "Page", Slug: "page", Path: "/page", LastRevisionID: contentRev.ID}); err != nil {
		t.Fatalf("SaveTrashEntry failed: %v", err)
	}
	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}

	err = service.RestorePage(pageID, "tester", nil)
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_invalid_revision" {
		t.Fatalf("expected invalid revision error, got %#v (%v)", localized, err)
	}

	service2, treeService2, storageDir2 := newRevisionTestService(t)
	parentKind := tree.NodeKindSection
	parentID, err := treeService2.CreateNode("tester", nil, "Docs", "docs", &parentKind)
	if err != nil {
		t.Fatalf("CreateNode(parent) failed: %v", err)
	}
	pageID2 := createRevisionTestPage(t, treeService2, "Child", "child", "hello")
	if err := treeService2.MoveNode("tester", pageID2, *parentID); err != nil {
		t.Fatalf("MoveNode failed: %v", err)
	}
	writeLiveAsset(t, storageDir2, pageID2, "a.txt", "asset-a")
	_, trash, err := service2.RecordDelete(pageID2, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService2.DeleteNode("tester", pageID2, false); err != nil {
		t.Fatalf("DeleteNode(page) failed: %v", err)
	}
	if err := treeService2.DeleteNode("tester", *parentID, true); err != nil {
		t.Fatalf("DeleteNode(parent) failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir2, "assets", pageID2)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}

	err = service2.RestorePage(pageID2, "tester", nil)
	localized, ok = sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_parent_required" {
		t.Fatalf("expected parent required error, got %#v (%v), trash=%#v", localized, err, trash)
	}

	service3, treeService3, _ := newRevisionTestService(t)
	pageID3 := createRevisionTestPage(t, treeService3, "Bad Kind", "bad-kind", "hello")
	_, trash3, err := service3.RecordDelete(pageID3, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService3.DeleteNode("tester", pageID3, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	rev3, err := service3.store.GetRevision(pageID3, trash3.LastRevisionID)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	rev3.Kind = "mystery"
	if err := service3.store.SaveRevision(rev3); err != nil {
		t.Fatalf("SaveRevision override failed: %v", err)
	}
	err = service3.RestorePage(pageID3, "tester", nil)
	localized, ok = sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_invalid_kind" {
		t.Fatalf("expected invalid kind error, got %#v (%v)", localized, err)
	}
}

func TestLocalizedErrorHelpers(t *testing.T) {
	cause := errors.New("boom")
	err := sharederrors.NewLocalizedError("code", "message", "template %s", cause, "arg")
	if err.Error() == "" {
		t.Fatalf("expected non-empty error string")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause")
	}
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "code" || localized.Args[0] != "arg" {
		t.Fatalf("localized = %#v", localized)
	}
	if _, ok := sharederrors.AsLocalizedError(errors.New("plain")); ok {
		t.Fatalf("plain error should not unwrap to LocalizedError")
	}
}

func TestServiceWrappersAndHelpers(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	state, err := service.CapturePageState(pageID)
	if err != nil {
		t.Fatalf("CapturePageState failed: %v", err)
	}
	if state.PageID != pageID || len(state.Assets) != 1 {
		t.Fatalf("state = %#v", state)
	}

	if _, _, err := service.RecordContentUpdate(pageID, "tester", "content"); err != nil {
		t.Fatalf("RecordContentUpdate failed: %v", err)
	}
	if _, _, err := service.RecordDelete(pageID, "tester", "delete"); err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}

	revisions, err := service.ListRevisions(pageID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revisions) < 2 {
		t.Fatalf("expected revisions, got %#v", revisions)
	}
	paged, _, err := service.ListRevisionsPage(pageID, "", 1)
	if err != nil {
		t.Fatalf("ListRevisionsPage failed: %v", err)
	}
	if len(paged) != 1 {
		t.Fatalf("expected one paged revision, got %d", len(paged))
	}
	trash, err := service.GetTrashEntry(pageID)
	if err != nil {
		t.Fatalf("GetTrashEntry failed: %v", err)
	}
	if trash.PageID != pageID {
		t.Fatalf("trash = %#v", trash)
	}
	allTrash, err := service.ListTrash()
	if err != nil {
		t.Fatalf("ListTrash failed: %v", err)
	}
	if len(allTrash) != 1 {
		t.Fatalf("expected one trash entry, got %d", len(allTrash))
	}

	if err := service.DeletePageData(pageID); err != nil {
		t.Fatalf("DeletePageData failed: %v", err)
	}
	revisions, err = service.ListRevisions(pageID)
	if err != nil {
		t.Fatalf("ListRevisions after delete failed: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("expected revisions to be deleted, got %#v", revisions)
	}
	if _, err := service.GetTrashEntry(pageID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected trash entry to be deleted, got %v", err)
	}

	if err := service.persistLiveAssets(pageID, nil); err != nil {
		t.Fatalf("persistLiveAssets(nil) failed: %v", err)
	}
	if _, err := service.scanLiveAssets("missing"); err != nil {
		t.Fatalf("scanLiveAssets(missing) failed: %v", err)
	}
}

func TestRestoreAndMappingHelpers(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	kind := tree.NodeKindSection
	parentID, err := treeService.CreateNode("tester", nil, "Docs", "docs", &kind)
	if err != nil {
		t.Fatalf("CreateNode(parent) failed: %v", err)
	}
	if err := treeService.MoveNode("tester", pageID, *parentID); err != nil {
		t.Fatalf("MoveNode failed: %v", err)
	}

	resolved, err := service.resolveRestoreParentID(pageID, *parentID, "/docs/page", nil)
	if err != nil {
		t.Fatalf("resolveRestoreParentID with stored parent failed: %v", err)
	}
	if resolved == nil || *resolved != *parentID {
		t.Fatalf("resolved parent = %#v", resolved)
	}

	rootParent := "root"
	resolvedRoot, err := service.resolveRestoreParentID(pageID, "", "/page", &rootParent)
	if err != nil || resolvedRoot != nil {
		t.Fatalf("resolveRestoreParentID root override = %#v, %v", resolvedRoot, err)
	}

	if got := restoreParentRoutePath("/docs/page"); got != "docs" {
		t.Fatalf("restoreParentRoutePath = %q", got)
	}
	if _, err := restoreNodeKind("page"); err != nil {
		t.Fatalf("restoreNodeKind(page) failed: %v", err)
	}
	if _, err := restoreNodeKind("weird"); err == nil {
		t.Fatalf("expected restoreNodeKind to fail for invalid kind")
	}

	mapped := service.mapRestoreTreeError("page", "slug", resolved, tree.ErrParentNotFound)
	localized, ok := sharederrors.AsLocalizedError(mapped)
	if !ok || localized.Code != "revision_restore_parent_not_found" {
		t.Fatalf("mapped parent error = %#v", mapped)
	}
	mapped = service.mapRestoreTreeError("page", "slug", resolved, tree.ErrPageAlreadyExists)
	localized, ok = sharederrors.AsLocalizedError(mapped)
	if !ok || localized.Code != "revision_restore_slug_conflict" {
		t.Fatalf("mapped slug error = %#v", mapped)
	}
	mapped = service.mapRestoreTreeError("page", "slug", resolved, errors.New("boom"))
	localized, ok = sharederrors.AsLocalizedError(mapped)
	if !ok || localized.Code != "revision_restore_failed" {
		t.Fatalf("mapped generic error = %#v", mapped)
	}

	if err := service.restoreAssets(pageID, []AssetRef{{Name: "../bad", SHA256: "x", SizeBytes: 1}}); err == nil {
		t.Fatalf("expected restoreAssets to reject invalid names")
	}

	if err := os.MkdirAll(filepath.Join(storageDir, "assets", pageID), 0o755); err != nil {
		t.Fatalf("MkdirAll assets failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(storageDir, "assets", pageID, "temp.txt"), []byte("temp"), 0o644); err != nil {
		t.Fatalf("WriteFile temp asset failed: %v", err)
	}
	service.rollbackRestoredNode("tester", pageID)
	if _, err := treeService.GetPage(pageID); err == nil {
		t.Fatalf("expected rollbackRestoredNode to delete page")
	}
	if _, err := os.Stat(filepath.Join(storageDir, "assets", pageID)); !os.IsNotExist(err) {
		t.Fatalf("expected rollbackRestoredNode to remove asset dir, got %v", err)
	}
}

func TestRecordAssetAndStructureBranches(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	rev1, created1, err := service.RecordAssetChange(pageID, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}
	if !created1 {
		t.Fatalf("expected first asset change to create revision")
	}
	rev2, created2, err := service.RecordAssetChange(pageID, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange second call failed: %v", err)
	}
	if created2 || rev2.ID != rev1.ID {
		t.Fatalf("expected second asset change to be noop: created=%v rev=%#v", created2, rev2)
	}

	parentKind := tree.NodeKindSection
	parentID, err := treeService.CreateNode("tester", nil, "Docs", "docs", &parentKind)
	if err != nil {
		t.Fatalf("CreateNode(parent) failed: %v", err)
	}
	if err := treeService.MoveNode("tester", pageID, *parentID); err != nil {
		t.Fatalf("MoveNode failed: %v", err)
	}
	structureRev, created3, err := service.RecordStructureChange(pageID, "tester", "structure")
	if err != nil {
		t.Fatalf("RecordStructureChange failed: %v", err)
	}
	if !created3 || structureRev.Type != RevisionTypeStructureUpdate || structureRev.ParentID != *parentID {
		t.Fatalf("unexpected structure revision: %#v created=%v", structureRev, created3)
	}
}

func TestRestorePageMissingSnapshotData(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")
	_, trash, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}

	rev, err := service.store.GetRevision(pageID, trash.LastRevisionID)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if err := os.Remove(service.store.contentBlobPath(rev.ContentHash)); err != nil {
		t.Fatalf("Remove content blob failed: %v", err)
	}
	err = service.RestorePage(pageID, "tester", nil)
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_content_missing" {
		t.Fatalf("expected content missing error, got %#v (%v)", localized, err)
	}

	service2, treeService2, storageDir2 := newRevisionTestService(t)
	pageID2 := createRevisionTestPage(t, treeService2, "Page2", "page2", "hello")
	writeLiveAsset(t, storageDir2, pageID2, "a.txt", "asset")
	_, trash2, err := service2.RecordDelete(pageID2, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete second failed: %v", err)
	}
	if err := treeService2.DeleteNode("tester", pageID2, false); err != nil {
		t.Fatalf("DeleteNode second failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir2, "assets", pageID2)); err != nil {
		t.Fatalf("RemoveAll second assets failed: %v", err)
	}
	deleteRev2, err := service2.store.GetRevision(pageID2, trash2.LastRevisionID)
	if err != nil {
		t.Fatalf("GetRevision second failed: %v", err)
	}
	if err := os.Remove(service2.store.assetManifestPath(deleteRev2.AssetManifestHash)); err != nil {
		t.Fatalf("Remove asset manifest failed: %v", err)
	}
	err = service2.RestorePage(pageID2, "tester", nil)
	localized, ok = sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_assets_missing" {
		t.Fatalf("expected assets missing error, got %#v (%v)", localized, err)
	}
}

func TestRestorePageAdditionalFailuresAndHelpers(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	if err := service.RestorePage("   ", "tester", nil); err == nil {
		t.Fatalf("expected empty page id to fail")
	} else if localized, ok := sharederrors.AsLocalizedError(err); !ok || localized.Code != "revision_restore_invalid_page_id" {
		t.Fatalf("expected invalid page id error, got %#v (%v)", localized, err)
	}

	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")
	deleteRev, trash, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}
	if err := service.store.DeleteTrashEntry(pageID); err != nil {
		t.Fatalf("DeleteTrashEntry failed: %v", err)
	}
	trash.LastRevisionID = "missing-revision"
	if err := service.store.SaveTrashEntry(trash); err != nil {
		t.Fatalf("SaveTrashEntry failed: %v", err)
	}

	err = service.RestorePage(pageID, "tester", nil)
	if localized, ok := sharederrors.AsLocalizedError(err); !ok || localized.Code != "revision_restore_revision_not_found" {
		t.Fatalf("expected revision not found error, got %#v (%v)", localized, err)
	}

	trash.LastRevisionID = deleteRev.ID
	if err := service.store.SaveTrashEntry(trash); err != nil {
		t.Fatalf("SaveTrashEntry restore failed: %v", err)
	}

	badParent := "missing-parent"
	err = service.RestorePage(pageID, "tester", &badParent)
	if localized, ok := sharederrors.AsLocalizedError(err); !ok || localized.Code != "revision_restore_parent_not_found" {
		t.Fatalf("expected parent not found error, got %#v (%v)", localized, err)
	}

	if err := service.restoreAssets(pageID, []AssetRef{{Name: "dup.txt", SHA256: "abc", SizeBytes: 1}, {Name: "dup.txt", SHA256: "def", SizeBytes: 1}}); err == nil {
		t.Fatalf("expected duplicate asset names to fail")
	}
	if err := service.restoreAssets(pageID, []AssetRef{{Name: "missing.txt", SHA256: "abc", SizeBytes: 3}}); err == nil {
		t.Fatalf("expected missing asset blob to fail")
	}

	assetPath := filepath.Join(storageDir, "standalone.txt")
	if err := os.WriteFile(assetPath, []byte("css"), 0o644); err != nil {
		t.Fatalf("WriteFile standalone asset failed: %v", err)
	}
	ref, err := buildAssetRef(assetPath, "style.css")
	if err != nil {
		t.Fatalf("buildAssetRef failed: %v", err)
	}
	if ref.MIMEType == "application/octet-stream" {
		t.Fatalf("expected extension-based mime type, got %#v", ref)
	}
	if _, err := buildAssetRef(filepath.Join(storageDir, "missing.txt"), "missing.txt"); err == nil {
		t.Fatalf("expected buildAssetRef on missing file to fail")
	}

	if got := restoreParentRoutePath(" /docs/page/child/ "); got != "docs/page" {
		t.Fatalf("restoreParentRoutePath nested = %q", got)
	}
	if _, err := restoreNodeKind(" section "); err != nil {
		t.Fatalf("restoreNodeKind(section) failed: %v", err)
	}

	service.rollbackRestoredNode("tester", "missing-page")
}

func TestRecordDeleteAndRestoreRevisionHelpers(t *testing.T) {
	loggerService := NewService(t.TempDir(), nil, nil)
	if loggerService == nil || loggerService.log == nil {
		t.Fatalf("expected NewService to initialize default logger")
	}

	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	deleteRev, trash, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if deleteRev.Type != RevisionTypeDelete || trash.LastRevisionID != deleteRev.ID {
		t.Fatalf("unexpected delete revision/trash: %#v %#v", deleteRev, trash)
	}

	if err := service.recordRestoreRevision(pageID, "tester"); err != nil {
		t.Fatalf("recordRestoreRevision failed: %v", err)
	}
	latest, err := service.GetLatestRevision(pageID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != RevisionTypeRestore {
		t.Fatalf("latest restore revision = %#v", latest)
	}

	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}
	if err := service.recordRestoreRevision(pageID, "tester"); err != nil {
		t.Fatalf("recordRestoreRevision without live assets failed: %v", err)
	}
}

func TestPersistAndScanAssetHelperBranches(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")
	if err := os.MkdirAll(filepath.Join(storageDir, "assets", pageID, "subdir"), 0o755); err != nil {
		t.Fatalf("MkdirAll subdir failed: %v", err)
	}

	refs, err := service.scanLiveAssets(pageID)
	if err != nil {
		t.Fatalf("scanLiveAssets failed: %v", err)
	}
	if len(refs) != 1 || refs[0].Name != "a.txt" {
		t.Fatalf("unexpected refs: %#v", refs)
	}

	if err := service.persistLiveAssets(pageID, []AssetRef{{Name: "a.txt", SHA256: "wrong", SizeBytes: int64(len("asset"))}}); err == nil {
		t.Fatalf("expected hash mismatch")
	}
	goodRef, err := buildAssetRef(filepath.Join(storageDir, "assets", pageID, "a.txt"), "a.txt")
	if err != nil {
		t.Fatalf("buildAssetRef failed: %v", err)
	}
	goodRef.SizeBytes++
	if err := service.persistLiveAssets(pageID, []AssetRef{goodRef}); err == nil {
		t.Fatalf("expected size mismatch")
	}

	badPageID := "bad-assets"
	badDir := filepath.Join(storageDir, "assets", badPageID)
	if err := os.MkdirAll(filepath.Dir(badDir), 0o755); err != nil {
		t.Fatalf("MkdirAll bad parent failed: %v", err)
	}
	if err := os.WriteFile(badDir, []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("WriteFile bad asset dir failed: %v", err)
	}
	if _, err := service.scanLiveAssets(badPageID); err == nil {
		t.Fatalf("expected scanLiveAssets to fail when path is a file")
	}
}

func TestRestoreAssetsHashAndSizeMismatch(t *testing.T) {
	service, _, _ := newRevisionTestService(t)

	hash := sha256HexBytes([]byte("asset"))
	assetBlob := service.store.assetBlobPath(hash)
	if err := os.MkdirAll(filepath.Dir(assetBlob), 0o755); err != nil {
		t.Fatalf("MkdirAll asset blob dir failed: %v", err)
	}
	if err := os.WriteFile(assetBlob, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("WriteFile tampered blob failed: %v", err)
	}
	if err := service.restoreAssets("page-1", []AssetRef{{Name: "a.txt", SHA256: hash, SizeBytes: int64(len("asset"))}}); err == nil {
		t.Fatalf("expected restored asset hash mismatch")
	}

	hash2, err := service.store.SaveContentBlob([]byte("size-ok"))
	if err != nil {
		t.Fatalf("SaveContentBlob second failed: %v", err)
	}
	assetBlob2 := service.store.assetBlobPath(hash2)
	if err := os.MkdirAll(filepath.Dir(assetBlob2), 0o755); err != nil {
		t.Fatalf("MkdirAll asset blob dir failed: %v", err)
	}
	if err := os.WriteFile(assetBlob2, []byte("size-ok"), 0o644); err != nil {
		t.Fatalf("WriteFile asset blob failed: %v", err)
	}
	if err := service.restoreAssets("page-2", []AssetRef{{Name: "a.txt", SHA256: hash2, SizeBytes: 999}}); err == nil {
		t.Fatalf("expected restored asset size mismatch")
	}
}

func TestRecordOperationsWithoutAssets(t *testing.T) {
	service, treeService, _ := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")

	structureRev, created, err := service.RecordStructureChange(pageID, "tester", "structure")
	if err != nil {
		t.Fatalf("RecordStructureChange failed: %v", err)
	}
	if !created || structureRev.Type != RevisionTypeStructureUpdate || structureRev.AssetManifestHash == "" {
		t.Fatalf("unexpected structure revision: %#v created=%v", structureRev, created)
	}

	deleteRev, trash, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if deleteRev.AssetManifestHash == "" || trash.PageID != pageID {
		t.Fatalf("unexpected delete revision/trash: %#v %#v", deleteRev, trash)
	}
}

func TestCapturePageStateAndNewRevisionHelpers(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	state, err := service.capturePageState(pageID, true)
	if err != nil {
		t.Fatalf("capturePageState with assets failed: %v", err)
	}
	if state.PageID != pageID || state.ParentID != "" || state.AssetManifestHash == "" {
		t.Fatalf("unexpected state: %#v", state)
	}
	if len(state.Assets) != 1 || state.Assets[0].Name != "a.txt" {
		t.Fatalf("unexpected state assets: %#v", state.Assets)
	}

	rev, err := service.newRevision(RevisionTypeContentUpdate, state, " tester ", "summary", state.AssetManifestHash)
	if err != nil {
		t.Fatalf("newRevision failed: %v", err)
	}
	if rev.PageID != pageID || rev.AuthorID != "tester" || rev.AssetManifestHash != state.AssetManifestHash {
		t.Fatalf("unexpected revision: %#v", rev)
	}
	if rev.PageCreatedAt.IsZero() || rev.PageUpdatedAt.IsZero() {
		t.Fatalf("expected page timestamps on revision: %#v", rev)
	}
}

func TestRecordUpdatesAfterDeleteRevisionDoNotNoop(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

	deleteRev, _, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	contentRev, created, err := service.RecordContentUpdate(pageID, "tester", "content after delete")
	if err != nil {
		t.Fatalf("RecordContentUpdate failed: %v", err)
	}
	if !created || contentRev.ID == deleteRev.ID || contentRev.Type != RevisionTypeContentUpdate {
		t.Fatalf("unexpected content revision after delete: %#v created=%v", contentRev, created)
	}

	service2, treeService2, storageDir2 := newRevisionTestService(t)
	pageID2 := createRevisionTestPage(t, treeService2, "Page2", "page2", "hello")
	writeLiveAsset(t, storageDir2, pageID2, "a.txt", "asset")
	deleteRev2, _, err := service2.RecordDelete(pageID2, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete second failed: %v", err)
	}
	assetRev, created, err := service2.RecordAssetChange(pageID2, "tester", "asset after delete")
	if err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}
	if !created || assetRev.ID == deleteRev2.ID || assetRev.Type != RevisionTypeAssetUpdate {
		t.Fatalf("unexpected asset revision after delete: %#v created=%v", assetRev, created)
	}
}

func TestRecordContentAndAssetUpdatesWithoutAssets(t *testing.T) {
	service, treeService, _ := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")

	assetRev1, created, err := service.RecordAssetChange(pageID, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange first failed: %v", err)
	}
	if !created || assetRev1.AssetManifestHash == "" {
		t.Fatalf("unexpected first asset revision: %#v created=%v", assetRev1, created)
	}
	assetRev2, created, err := service.RecordAssetChange(pageID, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange second failed: %v", err)
	}
	if created || assetRev2.ID != assetRev1.ID {
		t.Fatalf("expected second asset change to noop: %#v created=%v", assetRev2, created)
	}

	content := "hello-2"
	if err := treeService.UpdateNode("tester", pageID, "Page", "page", &content); err != nil {
		t.Fatalf("UpdateNode content failed: %v", err)
	}
	assetRev3, created, err := service.RecordAssetChange(pageID, "tester", "asset after content")
	if err != nil {
		t.Fatalf("RecordAssetChange after content failed: %v", err)
	}
	if !created || assetRev3.ID == assetRev1.ID {
		t.Fatalf("expected new asset revision after content change: %#v created=%v", assetRev3, created)
	}

	content = "hello-3"
	if err := treeService.UpdateNode("tester", pageID, "Page", "page", &content); err != nil {
		t.Fatalf("UpdateNode second content failed: %v", err)
	}
	contentRev, created, err := service.RecordContentUpdate(pageID, "tester", "content")
	if err != nil {
		t.Fatalf("RecordContentUpdate failed: %v", err)
	}
	if !created || contentRev.Type != RevisionTypeContentUpdate {
		t.Fatalf("unexpected content revision: %#v created=%v", contentRev, created)
	}
}

func TestRestorePageRollbackKeepsTrashOnAssetFailure(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	_, trash, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	deleteRev, err := service.store.GetRevision(pageID, trash.LastRevisionID)
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	assetRefs, err := service.store.LoadAssetManifest(deleteRev.AssetManifestHash)
	if err != nil {
		t.Fatalf("LoadAssetManifest failed: %v", err)
	}
	if len(assetRefs) != 1 {
		t.Fatalf("expected 1 asset ref, got %#v", assetRefs)
	}
	assetBlobPath := service.store.assetBlobPath(assetRefs[0].SHA256)
	if err := os.WriteFile(assetBlobPath, []byte("tampered"), 0o644); err != nil {
		t.Fatalf("WriteFile tampered asset blob failed: %v", err)
	}

	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}

	err = service.RestorePage(pageID, "tester", nil)
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_failed" {
		t.Fatalf("expected restore failure, got %#v (%v)", localized, err)
	}
	if _, err := treeService.GetPage(pageID); err == nil {
		t.Fatalf("expected restored page to be rolled back")
	}
	if _, err := os.Stat(filepath.Join(storageDir, "assets", pageID)); !os.IsNotExist(err) {
		t.Fatalf("expected asset dir to be removed after rollback, got %v", err)
	}
	if _, err := service.GetTrashEntry(pageID); err != nil {
		t.Fatalf("expected trash entry to remain after failed restore: %v", err)
	}
}

func TestRestorePagePreservesMetadataAndIdentity(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	before, err := treeService.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage before delete failed: %v", err)
	}
	beforeCreatedAt := before.Metadata.CreatedAt
	beforeUpdatedAt := before.Metadata.UpdatedAt
	beforeCreator := before.Metadata.CreatorID
	beforeAuthor := before.Metadata.LastAuthorID

	_, _, err = service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}

	rootParent := "root"
	if err := service.RestorePage(pageID, "tester", &rootParent); err != nil {
		t.Fatalf("RestorePage failed: %v", err)
	}

	after, err := treeService.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage after restore failed: %v", err)
	}
	if after.ID != pageID {
		t.Fatalf("restored page id = %q, want %q", after.ID, pageID)
	}
	if !after.Metadata.CreatedAt.Equal(beforeCreatedAt) || !after.Metadata.UpdatedAt.Equal(beforeUpdatedAt) {
		t.Fatalf("restored timestamps changed: before=(%v,%v) after=(%v,%v)", beforeCreatedAt, beforeUpdatedAt, after.Metadata.CreatedAt, after.Metadata.UpdatedAt)
	}
	if after.Metadata.CreatorID != beforeCreator || after.Metadata.LastAuthorID != beforeAuthor {
		t.Fatalf("restored author metadata changed: before=(%q,%q) after=(%q,%q)", beforeCreator, beforeAuthor, after.Metadata.CreatorID, after.Metadata.LastAuthorID)
	}
	if _, err := service.GetTrashEntry(pageID); err == nil {
		t.Fatalf("expected trash entry to be removed after successful restore")
	}
}

func TestRestoreRevisionRehydratesLivePageState(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)

	sectionKind := tree.NodeKindSection
	docsID, err := treeService.CreateNode("tester", nil, "Docs", "docs", &sectionKind)
	if err != nil {
		t.Fatalf("CreateNode(docs) failed: %v", err)
	}
	archiveID, err := treeService.CreateNode("tester", nil, "Archive", "archive", &sectionKind)
	if err != nil {
		t.Fatalf("CreateNode(archive) failed: %v", err)
	}

	pageKind := tree.NodeKindPage
	pageIDPtr, err := treeService.CreateNode("tester", docsID, "Original", "original", &pageKind)
	if err != nil {
		t.Fatalf("CreateNode(page) failed: %v", err)
	}
	pageID := *pageIDPtr

	originalContent := "first version"
	if err := treeService.UpdateNode("tester", pageID, "Original", "original", &originalContent); err != nil {
		t.Fatalf("UpdateNode(original) failed: %v", err)
	}
	writeLiveAsset(t, storageDir, pageID, "old.txt", "old-asset")
	originalRev, created, err := service.RecordAssetChange(pageID, "tester", "original state")
	if err != nil {
		t.Fatalf("RecordAssetChange(original) failed: %v", err)
	}
	if !created || originalRev == nil {
		t.Fatalf("expected original revision to be created")
	}

	changedContent := "second version"
	if err := treeService.UpdateNode("tester", pageID, "Changed", "changed", &changedContent); err != nil {
		t.Fatalf("UpdateNode(changed) failed: %v", err)
	}
	if err := treeService.MoveNode("tester", pageID, *archiveID); err != nil {
		t.Fatalf("MoveNode failed: %v", err)
	}
	if err := os.Remove(filepath.Join(storageDir, "assets", pageID, "old.txt")); err != nil {
		t.Fatalf("Remove(old asset) failed: %v", err)
	}
	writeLiveAsset(t, storageDir, pageID, "new.txt", "new-asset")

	if err := service.RestoreRevision(pageID, originalRev.ID, "tester"); err != nil {
		t.Fatalf("RestoreRevision failed: %v", err)
	}

	page, err := treeService.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage failed: %v", err)
	}
	if page.Title != "Changed" || page.Slug != "changed" {
		t.Fatalf("restored page identity = (%q,%q)", page.Title, page.Slug)
	}
	if page.Content != originalContent {
		t.Fatalf("restored content = %q, want %q", page.Content, originalContent)
	}
	if got := page.CalculatePath(); got != "/archive/changed" {
		t.Fatalf("restored path = %q", got)
	}

	oldAsset, err := os.ReadFile(filepath.Join(storageDir, "assets", pageID, "old.txt"))
	if err != nil {
		t.Fatalf("ReadFile(old asset) failed: %v", err)
	}
	if string(oldAsset) != "old-asset" {
		t.Fatalf("old asset = %q", string(oldAsset))
	}
	if _, err := os.Stat(filepath.Join(storageDir, "assets", pageID, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected new asset to be removed, got %v", err)
	}

	latest, err := service.GetLatestRevision(pageID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != RevisionTypeRestore {
		t.Fatalf("latest revision = %#v", latest)
	}
}

type failingDeleteOnceStore struct {
	called int
	err    error
}

func TestRestorePageIsIdempotentAfterDeleteTrashFailure(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	_, _, err := service.RecordDelete(pageID, "tester", "delete")
	if err != nil {
		t.Fatalf("RecordDelete failed: %v", err)
	}
	if err := treeService.DeleteNode("tester", pageID, false); err != nil {
		t.Fatalf("DeleteNode failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(storageDir, "assets", pageID)); err != nil {
		t.Fatalf("RemoveAll assets failed: %v", err)
	}

	deleteHook := &failingDeleteOnceStore{err: errors.New("delete trash failed")}
	service.deleteTrashEntry = func(id string) error {
		deleteHook.called++
		if deleteHook.called == 1 {
			return deleteHook.err
		}
		return service.store.DeleteTrashEntry(id)
	}

	err = service.RestorePage(pageID, "tester", nil)
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok || localized.Code != "revision_restore_failed" {
		t.Fatalf("expected restore failure on first commit attempt, got %#v (%v)", localized, err)
	}
	page, err := treeService.GetPage(pageID)
	if err != nil || page == nil {
		t.Fatalf("expected page to be restored before commit failure: %#v %v", page, err)
	}
	if _, err := service.GetTrashEntry(pageID); err != nil {
		t.Fatalf("expected trash to remain after commit failure: %v", err)
	}
	latest, err := service.GetLatestRevision(pageID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != RevisionTypeRestore {
		t.Fatalf("expected restore revision to be recorded before trash delete, got %#v", latest)
	}

	if err := service.RestorePage(pageID, "tester", nil); err != nil {
		t.Fatalf("expected second restore to complete idempotently: %v", err)
	}
	if _, err := service.GetTrashEntry(pageID); err == nil {
		t.Fatalf("expected trash entry to be removed after retry")
	}
	latest, err = service.GetLatestRevision(pageID)
	if err != nil {
		t.Fatalf("GetLatestRevision second failed: %v", err)
	}
	if latest == nil || latest.Type != RevisionTypeRestore {
		t.Fatalf("expected latest revision to stay restore after retry, got %#v", latest)
	}
	if deleteHook.called != 2 {
		t.Fatalf("expected delete hook to be called twice, got %d", deleteHook.called)
	}
}

func TestRecordContentAndStructureRebuildMissingPreviousManifest(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	firstRev, created, err := service.RecordAssetChange(pageID, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}
	if !created {
		t.Fatalf("expected initial asset revision")
	}
	missingManifestPath := service.store.assetManifestPath(firstRev.AssetManifestHash)
	if err := os.Remove(missingManifestPath); err != nil {
		t.Fatalf("Remove manifest failed: %v", err)
	}

	content := "hello-updated"
	if err := treeService.UpdateNode("tester", pageID, "Page", "page", &content); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}
	contentRev, created, err := service.RecordContentUpdate(pageID, "tester", "content")
	if err != nil {
		t.Fatalf("RecordContentUpdate failed: %v", err)
	}
	if !created {
		t.Fatalf("expected content revision after manifest rebuild")
	}
	if contentRev.AssetManifestHash != firstRev.AssetManifestHash {
		t.Fatalf("expected rebuilt manifest hash to match live assets, got %q want %q", contentRev.AssetManifestHash, firstRev.AssetManifestHash)
	}
	if _, err := service.store.LoadAssetManifest(contentRev.AssetManifestHash); err != nil {
		t.Fatalf("expected rebuilt manifest to be readable: %v", err)
	}

	if err := os.Remove(service.store.assetManifestPath(contentRev.AssetManifestHash)); err != nil {
		t.Fatalf("Remove rebuilt manifest failed: %v", err)
	}
	structureRev, created, err := service.RecordStructureChange(pageID, "tester", "structure")
	if err != nil {
		t.Fatalf("RecordStructureChange failed: %v", err)
	}
	if !created {
		t.Fatalf("expected structure revision after manifest rebuild")
	}
	if structureRev.AssetManifestHash != firstRev.AssetManifestHash {
		t.Fatalf("expected structure manifest hash to match live assets, got %q want %q", structureRev.AssetManifestHash, firstRev.AssetManifestHash)
	}
}

func TestCheckRevisionIntegrityReportsBrokenArtifacts(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)

	pageID1 := createRevisionTestPage(t, treeService, "Page1", "page1", "hello")
	_, _, err := service.RecordContentUpdate(pageID1, "tester", "content")
	if err != nil {
		t.Fatalf("RecordContentUpdate page1 failed: %v", err)
	}
	revs1, err := service.ListRevisions(pageID1)
	if err != nil || len(revs1) == 0 {
		t.Fatalf("ListRevisions page1 failed: %#v %v", revs1, err)
	}
	if err := os.Remove(service.store.contentBlobPath(revs1[0].ContentHash)); err != nil {
		t.Fatalf("Remove content blob failed: %v", err)
	}
	issues1, err := service.CheckRevisionIntegrity(pageID1)
	if err != nil {
		t.Fatalf("CheckRevisionIntegrity page1 failed: %v", err)
	}
	if len(issues1) != 1 || issues1[0].Code != "missing_content_blob" {
		t.Fatalf("unexpected page1 issues: %#v", issues1)
	}

	pageID2 := createRevisionTestPage(t, treeService, "Page2", "page2", "hello")
	writeLiveAsset(t, storageDir, pageID2, "a.txt", "asset-a")
	assetRev, _, err := service.RecordAssetChange(pageID2, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange page2 failed: %v", err)
	}
	if err := os.Remove(service.store.assetManifestPath(assetRev.AssetManifestHash)); err != nil {
		t.Fatalf("Remove asset manifest failed: %v", err)
	}
	issues2, err := service.CheckRevisionIntegrity(pageID2)
	if err != nil {
		t.Fatalf("CheckRevisionIntegrity page2 failed: %v", err)
	}
	if len(issues2) != 1 || issues2[0].Code != "missing_asset_manifest" {
		t.Fatalf("unexpected page2 issues: %#v", issues2)
	}

	pageID3 := createRevisionTestPage(t, treeService, "Page3", "page3", "hello")
	writeLiveAsset(t, storageDir, pageID3, "a.txt", "asset-a")
	assetRev3, _, err := service.RecordAssetChange(pageID3, "tester", "asset")
	if err != nil {
		t.Fatalf("RecordAssetChange page3 failed: %v", err)
	}
	refs, err := service.store.LoadAssetManifest(assetRev3.AssetManifestHash)
	if err != nil || len(refs) != 1 {
		t.Fatalf("LoadAssetManifest page3 failed: %#v %v", refs, err)
	}
	if err := os.WriteFile(service.store.assetBlobPath(refs[0].SHA256), []byte("tampered"), 0o644); err != nil {
		t.Fatalf("WriteFile tampered asset blob failed: %v", err)
	}
	issues3, err := service.CheckRevisionIntegrity(pageID3)
	if err != nil {
		t.Fatalf("CheckRevisionIntegrity page3 failed: %v", err)
	}
	if len(issues3) != 1 || issues3[0].Code != "asset_blob_hash_mismatch" {
		t.Fatalf("unexpected page3 issues: %#v", issues3)
	}
}

func TestCompareRevisionSnapshots(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "one")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	baseRev, _, err := service.RecordAssetChange(pageID, "tester", "base")
	if err != nil {
		t.Fatalf("RecordAssetChange base failed: %v", err)
	}

	content := "two"
	if err := treeService.UpdateNode("tester", pageID, "Page", "page", &content); err != nil {
		t.Fatalf("UpdateNode failed: %v", err)
	}
	writeLiveAsset(t, storageDir, pageID, "b.txt", "asset-b")
	targetRev, _, err := service.RecordAssetChange(pageID, "tester", "target")
	if err != nil {
		t.Fatalf("RecordAssetChange target failed: %v", err)
	}

	comparison, err := service.CompareRevisionSnapshots(pageID, baseRev.ID, targetRev.ID)
	if err != nil {
		t.Fatalf("CompareRevisionSnapshots failed: %v", err)
	}
	if comparison == nil || comparison.Base == nil || comparison.Target == nil {
		t.Fatalf("comparison = %#v", comparison)
	}
	if !comparison.ContentChanged {
		t.Fatalf("expected content to be marked as changed")
	}
	if len(comparison.AssetChanges) != 1 || comparison.AssetChanges[0].Name != "b.txt" || comparison.AssetChanges[0].Status != "added" {
		t.Fatalf("asset changes = %#v", comparison.AssetChanges)
	}
}

func TestGetRevisionAssetReturnsBlobForDeletedLiveAsset(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "one")
	writeLiveAsset(t, storageDir, pageID, "image.png", "asset-image")

	rev, _, err := service.RecordAssetChange(pageID, "tester", "with asset")
	if err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}

	if err := os.Remove(filepath.Join(storageDir, "assets", pageID, "image.png")); err != nil {
		t.Fatalf("Remove live asset failed: %v", err)
	}

	asset, err := service.GetRevisionAsset(pageID, rev.ID, "image.png")
	if err != nil {
		t.Fatalf("GetRevisionAsset failed: %v", err)
	}
	if asset == nil {
		t.Fatal("expected revision asset content")
	}
	if asset.Asset.Name != "image.png" {
		t.Fatalf("asset name = %q", asset.Asset.Name)
	}
	if string(asset.Content) != "asset-image" {
		t.Fatalf("asset content = %q", string(asset.Content))
	}
}

func TestGetRevisionAssetReturnsNotFoundForMissingManifestEntry(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "one")
	writeLiveAsset(t, storageDir, pageID, "image.png", "asset-image")

	rev, _, err := service.RecordAssetChange(pageID, "tester", "with asset")
	if err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}

	_, err = service.GetRevisionAsset(pageID, rev.ID, "missing.png")
	localized, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized error, got %T", err)
	}
	if localized.Code != "revision_preview_asset_not_found" {
		t.Fatalf("localized.Code = %q", localized.Code)
	}
}
