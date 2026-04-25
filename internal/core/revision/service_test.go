package revision

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

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

func TestRecordContentUpdatesHappyPathAndNoop(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID1 := createRevisionTestPage(t, treeService, "Page 1", "page-1", "hello")
	pageID2 := createRevisionTestPage(t, treeService, "Page 2", "page-2", "world")
	writeLiveAsset(t, storageDir, pageID1, "a.txt", "asset-a")
	writeLiveAsset(t, storageDir, pageID2, "b.txt", "asset-b")

	page1, err := treeService.GetPage(pageID1)
	if err != nil {
		t.Fatalf("GetPage(page1) failed: %v", err)
	}
	page2, err := treeService.GetPage(pageID2)
	if err != nil {
		t.Fatalf("GetPage(page2) failed: %v", err)
	}

	errs := service.RecordContentUpdates([]*tree.Page{page1, page2}, "tester", "batch")
	if len(errs) != 2 {
		t.Fatalf("expected 2 result errors, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("RecordContentUpdates error[%d] = %v", i, err)
		}
	}

	revisions1, err := service.ListRevisions(pageID1)
	if err != nil {
		t.Fatalf("ListRevisions(page1) failed: %v", err)
	}
	if len(revisions1) != 1 || revisions1[0].Type != RevisionTypeContentUpdate {
		t.Fatalf("unexpected revisions for page1: %#v", revisions1)
	}

	revisions2, err := service.ListRevisions(pageID2)
	if err != nil {
		t.Fatalf("ListRevisions(page2) failed: %v", err)
	}
	if len(revisions2) != 1 || revisions2[0].Type != RevisionTypeContentUpdate {
		t.Fatalf("unexpected revisions for page2: %#v", revisions2)
	}

	errs = service.RecordContentUpdates([]*tree.Page{page1, page2}, "tester", "batch")
	if len(errs) != 2 {
		t.Fatalf("expected 2 noop result errors, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("RecordContentUpdates noop error[%d] = %v", i, err)
		}
	}

	revisions1After, err := service.ListRevisions(pageID1)
	if err != nil {
		t.Fatalf("ListRevisions(page1 after noop) failed: %v", err)
	}
	if len(revisions1After) != 1 {
		t.Fatalf("expected page1 noop to keep 1 revision, got %d", len(revisions1After))
	}

	revisions2After, err := service.ListRevisions(pageID2)
	if err != nil {
		t.Fatalf("ListRevisions(page2 after noop) failed: %v", err)
	}
	if len(revisions2After) != 1 {
		t.Fatalf("expected page2 noop to keep 1 revision, got %d", len(revisions2After))
	}
}

func TestRecordContentUpdates_PreservesPerInputErrors(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID1 := createRevisionTestPage(t, treeService, "Page 1", "page-1", "hello")
	pageID2 := createRevisionTestPage(t, treeService, "Page 2", "page-2", "world")
	writeLiveAsset(t, storageDir, pageID1, "a.txt", "asset-a")
	writeLiveAsset(t, storageDir, pageID2, "b.txt", "asset-b")

	page1, err := treeService.GetPage(pageID1)
	if err != nil {
		t.Fatalf("GetPage(page1) failed: %v", err)
	}
	page2, err := treeService.GetPage(pageID2)
	if err != nil {
		t.Fatalf("GetPage(page2) failed: %v", err)
	}

	errs := service.RecordContentUpdates([]*tree.Page{page1, nil, page2}, "tester", "batch")
	if len(errs) != 3 {
		t.Fatalf("expected 3 result errors, got %d", len(errs))
	}
	if errs[0] != nil {
		t.Fatalf("unexpected error for page1: %v", errs[0])
	}
	if errs[1] == nil || errs[1].Error() != "page is required" {
		t.Fatalf("expected nil-page error in slot 1, got %v", errs[1])
	}
	if errs[2] != nil {
		t.Fatalf("unexpected error for page2: %v", errs[2])
	}

	revisions1, err := service.ListRevisions(pageID1)
	if err != nil {
		t.Fatalf("ListRevisions(page1) failed: %v", err)
	}
	if len(revisions1) != 1 {
		t.Fatalf("expected 1 revision for page1, got %d", len(revisions1))
	}

	revisions2, err := service.ListRevisions(pageID2)
	if err != nil {
		t.Fatalf("ListRevisions(page2) failed: %v", err)
	}
	if len(revisions2) != 1 {
		t.Fatalf("expected 1 revision for page2, got %d", len(revisions2))
	}
}

func TestRecordContentUpdates_DuplicatePageIDsStayDeterministic(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset-a")

	page, err := treeService.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage(page) failed: %v", err)
	}

	errs := service.RecordContentUpdates([]*tree.Page{page, page}, "tester", "batch")
	if len(errs) != 2 {
		t.Fatalf("expected 2 result errors, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("unexpected error in slot %d: %v", i, err)
		}
	}

	revisions, err := service.ListRevisions(pageID)
	if err != nil {
		t.Fatalf("ListRevisions(page) failed: %v", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("expected duplicate batch entry to yield 1 revision, got %d", len(revisions))
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

	revisions, err := service.ListRevisions(pageID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revisions) < 1 {
		t.Fatalf("expected revisions, got %#v", revisions)
	}
	paged, _, err := service.ListRevisionsPage(pageID, "", 1)
	if err != nil {
		t.Fatalf("ListRevisionsPage failed: %v", err)
	}
	if len(paged) != 1 {
		t.Fatalf("expected one paged revision, got %d", len(paged))
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

	if err := service.persistLiveAssets(pageID, nil); err != nil {
		t.Fatalf("persistLiveAssets(nil) failed: %v", err)
	}
	if _, err := service.scanLiveAssets("missing"); err != nil {
		t.Fatalf("scanLiveAssets(missing) failed: %v", err)
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

func TestRestoreAssetsHelpers(t *testing.T) {
	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")

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
}

func TestRecordRestoreRevisionHelper(t *testing.T) {
	loggerService := NewService(t.TempDir(), nil, nil)
	if loggerService == nil || loggerService.log == nil {
		t.Fatalf("expected NewService to initialize default logger")
	}

	service, treeService, storageDir := newRevisionTestService(t)
	pageID := createRevisionTestPage(t, treeService, "Page", "page", "hello")
	writeLiveAsset(t, storageDir, pageID, "a.txt", "asset")

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
	assetBlob := service.store.AssetBlobPath(hash)
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
	assetBlob2 := service.store.AssetBlobPath(hash2)
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
	// Restore rehydrates revision content and title while preserving the current slug/path.
	if page.Title != "Original" || page.Slug != "changed" {
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
	if err := os.WriteFile(service.store.AssetBlobPath(refs[0].SHA256), []byte("tampered"), 0o644); err != nil {
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
	content, err := os.ReadFile(asset.Path)
	if err != nil {
		t.Fatalf("read asset from path: %v", err)
	}
	if string(content) != "asset-image" {
		t.Fatalf("asset content = %q", string(content))
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
