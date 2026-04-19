package revision

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
)

func TestFSStoreRevisionReadPaths(t *testing.T) {
	store := NewFSStore(t.TempDir())
	created1 := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	created2 := created1.Add(time.Minute)
	created3 := created2.Add(time.Minute)

	rev1 := &Revision{ID: "rev1", PageID: "page-1", CreatedAt: created1, Type: RevisionTypeContentUpdate, Title: "A", Slug: "a"}
	rev2 := &Revision{ID: "rev2", PageID: "page-1", CreatedAt: created2, Type: RevisionTypeAssetUpdate, Title: "A", Slug: "a"}
	rev3 := &Revision{ID: "rev3", PageID: "page-1", CreatedAt: created3, Type: RevisionTypeStructureUpdate, Title: "A", Slug: "a"}
	for _, rev := range []*Revision{rev1, rev2, rev3} {
		if err := store.SaveRevision(rev); err != nil {
			t.Fatalf("SaveRevision(%s) failed: %v", rev.ID, err)
		}
	}

	latest, err := store.GetLatestRevision("page-1")
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.ID != "rev3" {
		t.Fatalf("latest = %#v", latest)
	}

	got, err := store.GetRevision("page-1", "rev2")
	if err != nil {
		t.Fatalf("GetRevision failed: %v", err)
	}
	if got.ID != "rev2" {
		t.Fatalf("GetRevision returned %q", got.ID)
	}

	firstPage, nextCursor, err := store.ListRevisionsPage("page-1", "", 2)
	if err != nil {
		t.Fatalf("ListRevisionsPage first page failed: %v", err)
	}
	if len(firstPage) != 2 || firstPage[0].ID != "rev3" || firstPage[1].ID != "rev2" {
		t.Fatalf("first page = %#v", firstPage)
	}
	if nextCursor == "" {
		t.Fatalf("expected next cursor")
	}

	secondPage, nextCursor2, err := store.ListRevisionsPage("page-1", nextCursor, 2)
	if err != nil {
		t.Fatalf("ListRevisionsPage second page failed: %v", err)
	}
	if len(secondPage) != 1 || secondPage[0].ID != "rev1" {
		t.Fatalf("second page = %#v", secondPage)
	}
	if nextCursor2 != "" {
		t.Fatalf("expected empty next cursor, got %q", nextCursor2)
	}
}

func TestFSStoreBlobPaths(t *testing.T) {
	store := NewFSStore(t.TempDir())

	contentHash, err := store.SaveContentBlob([]byte("hello"))
	if err != nil {
		t.Fatalf("SaveContentBlob failed: %v", err)
	}
	raw, err := store.ReadContentBlob(contentHash)
	if err != nil {
		t.Fatalf("ReadContentBlob failed: %v", err)
	}
	if string(raw) != "hello" {
		t.Fatalf("content blob = %q", string(raw))
	}

	assetSrcDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(assetSrcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	assetSrc := filepath.Join(assetSrcDir, "asset.txt")
	if err := os.WriteFile(assetSrc, []byte("asset-data"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	hash, size, err := store.SaveAssetBlobFromPath(assetSrc)
	if err != nil {
		t.Fatalf("SaveAssetBlobFromPath failed: %v", err)
	}
	if size != int64(len("asset-data")) {
		t.Fatalf("asset size = %d", size)
	}
	assetRaw, err := store.ReadAssetBlob(hash)
	if err != nil {
		t.Fatalf("ReadAssetBlob failed: %v", err)
	}
	if string(assetRaw) != "asset-data" {
		t.Fatalf("asset blob = %q", string(assetRaw))
	}

	manifestHash, err := store.SaveAssetManifest([]AssetRef{{Name: "asset.txt", SHA256: hash, SizeBytes: size}})
	if err != nil {
		t.Fatalf("SaveAssetManifest failed: %v", err)
	}
	manifest, err := store.LoadAssetManifest(manifestHash)
	if err != nil {
		t.Fatalf("LoadAssetManifest failed: %v", err)
	}
	if len(manifest) != 1 || manifest[0].Name != "asset.txt" {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestFSStoreDeletePageRevisions(t *testing.T) {
	store := NewFSStore(t.TempDir())
	createdAt := time.Date(2026, 4, 12, 18, 0, 0, 0, time.UTC)

	revision := &Revision{
		ID:        "rev1",
		PageID:    "page-1",
		CreatedAt: createdAt,
		Type:      RevisionTypeContentUpdate,
		Title:     "Page",
		Slug:      "page",
	}
	if err := store.SaveRevision(revision); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}

	if _, err := os.Stat(store.revisionsPageDir("page-1")); err != nil {
		t.Fatalf("expected revisions dir to exist, got %v", err)
	}

	if err := store.DeletePageRevisions("page-1"); err != nil {
		t.Fatalf("DeletePageRevisions failed: %v", err)
	}

	if _, err := os.Stat(store.revisionsPageDir("page-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected revisions dir to be removed, got %v", err)
	}

	revisions, err := store.ListRevisions("page-1")
	if err != nil {
		t.Fatalf("ListRevisions after delete failed: %v", err)
	}
	if len(revisions) != 0 {
		t.Fatalf("expected no revisions after delete, got %#v", revisions)
	}

	if err := store.DeletePageRevisions("page-1"); err != nil {
		t.Fatalf("DeletePageRevisions missing should be ignored: %v", err)
	}
}

func TestFSStoreValidationAndEmptyPaths(t *testing.T) {
	store := NewFSStore(t.TempDir())

	if _, err := store.ReadContentBlob(""); err != nil {
		t.Fatalf("ReadContentBlob empty hash failed: %v", err)
	}
	if _, err := store.LoadAssetManifest(""); err != nil {
		t.Fatalf("LoadAssetManifest empty hash failed: %v", err)
	}
	if _, err := store.ReadAssetBlob(""); err == nil {
		t.Fatalf("expected ReadAssetBlob empty hash to fail")
	}
	if err := store.SaveRevision(nil); err == nil {
		t.Fatalf("expected SaveRevision(nil) to fail")
	}
}

func TestFSStoreListRevisionWrappersAndValidation(t *testing.T) {
	store := NewFSStore(t.TempDir())
	if got, err := store.ListRevisions("missing"); err != nil || len(got) != 0 {
		t.Fatalf("ListRevisions(missing) = %#v, %v", got, err)
	}
	if got, err := store.GetLatestRevision("missing"); err != nil || got != nil {
		t.Fatalf("GetLatestRevision(missing) = %#v, %v", got, err)
	}
	if _, err := store.GetRevision("missing", ""); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist for empty revision id, got %v", err)
	}
	if _, err := store.GetRevision("missing", "rev1"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist for missing revision, got %v", err)
	}

	if got := shardHash("a"); got != "00" {
		t.Fatalf("shardHash short = %q", got)
	}
	if got := shardHash("abcd"); got != "ab" {
		t.Fatalf("shardHash normal = %q", got)
	}

	items := cloneAndSortAssetRefs([]AssetRef{{Name: "b.txt", SHA256: "2"}, {Name: "a.txt", SHA256: "1"}})
	if items[0].Name != "a.txt" {
		t.Fatalf("sorted items = %#v", items)
	}
}

func TestFSStoreIdempotentSaves(t *testing.T) {
	store := NewFSStore(t.TempDir())

	h1, err := store.SaveContentBlob([]byte("same"))
	if err != nil {
		t.Fatalf("first SaveContentBlob failed: %v", err)
	}
	h2, err := store.SaveContentBlob([]byte("same"))
	if err != nil {
		t.Fatalf("second SaveContentBlob failed: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("content hash mismatch: %q vs %q", h1, h2)
	}

	srcDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	src := filepath.Join(srcDir, "asset.txt")
	if err := os.WriteFile(src, []byte("asset"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	hash1, size1, err := store.SaveAssetBlobFromPath(src)
	if err != nil {
		t.Fatalf("first SaveAssetBlobFromPath failed: %v", err)
	}
	hash2, size2, err := store.SaveAssetBlobFromPath(src)
	if err != nil {
		t.Fatalf("second SaveAssetBlobFromPath failed: %v", err)
	}
	if hash1 != hash2 || size1 != size2 {
		t.Fatalf("asset save mismatch: %q/%d vs %q/%d", hash1, size1, hash2, size2)
	}

	manifest := []AssetRef{{Name: "asset.txt", SHA256: hash1, SizeBytes: size1}}
	m1, err := store.SaveAssetManifest(manifest)
	if err != nil {
		t.Fatalf("first SaveAssetManifest failed: %v", err)
	}
	m2, err := store.SaveAssetManifest(manifest)
	if err != nil {
		t.Fatalf("second SaveAssetManifest failed: %v", err)
	}
	if m1 != m2 {
		t.Fatalf("manifest hash mismatch: %q vs %q", m1, m2)
	}

	if err := store.SaveRevision(&Revision{}); err == nil {
		t.Fatalf("expected zero-value revision to fail")
	}
}

func TestFSStoreCursorAndFileFilteringHelpers(t *testing.T) {
	store := NewFSStore(t.TempDir())
	pageID := "page-1"
	created := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		rev := &Revision{ID: string(rune('a' + i)), PageID: pageID, CreatedAt: created.Add(time.Duration(i) * time.Minute), Type: RevisionTypeContentUpdate, Title: "Page", Slug: "page"}
		if err := store.SaveRevision(rev); err != nil {
			t.Fatalf("SaveRevision(%d) failed: %v", i, err)
		}
	}

	dir := store.revisionsPageDir(pageID)
	if err := os.MkdirAll(filepath.Join(dir, "ignored-dir"), 0o755); err != nil {
		t.Fatalf("MkdirAll ignored dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("WriteFile ignored file failed: %v", err)
	}

	names, err := store.revisionFileNames(pageID)
	if err != nil {
		t.Fatalf("revisionFileNames failed: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 revision files, got %#v", names)
	}

	if got, next, err := store.ListRevisionsPage(pageID, "missing-cursor", 1); err != nil || len(got) != 0 || next != "" {
		t.Fatalf("ListRevisionsPage missing cursor = %#v, %q, %v", got, next, err)
	}

	brokenDir := store.revisionsPageDir("broken-page")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("MkdirAll broken dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "20260326T120000.000000000Z_a.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile broken revision failed: %v", err)
	}
	if _, err := store.GetLatestRevision("broken-page"); err == nil {
		t.Fatalf("expected invalid latest revision json to fail")
	}
}

func TestFSStoreJSONHelpersAndLocalizedNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "value.json")
	payload := map[string]string{"a": "b"}
	if err := writeJSONAtomic(path, payload); err != nil {
		t.Fatalf("writeJSONAtomic failed: %v", err)
	}
	var got map[string]string
	if err := readJSON(path, &got); err != nil {
		t.Fatalf("readJSON failed: %v", err)
	}
	if got["a"] != "b" {
		t.Fatalf("unexpected json payload: %#v", got)
	}

	badPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(badPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile bad json failed: %v", err)
	}
	if err := readJSON(badPath, &got); err == nil {
		t.Fatalf("expected invalid json to fail")
	}

	var localized *sharederrors.LocalizedError
	if localized.Error() != "" {
		t.Fatalf("nil localized error string should be empty")
	}
	if localized.Unwrap() != nil {
		t.Fatalf("nil localized error unwrap should be nil")
	}
}

func TestFSStoreAssetBlobErrors(t *testing.T) {
	store := NewFSStore(t.TempDir())
	if _, _, err := store.SaveAssetBlobFromPath(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatalf("expected SaveAssetBlobFromPath on missing file to fail")
	}
}

func TestFSStoreFailuresOnInvalidBasePath(t *testing.T) {
	root := t.TempDir()
	invalidBase := filepath.Join(root, "not-a-dir")
	if err := os.WriteFile(invalidBase, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile invalid base failed: %v", err)
	}
	store := NewFSStore(invalidBase)

	if _, err := store.SaveContentBlob([]byte("hello")); err == nil {
		t.Fatalf("expected SaveContentBlob to fail")
	}

	src := filepath.Join(root, "asset.txt")
	if err := os.WriteFile(src, []byte("asset"), 0o644); err != nil {
		t.Fatalf("WriteFile src asset failed: %v", err)
	}
	if _, _, err := store.SaveAssetBlobFromPath(src); err == nil {
		t.Fatalf("expected SaveAssetBlobFromPath to fail")
	}
	if _, err := store.SaveAssetManifest([]AssetRef{{Name: "asset.txt", SHA256: "abc", SizeBytes: 5}}); err == nil {
		t.Fatalf("expected SaveAssetManifest to fail")
	}

	rev := &Revision{ID: "rev1", PageID: "page-1", CreatedAt: time.Now().UTC(), Type: RevisionTypeContentUpdate, Title: "Page", Slug: "page"}
	if err := store.SaveRevision(rev); err == nil {
		t.Fatalf("expected SaveRevision to fail")
	}
	if _, err := store.ListRevisions("page-1"); err == nil {
		t.Fatalf("expected ListRevisions to fail")
	}
}

func TestFSStoreGetRevisionUsesAndBackfillsIndex(t *testing.T) {
	store := NewFSStore(t.TempDir())
	created := time.Date(2026, 3, 26, 12, 30, 0, 0, time.UTC)
	rev := &Revision{ID: "rev-index", PageID: "page-1", CreatedAt: created, Type: RevisionTypeContentUpdate, Title: "Page", Slug: "page"}
	if err := store.SaveRevision(rev); err != nil {
		t.Fatalf("SaveRevision failed: %v", err)
	}

	index, err := store.loadRevisionIndex("page-1")
	if err != nil {
		t.Fatalf("loadRevisionIndex failed: %v", err)
	}
	if got := index[rev.ID]; got == "" {
		t.Fatalf("expected revision index entry for %q, got %#v", rev.ID, index)
	}

	if err := os.Remove(store.revisionIndexPath("page-1")); err != nil {
		t.Fatalf("Remove revision index failed: %v", err)
	}
	got, err := store.GetRevision("page-1", rev.ID)
	if err != nil {
		t.Fatalf("GetRevision fallback failed: %v", err)
	}
	if got.ID != rev.ID {
		t.Fatalf("GetRevision returned %q", got.ID)
	}
	index, err = store.loadRevisionIndex("page-1")
	if err != nil {
		t.Fatalf("loadRevisionIndex second failed: %v", err)
	}
	if got := index[rev.ID]; got == "" {
		t.Fatalf("expected revision index backfill for %q, got %#v", rev.ID, index)
	}
}
