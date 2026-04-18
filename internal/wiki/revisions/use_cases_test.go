package revisions_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
)

type testDeps struct {
	storageDir string
	tree       *tree.TreeService
	slug       *tree.SlugService
	revision   *revision.Service
	links      *links.LinkService
	assets     *assets.AssetService
}

func newTestDeps(t *testing.T) *testDeps {
	t.Helper()
	storageDir := t.TempDir()

	treeService := tree.NewTreeService(storageDir)
	if err := treeService.LoadTree(); err != nil {
		t.Fatalf("failed to load tree: %v", err)
	}

	slugService := tree.NewSlugService()
	assetService := assets.NewAssetService(storageDir, slugService)

	linksStore, err := links.NewLinksStore(storageDir)
	if err != nil {
		t.Fatalf("failed to create links store: %v", err)
	}
	linkService := links.NewLinkService(storageDir, treeService, linksStore)

	revService := revision.NewService(storageDir, treeService, nil, revision.ServiceOptions{})

	return &testDeps{
		storageDir: storageDir,
		tree:       treeService,
		slug:       slugService,
		revision:   revService,
		links:      linkService,
		assets:     assetService,
	}
}

func pageKind() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}

func sectionKind() *tree.NodeKind {
	k := tree.NodeKindSection
	return &k
}


func TestRestoreRevisionUseCase_RestoresAssetsAndStructure(t *testing.T) {
	deps := newTestDeps(t)
	createUC := wikipages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, nil)
	updateUC := wikipages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, nil)
	moveUC := wikipages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, nil)
	restoreUC := wikirevisions.NewRestoreRevisionUseCase(deps.revision, deps.tree, deps.links, nil)

	docs, err := createUC.Execute(context.Background(), wikipages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: sectionKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage(docs) failed: %v", err)
	}
	archive, err := createUC.Execute(context.Background(), wikipages.CreatePageInput{
		UserID: "system", Title: "Archive", Slug: "archive", Kind: sectionKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage(archive) failed: %v", err)
	}
	page, err := createUC.Execute(context.Background(), wikipages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "Original", Slug: "original", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage(page) failed: %v", err)
	}

	originalContent := "first version"
	pageOut, err := updateUC.Execute(context.Background(), wikipages.UpdatePageInput{
		UserID: "system", ID: page.Page.ID, Title: "Original", Slug: "original", Content: &originalContent, Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("UpdatePage(original) failed: %v", err)
	}

	assetDir := filepath.Join(deps.assets.GetAssetsDir(), pageOut.Page.ID)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(assetDir) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "old.txt"), []byte("old-asset"), 0o644); err != nil {
		t.Fatalf("WriteFile(old asset) failed: %v", err)
	}
	if _, _, err := deps.revision.RecordAssetChange(pageOut.Page.ID, "system", ""); err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}

	originalRevision, err := deps.revision.GetLatestRevision(pageOut.Page.ID)
	if err != nil || originalRevision == nil {
		t.Fatalf("GetLatestRevision(original) failed: %#v %v", originalRevision, err)
	}

	changedContent := "second version"
	pageOut, err = updateUC.Execute(context.Background(), wikipages.UpdatePageInput{
		UserID: "system", ID: pageOut.Page.ID, Title: "Changed", Slug: "changed", Content: &changedContent, Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("UpdatePage(changed) failed: %v", err)
	}
	if err := moveUC.Execute(context.Background(), wikipages.MovePageInput{
		UserID: "system", ID: pageOut.Page.ID, ParentID: archive.Page.ID,
	}); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}
	if err := os.Remove(filepath.Join(assetDir, "old.txt")); err != nil {
		t.Fatalf("Remove(old asset) failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetDir, "new.txt"), []byte("new-asset"), 0o644); err != nil {
		t.Fatalf("WriteFile(new asset) failed: %v", err)
	}
	if _, _, err := deps.revision.RecordAssetChange(pageOut.Page.ID, "system", ""); err != nil {
		t.Fatalf("RecordAssetChange failed: %v", err)
	}

	restored, err := restoreUC.Execute(context.Background(), wikirevisions.RestoreRevisionInput{
		UserID: "system", PageID: pageOut.Page.ID, RevisionID: originalRevision.ID,
	})
	if err != nil {
		t.Fatalf("RestoreRevision failed: %v", err)
	}
	if restored.Page.Title != "Changed" || restored.Page.Slug != "changed" {
		t.Fatalf("restored identity = (%q,%q)", restored.Page.Title, restored.Page.Slug)
	}
	if restored.Page.CalculatePath() != "/archive/changed" {
		t.Fatalf("restored path = %q", restored.Page.CalculatePath())
	}
	if restored.Page.Content != originalContent {
		t.Fatalf("restored content = %q", restored.Page.Content)
	}

	oldAsset, err := os.ReadFile(filepath.Join(assetDir, "old.txt"))
	if err != nil {
		t.Fatalf("ReadFile(old asset) failed: %v", err)
	}
	if string(oldAsset) != "old-asset" {
		t.Fatalf("old asset = %q", string(oldAsset))
	}
	if _, err := os.Stat(filepath.Join(assetDir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected new asset to be removed, got %v", err)
	}
}
