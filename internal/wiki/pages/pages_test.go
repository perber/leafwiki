package pages_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/test_utils"
	wikiassets "github.com/perber/wiki/internal/wiki/assets"
	"github.com/perber/wiki/internal/wiki/pages"
	wikirevisions "github.com/perber/wiki/internal/wiki/revisions"
)

// testDeps holds real services backed by a temporary directory.
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

	revService := revision.NewService(
		storageDir, treeService, slog.Default(),
		revision.ServiceOptions{},
	)

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

// ─────────────────────────────────────────────────────────────────────────────
// CreatePageUseCase
// ─────────────────────────────────────────────────────────────────────────────

func TestCreatePageUseCase_HappyPath_Root(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	out, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "Home",
		Slug:   "home",
		Kind:   pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Page.Title != "Home" {
		t.Errorf("expected title 'Home', got %q", out.Page.Title)
	}
	if out.Page.Slug != "home" {
		t.Errorf("expected slug 'home', got %q", out.Page.Slug)
	}
}

func TestCreatePageUseCase_HappyPath_WithParent(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	parent, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "Docs",
		Slug:   "docs",
		Kind:   pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating parent: %v", err)
	}

	child, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID:   "user1",
		ParentID: &parent.Page.ID,
		Title:    "Reference",
		Slug:     "reference",
		Kind:     pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating child: %v", err)
	}
	if child.Page.Parent == nil || child.Page.Parent.ID != parent.Page.ID {
		t.Errorf("expected parent ID %q, got %v", parent.Page.ID, child.Page.Parent)
	}
}

func TestCreatePageUseCase_EmptyTitle_ReturnsValidationError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "",
		Slug:   "home",
		Kind:   pageKind(),
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	var ve *sharederrors.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
}

func TestCreatePageUseCase_ReservedSlug_ReturnsValidationError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "Reserved",
		Slug:   "e", // too short / reserved
		Kind:   pageKind(),
	})
	if err == nil {
		t.Fatal("expected error for reserved slug, got nil")
	}
	var ve *sharederrors.ValidationErrors
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
}

func TestCreatePageUseCase_NilKind_ReturnsValidationError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "Test",
		Slug:   "test",
		Kind:   nil,
	})
	if err == nil {
		t.Fatal("expected error for nil kind, got nil")
	}
}

func TestCreatePageUseCase_Section_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	out, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "Section",
		Slug:   "section",
		Kind:   sectionKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Page.Kind != tree.NodeKindSection {
		t.Errorf("expected kind %q, got %q", tree.NodeKindSection, out.Page.Kind)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdatePageUseCase
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdatePageUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	created, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Old Title", Slug: "old-title", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}

	content := "updated content"
	out, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID:  "user1",
		ID:      created.Page.ID,
		Version: created.Page.Version(),
		Title:   "New Title",
		Slug:    "new-title",
		Content: &content,
		Kind:    pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error updating page: %v", err)
	}
	if out.Page.Title != "New Title" {
		t.Errorf("expected title 'New Title', got %q", out.Page.Title)
	}
	if out.Page.Slug != "new-title" {
		t.Errorf("expected slug 'new-title', got %q", out.Page.Slug)
	}
}

func TestUpdatePageUseCase_VersionConflict_ReturnsLocalizedError(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	created, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Old Title", Slug: "old-title", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}
	staleVersion := created.Page.Version()

	firstContent := "first update"
	updated, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID:  "user1",
		ID:      created.Page.ID,
		Version: staleVersion,
		Title:   "New Title",
		Slug:    "new-title",
		Content: &firstContent,
		Kind:    pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error applying first update: %v", err)
	}

	secondContent := "second update"
	_, err = updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID:  "user2",
		ID:      created.Page.ID,
		Version: staleVersion,
		Title:   updated.Page.Title,
		Slug:    updated.Page.Slug,
		Content: &secondContent,
		Kind:    pageKind(),
	})
	if err == nil {
		t.Fatal("expected version conflict, got nil")
	}
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized error, got %T: %v", err, err)
	}
	if loc.Code != pages.ErrCodePageVersionConflict {
		t.Fatalf("expected %q, got %q", pages.ErrCodePageVersionConflict, loc.Code)
	}
}

func TestUpdatePageUseCase_EmptyTitle_ReturnsValidationError(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	created, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Page", Slug: "page", Kind: pageKind(),
	})

	_, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: created.Page.ID, Version: created.Page.Version(), Title: "", Slug: "page", Kind: pageKind(),
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// DeletePageUseCase
// ─────────────────────────────────────────────────────────────────────────────

func TestDeletePageUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	deleteUC := pages.NewDeletePageUseCase(deps.tree, deps.revision, deps.links, deps.assets, slog.Default())

	created, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "To Delete", Slug: "to-delete", Kind: pageKind(),
	})

	if err := deleteUC.Execute(context.Background(), pages.DeletePageInput{
		UserID:    "user1",
		ID:        created.Page.ID,
		Version:   created.Page.Version(),
		Recursive: false,
	}); err != nil {
		t.Fatalf("unexpected error deleting page: %v", err)
	}

	// Verify it is gone
	if _, err := deps.tree.GetPage(created.Page.ID); !errors.Is(err, tree.ErrPageNotFound) {
		t.Errorf("expected page-not-found after delete, got %v", err)
	}
}

func TestDeletePageUseCase_Root_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	deleteUC := pages.NewDeletePageUseCase(deps.tree, deps.revision, deps.links, deps.assets, slog.Default())

	err := deleteUC.Execute(context.Background(), pages.DeletePageInput{
		UserID: "user1", ID: "root", Recursive: false,
	})
	if err == nil {
		t.Fatal("expected error when deleting root, got nil")
	}
}

func TestDeletePageUseCase_WithChildren_Recursive(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	deleteUC := pages.NewDeletePageUseCase(deps.tree, deps.revision, deps.links, deps.assets, slog.Default())

	parent, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent", Slug: "parent", Kind: pageKind(),
	})
	createUC.Execute(context.Background(), pages.CreatePageInput{ //nolint:errcheck
		UserID: "user1", ParentID: &parent.Page.ID, Title: "Child", Slug: "child", Kind: pageKind(),
	})

	err := deleteUC.Execute(context.Background(), pages.DeletePageInput{
		UserID: "user1", ID: parent.Page.ID, Version: parent.Page.Version(), Recursive: true,
	})
	if err != nil {
		t.Fatalf("unexpected error on recursive delete: %v", err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MovePageUseCase
// ─────────────────────────────────────────────────────────────────────────────

func TestMovePageUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	parent, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent", Slug: "parent", Kind: pageKind(),
	})
	child, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Child", Slug: "child", Kind: pageKind(),
	})

	if err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID:   "user1",
		ID:       child.Page.ID,
		Version:  child.Page.Version(),
		ParentID: parent.Page.ID,
	}); err != nil {
		t.Fatalf("unexpected error moving page: %v", err)
	}

	moved, err := deps.tree.GetPage(child.Page.ID)
	if err != nil {
		t.Fatalf("could not get moved page: %v", err)
	}
	if moved.Parent == nil || moved.Parent.ID != parent.Page.ID {
		t.Errorf("expected parent %q after move, got %v", parent.Page.ID, moved.Parent)
	}
}

func TestMovePageUseCase_VersionConflict_ReturnsLocalizedError(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	parentA, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent A", Slug: "parent-a", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating parent A: %v", err)
	}
	parentB, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent B", Slug: "parent-b", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating parent B: %v", err)
	}
	parentC, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent C", Slug: "parent-c", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating parent C: %v", err)
	}
	child, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID:   "user1",
		ParentID: &parentA.Page.ID,
		Title:    "Child",
		Slug:     "child",
		Kind:     pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating child: %v", err)
	}
	staleVersion := child.Page.Version()

	if err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID:   "user1",
		ID:       child.Page.ID,
		Version:  staleVersion,
		ParentID: parentB.Page.ID,
	}); err != nil {
		t.Fatalf("unexpected error applying first move: %v", err)
	}

	err = moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID:   "user2",
		ID:       child.Page.ID,
		Version:  staleVersion,
		ParentID: parentC.Page.ID,
	})
	if err == nil {
		t.Fatal("expected version conflict, got nil")
	}
	loc, ok := sharederrors.AsLocalizedError(err)
	if !ok {
		t.Fatalf("expected localized error, got %T: %v", err, err)
	}
	if loc.Code != pages.ErrCodePageVersionConflict {
		t.Fatalf("expected %q, got %q", pages.ErrCodePageVersionConflict, loc.Code)
	}
}

func TestMovePageUseCase_Root_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID: "user1", ID: "root", Version: "root-version", ParentID: "root",
	})
	if err == nil {
		t.Fatal("expected error when moving root, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// EnsurePathUseCase
// ─────────────────────────────────────────────────────────────────────────────

func TestEnsurePathUseCase_CreatesNewPath(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewEnsurePathUseCase(deps.tree, deps.slug, nil, deps.links, slog.Default())

	out, err := uc.Execute(context.Background(), pages.EnsurePathInput{
		UserID:      "user1",
		TargetPath:  "docs/reference",
		TargetTitle: "Reference",
		Kind:        pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Page == nil {
		t.Fatal("expected page in output, got nil")
	}
}

func TestEnsurePathUseCase_ExistingPath_ReturnsExistingPage(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewEnsurePathUseCase(deps.tree, deps.slug, nil, deps.links, slog.Default())

	out1, err := uc.Execute(context.Background(), pages.EnsurePathInput{
		UserID: "user1", TargetPath: "docs", TargetTitle: "Docs", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error on first create: %v", err)
	}

	out2, err := uc.Execute(context.Background(), pages.EnsurePathInput{
		UserID: "user1", TargetPath: "docs", TargetTitle: "Docs", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error on second ensure: %v", err)
	}
	if out1.Page.ID != out2.Page.ID {
		t.Errorf("expected same page ID, got %q vs %q", out1.Page.ID, out2.Page.ID)
	}
}

func TestEnsurePathUseCase_EmptyPath_ReturnsValidationError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewEnsurePathUseCase(deps.tree, deps.slug, nil, deps.links, slog.Default())

	_, err := uc.Execute(context.Background(), pages.EnsurePathInput{
		UserID: "user1", TargetPath: "", TargetTitle: "Title", Kind: pageKind(),
	})
	if err == nil {
		t.Fatal("expected validation error for empty path, got nil")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetPageUseCase
// ─────────────────────────────────────────────────────────────────────────────

func TestGetPageUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	getUC := pages.NewGetPageUseCase(deps.tree)

	created, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Home", Slug: "home", Kind: pageKind(),
	})

	out, err := getUC.Execute(context.Background(), pages.GetPageInput{ID: created.Page.ID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Page.ID != created.Page.ID {
		t.Errorf("expected ID %q, got %q", created.Page.ID, out.Page.ID)
	}
}

func TestGetPageUseCase_NotFound_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	getUC := pages.NewGetPageUseCase(deps.tree)

	_, err := getUC.Execute(context.Background(), pages.GetPageInput{ID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent page, got nil")
	}
}

func TestCreatePageUseCase_ReservedHistorySlug_ReturnsValidationError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1",
		Title:  "Reserved",
		Slug:   "history",
		Kind:   pageKind(),
	})
	if err == nil {
		t.Fatal("expected error for reserved history slug, got nil")
	}
}

func TestCreatePageUseCase_PageExists_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	if _, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Duplicate", Slug: "duplicate", Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error creating initial page: %v", err)
	}

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Duplicate", Slug: "duplicate", Kind: pageKind(),
	})
	if err == nil {
		t.Fatal("expected duplicate page error, got nil")
	}
}

func TestCreatePageUseCase_InvalidParent_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	invalidID := "not-real"

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &invalidID, Title: "Broken", Slug: "broken", Kind: pageKind(),
	})
	if err == nil {
		t.Fatal("expected invalid parent error, got nil")
	}
}

func TestCreatePageUseCase_RejectsCaseInsensitiveSlugConflict(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	if _, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Upper", Slug: "ABCD-efg", Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error creating initial page: %v", err)
	}

	_, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Lower", Slug: "abcd-efg", Kind: pageKind(),
	})
	if err == nil {
		t.Fatal("expected conflict for case-insensitive duplicate slug")
	}
}

func TestCreatePageUseCase_RecordsPageCreatedRevision(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	out, err := uc.Execute(context.Background(), pages.CreatePageInput{
		UserID: "editor", Title: "My Page", Slug: "my-page", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	latest, err := deps.revision.GetLatestRevision(out.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected latest revision, got nil")
	}
	if latest.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("revision type = %q, want %q", latest.Type, revision.RevisionTypeContentUpdate)
	}
	if latest.Summary != "page created" {
		t.Fatalf("revision summary = %q, want %q", latest.Summary, "page created")
	}
	if latest.AuthorID != "editor" {
		t.Fatalf("revision authorID = %q, want %q", latest.AuthorID, "editor")
	}
}

func TestUpdatePageUseCase_AllowsUppercaseSlug(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	created, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Original", Slug: "original", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}

	content := "# Updated"
	out, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID:  "user1",
		ID:      created.Page.ID,
		Version: created.Page.Version(),
		Title:   "Original",
		Slug:    "ABCD-efg",
		Content: &content,
		Kind:    pageKind(),
	})
	if err != nil {
		t.Fatalf("expected uppercase slug update to succeed, got %v", err)
	}
	if out.Page.Slug != "ABCD-efg" {
		t.Fatalf("expected slug to be preserved, got %q", out.Page.Slug)
	}
}

func TestDeletePageUseCase_EmptyID_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	deleteUC := pages.NewDeletePageUseCase(deps.tree, deps.revision, deps.links, deps.assets, slog.Default())

	err := deleteUC.Execute(context.Background(), pages.DeletePageInput{
		UserID: "user1", ID: "", Recursive: false,
	})
	if err == nil {
		t.Fatal("expected error when deleting empty page ID, got nil")
	}
}

func TestFindByPathUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	findUC := pages.NewFindByPathUseCase(deps.tree)

	if _, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Company", Slug: "company", Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}

	out, err := findUC.Execute(context.Background(), pages.FindByPathInput{RoutePath: "company"})
	if err != nil {
		t.Fatalf("unexpected error finding page: %v", err)
	}
	if out.Page.Slug != "company" {
		t.Errorf("expected slug 'company', got %q", out.Page.Slug)
	}
}

func TestFindByPathUseCase_NotFound_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	findUC := pages.NewFindByPathUseCase(deps.tree)

	_, err := findUC.Execute(context.Background(), pages.FindByPathInput{RoutePath: "does/not/exist"})
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestSortPagesUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	sortUC := pages.NewSortPagesUseCase(deps.tree)

	parent, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent", Slug: "parent", Kind: pageKind(),
	})
	child1, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &parent.Page.ID, Title: "Child1", Slug: "child1", Kind: pageKind(),
	})
	child2, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &parent.Page.ID, Title: "Child2", Slug: "child2", Kind: pageKind(),
	})

	if err := sortUC.Execute(context.Background(), pages.SortPagesInput{
		ParentID: parent.Page.ID, OrderedIDs: []string{child2.Page.ID, child1.Page.ID},
	}); err != nil {
		t.Fatalf("unexpected error sorting pages: %v", err)
	}

	sortedParent, err := deps.tree.GetPage(parent.Page.ID)
	if err != nil {
		t.Fatalf("failed to reload parent: %v", err)
	}
	if len(sortedParent.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(sortedParent.Children))
	}
	if sortedParent.Children[0].ID != child2.Page.ID || sortedParent.Children[1].ID != child1.Page.ID {
		t.Errorf("expected order [%s, %s], got [%s, %s]", child2.Page.ID, child1.Page.ID, sortedParent.Children[0].ID, sortedParent.Children[1].ID)
	}
}

func TestSuggestSlugUseCase_Unique(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewSuggestSlugUseCase(deps.tree, deps.slug)

	out, err := uc.Execute(context.Background(), pages.SuggestSlugInput{
		ParentID: "root",
		Title:    "My Page",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Slug != "my-page" {
		t.Errorf("expected 'my-page', got %q", out.Slug)
	}
}

func TestSuggestSlugUseCase_Conflict(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	uc := pages.NewSuggestSlugUseCase(deps.tree, deps.slug)

	if _, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "My Page", Slug: "my-page", Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}

	out, err := uc.Execute(context.Background(), pages.SuggestSlugInput{
		ParentID: deps.tree.GetTree().ID,
		Title:    "My Page",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Slug != "my-page-1" {
		t.Errorf("expected 'my-page-1', got %q", out.Slug)
	}
}

func TestSuggestSlugUseCase_DeepHierarchy(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	uc := pages.NewSuggestSlugUseCase(deps.tree, deps.slug)

	arch, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Architecture", Slug: "architecture", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating architecture: %v", err)
	}
	backend, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &arch.Page.ID, Title: "Backend", Slug: "backend", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating backend: %v", err)
	}

	out, err := uc.Execute(context.Background(), pages.SuggestSlugInput{
		ParentID: backend.Page.ID,
		Title:    "Data Layer",
	})
	if err != nil {
		t.Fatalf("unexpected error suggesting slug: %v", err)
	}
	if out.Slug != "data-layer" {
		t.Errorf("expected 'data-layer', got %q", out.Slug)
	}

	if _, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", ParentID: &backend.Page.ID, Title: "Data Layer", Slug: "data-layer", Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error creating duplicate title page: %v", err)
	}

	out2, err := uc.Execute(context.Background(), pages.SuggestSlugInput{
		ParentID: backend.Page.ID,
		Title:    "Data Layer",
	})
	if err != nil {
		t.Fatalf("unexpected error suggesting second slug: %v", err)
	}
	if out2.Slug != "data-layer-1" {
		t.Errorf("expected 'data-layer-1', got %q", out2.Slug)
	}
}

func TestCopyPageUseCase_HappyPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	copyUC := pages.NewCopyPageUseCase(deps.tree, deps.slug, deps.revision, deps.links, deps.assets, slog.Default())

	original, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Original", Slug: "original", Kind: pageKind(),
	})

	out, err := copyUC.Execute(context.Background(), pages.CopyPageInput{
		UserID: "user1", SourcePageID: original.Page.ID, Title: "Copy of Original", Slug: "copy-of-original",
	})
	if err != nil {
		t.Fatalf("unexpected error copying page: %v", err)
	}
	if out.Page.Title != "Copy of Original" {
		t.Errorf("expected title 'Copy of Original', got %q", out.Page.Title)
	}
	if out.Page.Slug != "copy-of-original" {
		t.Errorf("expected slug 'copy-of-original', got %q", out.Page.Slug)
	}
	if out.Page.ID == original.Page.ID {
		t.Error("expected copied page to have a different ID")
	}
}

func TestCopyPageUseCase_WithParent(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	copyUC := pages.NewCopyPageUseCase(deps.tree, deps.slug, deps.revision, deps.links, deps.assets, slog.Default())

	parent, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Parent", Slug: "parent", Kind: pageKind(),
	})
	original, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Original", Slug: "original", Kind: pageKind(),
	})

	out, err := copyUC.Execute(context.Background(), pages.CopyPageInput{
		UserID: "user1", SourcePageID: original.Page.ID, TargetParentID: &parent.Page.ID, Title: "Copy of Original", Slug: "copy-of-original",
	})
	if err != nil {
		t.Fatalf("unexpected error copying page: %v", err)
	}
	if out.Page.Parent == nil || out.Page.Parent.ID != parent.Page.ID {
		t.Errorf("expected parent ID %q, got %v", parent.Page.ID, out.Page.Parent)
	}
}

func TestCopyPageUseCase_NonExistentSource_ReturnsError(t *testing.T) {
	deps := newTestDeps(t)
	copyUC := pages.NewCopyPageUseCase(deps.tree, deps.slug, deps.revision, deps.links, deps.assets, slog.Default())

	_, err := copyUC.Execute(context.Background(), pages.CopyPageInput{
		UserID: "user1", SourcePageID: "non-existent-id", Title: "Copy", Slug: "copy",
	})
	if err == nil {
		t.Fatal("expected error for non-existent source page, got nil")
	}
}

func TestCopyPageUseCase_WithAssets(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	copyUC := pages.NewCopyPageUseCase(deps.tree, deps.slug, deps.revision, deps.links, deps.assets, slog.Default())

	original, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Original", Slug: "original", Kind: pageKind(),
	})

	file, _, err := test_utils.CreateMultipartFile("image.png", []byte("image content"))
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(file.Close, t)

	if _, err := deps.assets.SaveAssetForPage(original.Page.PageNode, file, "image.png", 1024); err != nil {
		t.Fatalf("Failed to save asset for original page: %v", err)
	}

	out, err := copyUC.Execute(context.Background(), pages.CopyPageInput{
		UserID: "user1", SourcePageID: original.Page.ID, Title: "Copy of Original", Slug: "copy-of-original",
	})
	if err != nil {
		t.Fatalf("unexpected error copying page: %v", err)
	}

	copiedAssets, err := deps.assets.ListAssetsForPage(out.Page.PageNode)
	if err != nil {
		t.Fatalf("Failed to list assets for copied page: %v", err)
	}
	if len(copiedAssets) != 1 {
		t.Errorf("expected 1 asset for copied page, got %d", len(copiedAssets))
	}
}

func TestCopyPageUseCase_RecordsContentRevision(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	copyUC := pages.NewCopyPageUseCase(deps.tree, deps.slug, deps.revision, deps.links, deps.assets, slog.Default())

	original, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "user1", Title: "Original", Slug: "original", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("unexpected error creating page: %v", err)
	}

	content := "original content"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "user1", ID: original.Page.ID, Version: original.Page.Version(), Title: original.Page.Title, Slug: original.Page.Slug, Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("unexpected error updating page: %v", err)
	}

	out, err := copyUC.Execute(context.Background(), pages.CopyPageInput{
		UserID: "editor", SourcePageID: original.Page.ID, Title: "Copy of Original", Slug: "copy-of-original",
	})
	if err != nil {
		t.Fatalf("unexpected error copying page: %v", err)
	}

	latest, err := deps.revision.GetLatestRevision(out.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil {
		t.Fatal("expected latest revision for copied page")
	}
	if latest.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("latest revision type = %q, want %q", latest.Type, revision.RevisionTypeContentUpdate)
	}
	if latest.AuthorID != "editor" {
		t.Fatalf("latest author = %q, want %q", latest.AuthorID, "editor")
	}
	if latest.Summary != "page copied" {
		t.Fatalf("latest summary = %q, want %q", latest.Summary, "page copied")
	}
}

func TestPreviewPageRefactorUseCase_RenameListsAffectedPages(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	previewUC := pages.NewPreviewPageRefactorUseCase(deps.tree, deps.slug, deps.links, slog.Default())

	target, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Target", Slug: "target", Kind: pageKind(),
	})
	ref, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Ref", Slug: "ref", Kind: pageKind(),
	})
	content := "[Target](/target)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: ref.Page.ID, Version: ref.Page.Version(), Title: ref.Page.Title, Slug: ref.Page.Slug, Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	preview, err := previewUC.Execute(context.Background(), pages.RefactorPreviewInput{
		PageID: target.Page.ID,
		Kind:   pages.RefactorKindRename,
		Title:  target.Page.Title,
		Slug:   "target-renamed",
	})
	if err != nil {
		t.Fatalf("PreviewPageRefactor failed: %v", err)
	}
	if preview.OldPath != "/target" {
		t.Fatalf("OldPath = %q, want %q", preview.OldPath, "/target")
	}
	if preview.NewPath != "/target-renamed" {
		t.Fatalf("NewPath = %q, want %q", preview.NewPath, "/target-renamed")
	}
	if preview.Counts.AffectedPages != 1 {
		t.Fatalf("AffectedPages = %d, want 1", preview.Counts.AffectedPages)
	}
	if len(preview.AffectedPages) != 1 {
		t.Fatalf("expected 1 affected page, got %d", len(preview.AffectedPages))
	}
	if preview.AffectedPages[0].FromPageID != ref.Page.ID {
		t.Fatalf("FromPageID = %q, want %q", preview.AffectedPages[0].FromPageID, ref.Page.ID)
	}
}

func TestApplyPageRefactorUseCase_RenameRewritesIncomingLinks(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	applyUC := pages.NewApplyPageRefactorUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	target, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Target", Slug: "target", Kind: pageKind(),
	})
	ref, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Ref", Slug: "ref", Kind: pageKind(),
	})
	content := "[Target](/target)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: ref.Page.ID, Version: ref.Page.Version(), Title: ref.Page.Title, Slug: ref.Page.Slug, Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	beforeRefRevision, err := deps.revision.GetLatestRevision(ref.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(ref before refactor) failed: %v", err)
	}

	updated, err := applyUC.Execute(context.Background(), pages.RefactorApplyInput{
		UserID:  "system",
		Version: target.Page.Version(),
		RefactorPreviewInput: pages.RefactorPreviewInput{
			PageID:  target.Page.ID,
			Kind:    pages.RefactorKindRename,
			Title:   "Target Renamed",
			Slug:    "target-renamed",
			Content: &target.Page.Content,
		},
		RewriteLinks: true,
	})
	if err != nil {
		t.Fatalf("ApplyPageRefactor failed: %v", err)
	}
	if updated.CalculatePath() != "/target-renamed" {
		t.Fatalf("updated path mismatch: %q", updated.CalculatePath())
	}

	refPage, err := deps.tree.GetPage(ref.Page.ID)
	if err != nil {
		t.Fatalf("GetPage(ref) failed: %v", err)
	}
	if refPage.Content != "[Target](/target-renamed)" {
		t.Fatalf("ref content = %q, want %q", refPage.Content, "[Target](/target-renamed)")
	}

	outgoing, err := deps.links.GetOutgoingLinksForPage(ref.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if outgoing.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d", outgoing.Count)
	}
	if outgoing.Outgoings[0].ToPath != "/target-renamed" {
		t.Fatalf("ToPath = %q, want %q", outgoing.Outgoings[0].ToPath, "/target-renamed")
	}
	if outgoing.Outgoings[0].Broken {
		t.Fatalf("expected rewritten link to be healed")
	}

	afterRefRevision, err := deps.revision.GetLatestRevision(ref.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(ref after refactor) failed: %v", err)
	}
	if afterRefRevision == nil || beforeRefRevision == nil {
		t.Fatalf("expected revisions before and after refactor")
	}
	if afterRefRevision.ID == beforeRefRevision.ID {
		t.Fatalf("expected rewritten ref page to create a new revision")
	}
	if afterRefRevision.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("expected rewritten ref page latest revision type %q, got %q", revision.RevisionTypeContentUpdate, afterRefRevision.Type)
	}
}

func TestPreviewPageRefactorUseCase_UsesEmptyWarningArrays(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	previewUC := pages.NewPreviewPageRefactorUseCase(deps.tree, deps.slug, deps.links, slog.Default())

	page, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Target", Slug: "target", Kind: pageKind(),
	})

	preview, err := previewUC.Execute(context.Background(), pages.RefactorPreviewInput{
		PageID: page.Page.ID,
		Kind:   pages.RefactorKindRename,
		Title:  page.Page.Title,
		Slug:   "target-renamed",
	})
	if err != nil {
		t.Fatalf("PreviewPageRefactor failed: %v", err)
	}
	if preview.Warnings == nil {
		t.Fatalf("expected preview warnings to be an empty slice, got nil")
	}
	if len(preview.Warnings) != 0 {
		t.Fatalf("expected no preview warnings, got %d", len(preview.Warnings))
	}
	for i, affected := range preview.AffectedPages {
		if affected.Warnings == nil {
			t.Fatalf("affected page %d warnings should be empty slice, got nil", i)
		}
		if affected.MatchedPaths == nil {
			t.Fatalf("affected page %d matched paths should be empty slice, got nil", i)
		}
	}
}

func TestPreviewPageRefactorUseCase_Move_ExcludesMovedSubtreeFromOptionalAffectedPages(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	previewUC := pages.NewPreviewPageRefactorUseCase(deps.tree, deps.slug, deps.links, slog.Default())

	docs, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: pageKind(),
	})
	pageA, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "Page A", Slug: "page-a", Kind: pageKind(),
	})
	pageB, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "Page B", Slug: "page-b", Kind: pageKind(),
	})
	archive, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Archive", Slug: "archive", Kind: pageKind(),
	})

	contentA := "[To B](../page-b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: pageA.Page.ID, Version: pageA.Page.Version(), Title: pageA.Page.Title, Slug: pageA.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage(pageA) failed: %v", err)
	}

	preview, err := previewUC.Execute(context.Background(), pages.RefactorPreviewInput{
		PageID:      pageA.Page.ID,
		Kind:        pages.RefactorKindMove,
		NewParentID: &archive.Page.ID,
	})
	if err != nil {
		t.Fatalf("PreviewPageRefactor failed: %v", err)
	}
	if preview.Counts.AffectedPages != 0 {
		t.Fatalf("expected no optional affected pages, got %d", preview.Counts.AffectedPages)
	}
	if len(preview.AffectedPages) != 0 {
		t.Fatalf("expected no affected pages, got %d", len(preview.AffectedPages))
	}

	_ = pageB
}

func TestApplyPageRefactorUseCase_Move_RewritesRelativeOutgoingLinksInMovedPage(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	applyUC := pages.NewApplyPageRefactorUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	docs, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: pageKind(),
	})
	pageA, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "Page A", Slug: "page-a", Kind: pageKind(),
	})
	pageB, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "Page B", Slug: "page-b", Kind: pageKind(),
	})
	archive, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Archive", Slug: "archive", Kind: pageKind(),
	})

	contentA := "[To B](../page-b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: pageA.Page.ID, Version: pageA.Page.Version(), Title: pageA.Page.Title, Slug: pageA.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage(pageA) failed: %v", err)
	}

	beforeMovedRevision, err := deps.revision.GetLatestRevision(pageA.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(pageA before refactor) failed: %v", err)
	}

	updated, err := applyUC.Execute(context.Background(), pages.RefactorApplyInput{
		UserID:  "system",
		Version: pageA.Page.Version(),
		RefactorPreviewInput: pages.RefactorPreviewInput{
			PageID:      pageA.Page.ID,
			Kind:        pages.RefactorKindMove,
			NewParentID: &archive.Page.ID,
		},
		RewriteLinks: false,
	})
	if err != nil {
		t.Fatalf("ApplyPageRefactor(move) failed: %v", err)
	}
	if updated.CalculatePath() != "/archive/page-a" {
		t.Fatalf("updated path = %q, want %q", updated.CalculatePath(), "/archive/page-a")
	}

	movedPage, err := deps.tree.GetPage(pageA.Page.ID)
	if err != nil {
		t.Fatalf("GetPage(pageA) failed: %v", err)
	}
	if movedPage.Content != "[To B](../../docs/page-b)" {
		t.Fatalf("moved page content = %q, want %q", movedPage.Content, "[To B](../../docs/page-b)")
	}

	outgoing, err := deps.links.GetOutgoingLinksForPage(pageA.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(pageA) failed: %v", err)
	}
	if outgoing.Count != 1 {
		t.Fatalf("expected 1 outgoing link, got %d", outgoing.Count)
	}
	if outgoing.Outgoings[0].ToPageID != pageB.Page.ID {
		t.Fatalf("ToPageID = %q, want %q", outgoing.Outgoings[0].ToPageID, pageB.Page.ID)
	}
	if outgoing.Outgoings[0].ToPath != "/docs/page-b" {
		t.Fatalf("ToPath = %q, want %q", outgoing.Outgoings[0].ToPath, "/docs/page-b")
	}
	if outgoing.Outgoings[0].Broken {
		t.Fatalf("expected outgoing link to remain valid after move refactor")
	}

	afterMovedRevision, err := deps.revision.GetLatestRevision(pageA.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(pageA after refactor) failed: %v", err)
	}
	if afterMovedRevision == nil || beforeMovedRevision == nil {
		t.Fatalf("expected revisions before and after move")
	}
	if afterMovedRevision.ID == beforeMovedRevision.ID {
		t.Fatalf("expected moved page rewrite to create a new revision")
	}
	if afterMovedRevision.Type != revision.RevisionTypeContentUpdate {
		t.Fatalf("expected moved page latest revision type %q, got %q", revision.RevisionTypeContentUpdate, afterMovedRevision.Type)
	}
}

func TestEnsurePathUseCase_HealsLinksForAllCreatedSegments(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	ensureUC := pages.NewEnsurePathUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	pageA, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Page A", Slug: "a", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}

	contentA := "Links: [X](/x) and [XY](/x/y)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: pageA.Page.ID, Version: pageA.Page.Version(), Title: pageA.Page.Title, Slug: pageA.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	out1, err := deps.links.GetOutgoingLinksForPage(pageA.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if out1.Count != 2 {
		t.Fatalf("expected 2 outgoings before ensure, got %d: %#v", out1.Count, out1.Outgoings)
	}

	byPath := map[string]bool{}
	for _, it := range out1.Outgoings {
		byPath[it.ToPath] = it.Broken
	}
	if broken, ok := byPath["/x"]; !ok || !broken {
		t.Fatalf("expected /x to be broken before ensure, got map=%#v, out=%#v", byPath, out1.Outgoings)
	}
	if broken, ok := byPath["/x/y"]; !ok || !broken {
		t.Fatalf("expected /x/y to be broken before ensure, got map=%#v, out=%#v", byPath, out1.Outgoings)
	}

	if _, err := ensureUC.Execute(context.Background(), pages.EnsurePathInput{
		UserID: "system", TargetPath: "/x/y", TargetTitle: "X Y", Kind: pageKind(),
	}); err != nil {
		t.Fatalf("EnsurePath failed: %v", err)
	}

	out2, err := deps.links.GetOutgoingLinksForPage(pageA.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks (after ensure) failed: %v", err)
	}
	if out2.Count != 2 {
		t.Fatalf("expected 2 outgoings after ensure, got %d: %#v", out2.Count, out2.Outgoings)
	}

	var gotX, gotXY *struct {
		broken bool
		toPage string
	}
	for _, it := range out2.Outgoings {
		if it.ToPath == "/x" {
			gotX = &struct {
				broken bool
				toPage string
			}{it.Broken, it.ToPageID}
		}
		if it.ToPath == "/x/y" {
			gotXY = &struct {
				broken bool
				toPage string
			}{it.Broken, it.ToPageID}
		}
	}

	if gotX == nil || gotX.broken || gotX.toPage == "" {
		t.Fatalf("expected /x healed with ToPageID, got %#v", out2.Outgoings)
	}
	if gotXY == nil || gotXY.broken || gotXY.toPage == "" {
		t.Fatalf("expected /x/y healed with ToPageID, got %#v", out2.Outgoings)
	}
}

func TestDeletePageUseCase_NonRecursive_MarksIncomingBroken(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	deleteUC := pages.NewDeletePageUseCase(deps.tree, deps.revision, deps.links, deps.assets, slog.Default())

	a, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Page A", Slug: "a", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage A failed: %v", err)
	}
	contentA := "Link to B: [Go](/b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	b, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Page B", Slug: "b", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage B failed: %v", err)
	}
	contentB := "# Page B"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: b.Page.Slug, Content: &contentB, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}
	if err := deleteUC.Execute(context.Background(), pages.DeletePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Recursive: false,
	}); err != nil {
		t.Fatalf("DeletePage failed: %v", err)
	}

	out, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks failed: %v", err)
	}
	if out.Count != 1 {
		t.Fatalf("expected 1 outgoing, got %d", out.Count)
	}
	got := out.Outgoings[0]
	if got.ToPath != "/b" || !got.Broken || got.ToPageID != "" {
		t.Fatalf("unexpected outgoing after delete: %#v", got)
	}

	bl, err := deps.links.GetBacklinksForPage(b.Page.ID)
	if err != nil {
		t.Fatalf("GetBacklinks failed: %v", err)
	}
	if bl.Count != 0 {
		t.Fatalf("expected 0 backlinks after delete, got %d", bl.Count)
	}
}

func TestDeletePageUseCase_Recursive_RemovesOutgoingForSubtree_AndBreaksIncomingByPrefix(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	deleteUC := pages.NewDeletePageUseCase(deps.tree, deps.revision, deps.links, deps.assets, slog.Default())

	docs, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: pageKind(),
	})
	a, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "A", Slug: "a", Kind: pageKind(),
	})
	b, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "B", Slug: "b", Kind: pageKind(),
	})

	contentA := "Link to B: [B](/docs/b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage a failed: %v", err)
	}
	contentB := "# B"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: b.Page.Slug, Content: &contentB, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage b failed: %v", err)
	}

	c, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "C", Slug: "c", Kind: pageKind(),
	})
	contentC := "Incoming link: [B](/docs/b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: c.Page.ID, Version: c.Page.Version(), Title: c.Page.Title, Slug: c.Page.Slug, Content: &contentC, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage c failed: %v", err)
	}

	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}
	outA, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || outA.Count != 1 {
		t.Fatalf("expected 1 outgoing from a before delete, got err=%v out=%#v", err, outA)
	}

	if err := deleteUC.Execute(context.Background(), pages.DeletePageInput{
		UserID: "system", ID: docs.Page.ID, Version: docs.Page.Version(), Recursive: true,
	}); err != nil {
		t.Fatalf("DeletePage(docs, recursive) failed: %v", err)
	}

	outAAfter, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(a) after delete failed: %v", err)
	}
	if outAAfter.Count != 0 {
		t.Fatalf("expected 0 outgoing from deleted page a, got %d", outAAfter.Count)
	}

	outC, err := deps.links.GetOutgoingLinksForPage(c.Page.ID)
	if err != nil {
		t.Fatalf("GetOutgoingLinks(c) after delete failed: %v", err)
	}
	if outC.Count != 1 {
		t.Fatalf("expected 1 outgoing from c, got %d", outC.Count)
	}
	got := outC.Outgoings[0]
	if got.ToPath != "/docs/b" || !got.Broken || got.ToPageID != "" {
		t.Fatalf("unexpected outgoing after recursive delete: %#v", got)
	}
}

func TestUpdatePageUseCase_RenamePage_MarksOldBroken_HealsNewExactPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	a, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "A", Slug: "a", Kind: pageKind(),
	})
	contentA := "Links: [B](/b) and [B2](/b2)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	b, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "B", Slug: "b", Kind: pageKind(),
	})
	contentB := "# B"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: b.Page.Slug, Content: &contentB, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}
	out1, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out1.Count != 2 {
		t.Fatalf("unexpected outgoing before rename err=%v out=%#v", err, out1)
	}

	contentB2 := "# B (renamed)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: "b2", Content: &contentB2, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("Rename B failed: %v", err)
	}

	out2, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out2.Count != 2 {
		t.Fatalf("unexpected outgoing after rename err=%v out=%#v", err, out2)
	}
	byPath := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out2.Outgoings {
		byPath[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}
	if got, ok := byPath["/b"]; !ok || !got.broken || got.toID != "" {
		t.Fatalf("expected /b broken after rename, got %#v", byPath)
	}
	if got, ok := byPath["/b2"]; !ok || got.broken || got.toID != b.Page.ID {
		t.Fatalf("expected /b2 healed to %q, got %#v", b.Page.ID, byPath)
	}
}

func TestUpdatePageUseCase_RenameSubtree_BreaksOldPrefix_HealsNewSubpaths(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	docs, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: pageKind(),
	})
	b, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "B", Slug: "b", Kind: pageKind(),
	})
	contentB := "# B"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: b.Page.Slug, Content: &contentB, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	a, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "A", Slug: "a", Kind: pageKind(),
	})
	contentA := "Links: [Old](/docs/b) and [New](/docs2/b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	contentDocs2 := "# Docs"
	nodeSection := tree.NodeKindSection
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: docs.Page.ID, Version: docs.Page.Version(), Title: docs.Page.Title, Slug: "docs2", Content: &contentDocs2, Kind: &nodeSection,
	}); err != nil {
		t.Fatalf("Rename docs failed: %v", err)
	}

	out2, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out2.Count != 2 {
		t.Fatalf("unexpected outgoing after subtree rename err=%v out=%#v", err, out2)
	}
	byPath := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out2.Outgoings {
		byPath[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}
	if got, ok := byPath["/docs/b"]; !ok || !got.broken || got.toID != "" {
		t.Fatalf("expected /docs/b broken, got %#v", byPath)
	}
	if got, ok := byPath["/docs2/b"]; !ok || got.broken || got.toID != b.Page.ID {
		t.Fatalf("expected /docs2/b healed to %q, got %#v", b.Page.ID, byPath)
	}
}

func TestMovePageUseCase_MarksOldBroken_HealsNewExactPath(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	a, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "A", Slug: "a", Kind: pageKind(),
	})
	contentA := "Links: [B](/b) and [B2](/projects/b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}

	b, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "B", Slug: "b", Kind: pageKind(),
	})
	contentB := "# B"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: b.Page.Slug, Content: &contentB, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	projects, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Projects", Slug: "projects", Kind: pageKind(),
	})
	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	if err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), ParentID: projects.Page.ID,
	}); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	out2, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out2.Count != 2 {
		t.Fatalf("unexpected outgoing after move err=%v out=%#v", err, out2)
	}
	state := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out2.Outgoings {
		state[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}
	if got := state["/b"]; !got.broken || got.toID != "" {
		t.Fatalf("expected /b broken after move, got %#v", state)
	}
	if got := state["/projects/b"]; got.broken || got.toID != b.Page.ID {
		t.Fatalf("expected /projects/b healed to %q, got %#v", b.Page.ID, state)
	}
}

func TestMovePageUseCase_MoveSubtree_BreaksOldPrefix_HealsNewSubpaths(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	docs, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: pageKind(),
	})
	b, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "B", Slug: "b", Kind: pageKind(),
	})
	contentB := "# B"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: b.Page.ID, Version: b.Page.Version(), Title: b.Page.Title, Slug: b.Page.Slug, Content: &contentB, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage B failed: %v", err)
	}

	archive, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Archive", Slug: "archive", Kind: pageKind(),
	})
	a, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "A", Slug: "a", Kind: pageKind(),
	})
	contentA := "Links: [Old](/docs/b) and [New](/archive/docs/b)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage A failed: %v", err)
	}
	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	if err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID: "system", ID: docs.Page.ID, Version: docs.Page.Version(), ParentID: archive.Page.ID,
	}); err != nil {
		t.Fatalf("MovePage(docs -> archive) failed: %v", err)
	}

	out, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out.Count != 2 {
		t.Fatalf("unexpected outgoing after subtree move err=%v out=%#v", err, out)
	}
	state := map[string]struct {
		broken bool
		toID   string
	}{}
	for _, it := range out.Outgoings {
		state[it.ToPath] = struct {
			broken bool
			toID   string
		}{it.Broken, it.ToPageID}
	}
	if got := state["/docs/b"]; !got.broken || got.toID != "" {
		t.Fatalf("expected /docs/b broken after move, got %#v", state)
	}
	if got := state["/archive/docs/b"]; got.broken || got.toID != b.Page.ID {
		t.Fatalf("expected /archive/docs/b healed to %q, got %#v", b.Page.ID, state)
	}
}

func TestMovePageUseCase_ReindexesRelativeLinks(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	docs, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Docs", Slug: "docs", Kind: pageKind(),
	})
	docsShared, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "Shared", Slug: "shared", Kind: pageKind(),
	})
	contentDocsShared := "# Docs Shared"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: docsShared.Page.ID, Version: docsShared.Page.Version(), Title: docsShared.Page.Title, Slug: docsShared.Page.Slug, Content: &contentDocsShared, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage /docs/shared failed: %v", err)
	}

	a, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &docs.Page.ID, Title: "A", Slug: "a", Kind: pageKind(),
	})
	contentA := "Relative: [S](../shared)"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), Title: a.Page.Title, Slug: a.Page.Slug, Content: &contentA, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage /docs/a failed: %v", err)
	}

	guide, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Guide", Slug: "guide", Kind: pageKind(),
	})
	guideShared, _ := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", ParentID: &guide.Page.ID, Title: "Shared", Slug: "shared", Kind: pageKind(),
	})
	contentGuideShared := "# Guide Shared"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: guideShared.Page.ID, Version: guideShared.Page.Version(), Title: guideShared.Page.Title, Slug: guideShared.Page.Slug, Content: &contentGuideShared, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage /guide/shared failed: %v", err)
	}

	if err := deps.links.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages failed: %v", err)
	}

	out1, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out1.Count != 1 {
		t.Fatalf("unexpected outgoing before move err=%v out=%#v", err, out1)
	}
	if out1.Outgoings[0].ToPath != "/docs/shared" || out1.Outgoings[0].Broken || out1.Outgoings[0].ToPageID != docsShared.Page.ID {
		t.Fatalf("unexpected outgoing before move: %#v", out1.Outgoings[0])
	}

	if err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID: "system", ID: a.Page.ID, Version: a.Page.Version(), ParentID: guide.Page.ID,
	}); err != nil {
		t.Fatalf("MovePage(a -> guide) failed: %v", err)
	}

	out2, err := deps.links.GetOutgoingLinksForPage(a.Page.ID)
	if err != nil || out2.Count != 1 {
		t.Fatalf("unexpected outgoing after move err=%v out=%#v", err, out2)
	}
	if out2.Outgoings[0].ToPath != "/guide/shared" || out2.Outgoings[0].Broken || out2.Outgoings[0].ToPageID != guideShared.Page.ID {
		t.Fatalf("unexpected outgoing after move: %#v", out2.Outgoings[0])
	}
}

func TestAssetUseCases_RecordAssetRevisionForUser(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	uploadUC := wikiassets.NewUploadAssetUseCase(deps.tree, deps.assets, deps.revision, slog.Default())
	renameUC := wikiassets.NewRenameAssetUseCase(deps.tree, deps.assets, deps.revision, slog.Default())
	deleteUC := wikiassets.NewDeleteAssetUseCase(deps.tree, deps.assets, deps.revision, slog.Default())
	listUC := wikiassets.NewListAssetsUseCase(deps.tree, deps.assets)

	writeAsset := func(t *testing.T, pageID, name string, data []byte) {
		t.Helper()
		assetDir := filepath.Join(deps.assets.GetAssetsDir(), pageID)
		if err := os.MkdirAll(assetDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(assetDir) failed: %v", err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, name), data, 0o644); err != nil {
			t.Fatalf("WriteFile(asset) failed: %v", err)
		}
	}

	tests := []struct {
		name      string
		setup     func(t *testing.T, pageID string)
		operate   func(t *testing.T, pageID string)
		wantAsset string
	}{
		{
			name: "upload",
			operate: func(t *testing.T, pageID string) {
				t.Helper()
				file, err := os.CreateTemp(t.TempDir(), "asset-upload-*")
				if err != nil {
					t.Fatalf("CreateTemp failed: %v", err)
				}
				t.Cleanup(func() {
					if err := file.Close(); err != nil {
						t.Fatalf("Close(file) failed: %v", err)
					}
				})
				if _, err := file.WriteString("payload"); err != nil {
					t.Fatalf("WriteString(file) failed: %v", err)
				}
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					t.Fatalf("Seek(file) failed: %v", err)
				}
				if _, err := uploadUC.Execute(context.Background(), wikiassets.UploadAssetInput{
					UserID: "editor", PageID: pageID, File: file, Filename: "uploaded.txt", MaxBytes: 1024,
				}); err != nil {
					t.Fatalf("UploadAsset failed: %v", err)
				}
			},
			wantAsset: "uploaded.txt",
		},
		{
			name: "rename",
			setup: func(t *testing.T, pageID string) {
				t.Helper()
				writeAsset(t, pageID, "old.txt", []byte("payload"))
			},
			operate: func(t *testing.T, pageID string) {
				t.Helper()
				if _, err := renameUC.Execute(context.Background(), wikiassets.RenameAssetInput{
					UserID: "editor", PageID: pageID, OldFilename: "old.txt", NewFilename: "new.txt",
				}); err != nil {
					t.Fatalf("RenameAsset failed: %v", err)
				}
			},
			wantAsset: "new.txt",
		},
		{
			name: "delete",
			setup: func(t *testing.T, pageID string) {
				t.Helper()
				writeAsset(t, pageID, "delete.txt", []byte("payload"))
				if _, _, err := deps.revision.RecordAssetChange(pageID, "system", ""); err != nil {
					t.Fatalf("RecordAssetChange failed: %v", err)
				}
			},
			operate: func(t *testing.T, pageID string) {
				t.Helper()
				if err := deleteUC.Execute(context.Background(), wikiassets.DeleteAssetInput{
					UserID: "editor", PageID: pageID, Filename: "delete.txt",
				}); err != nil {
					t.Fatalf("DeleteAsset failed: %v", err)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := createUC.Execute(context.Background(), pages.CreatePageInput{
				UserID: "system", Title: "Asset Page " + tc.name, Slug: "asset-page-" + tc.name, Kind: pageKind(),
			})
			if err != nil {
				t.Fatalf("CreatePage failed: %v", err)
			}
			if tc.setup != nil {
				tc.setup(t, page.Page.ID)
			}
			tc.operate(t, page.Page.ID)

			latest, err := deps.revision.GetLatestRevision(page.Page.ID)
			if err != nil {
				t.Fatalf("GetLatestRevision failed: %v", err)
			}
			if latest == nil || latest.Type != revision.RevisionTypeAssetUpdate {
				t.Fatalf("latest revision = %#v", latest)
			}
			if latest.AuthorID != "editor" {
				t.Fatalf("latest author = %q, want %q", latest.AuthorID, "editor")
			}

			assetsOut, err := listUC.Execute(context.Background(), wikiassets.ListAssetsInput{PageID: page.Page.ID})
			if err != nil {
				t.Fatalf("ListAssets failed: %v", err)
			}
			if tc.wantAsset == "" {
				if len(assetsOut.Files) != 0 {
					t.Fatalf("assets = %#v, want empty", assetsOut.Files)
				}
				return
			}
			if len(assetsOut.Files) != 1 || !strings.HasSuffix(assetsOut.Files[0], "/"+tc.wantAsset) {
				t.Fatalf("assets = %#v, want suffix %q", assetsOut.Files, tc.wantAsset)
			}
		})
	}
}

func TestCheckIntegrityUseCase_Passthrough(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	checkUC := wikirevisions.NewCheckIntegrityUseCase(deps.revision)

	page, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Page", Slug: "page", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}
	content := "hello"
	pageOut, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: page.Page.ID, Version: page.Page.Version(), Title: page.Page.Title, Slug: page.Page.Slug, Content: &content, Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("UpdatePage failed: %v", err)
	}

	rev, err := deps.revision.GetLatestRevision(pageOut.Page.ID)
	if err != nil || rev == nil {
		t.Fatalf("GetLatestRevision failed: %#v %v", rev, err)
	}
	contentBlobPath := filepath.Join(deps.storageDir, ".leafwiki", "blobs", "content", "sha256", rev.ContentHash[:2], rev.ContentHash)
	if err := os.Remove(contentBlobPath); err != nil {
		t.Fatalf("Remove content blob failed: %v", err)
	}

	out, err := checkUC.Execute(context.Background(), wikirevisions.CheckIntegrityInput{PageID: pageOut.Page.ID})
	if err != nil {
		t.Fatalf("CheckRevisionIntegrity failed: %v", err)
	}
	if len(out.Issues) != 1 {
		t.Fatalf("expected 1 integrity issue, got %#v", out.Issues)
	}
	if out.Issues[0].Code != "missing_content_blob" {
		t.Fatalf("unexpected integrity issue: %#v", out.Issues[0])
	}
}

func TestEnsurePathUseCase_RecordsRevisionForEachCreatedSegment(t *testing.T) {
	deps := newTestDeps(t)
	uc := pages.NewEnsurePathUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	out, err := uc.Execute(context.Background(), pages.EnsurePathInput{
		UserID: "system", TargetPath: "/x/y", TargetTitle: "X Y", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("EnsurePath failed: %v", err)
	}

	latestY, err := deps.revision.GetLatestRevision(out.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(y) failed: %v", err)
	}
	if latestY == nil || latestY.Summary != "page created via ensure path" {
		t.Fatalf("unexpected y latest revision: %#v", latestY)
	}

	xPage, err := deps.tree.FindPageByRoutePath("x")
	if err != nil {
		t.Fatalf("FindByPath x failed: %v", err)
	}
	latestX, err := deps.revision.GetLatestRevision(xPage.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(x) failed: %v", err)
	}
	if latestX == nil || latestX.Summary != "page created via ensure path" {
		t.Fatalf("unexpected x latest revision: %#v", latestX)
	}
}

func TestMovePageUseCase_RecordsStructureRevision(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	moveUC := pages.NewMovePageUseCase(deps.tree, deps.revision, deps.links, slog.Default())

	dest, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Dest", Slug: "dest", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage(dest) failed: %v", err)
	}
	page, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Move Me", Slug: "move-me", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage(page) failed: %v", err)
	}

	if err := moveUC.Execute(context.Background(), pages.MovePageInput{
		UserID: "system", ID: page.Page.ID, Version: page.Page.Version(), ParentID: dest.Page.ID,
	}); err != nil {
		t.Fatalf("MovePage failed: %v", err)
	}

	latest, err := deps.revision.GetLatestRevision(page.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision failed: %v", err)
	}
	if latest == nil || latest.Type != revision.RevisionTypeStructureUpdate {
		t.Fatalf("latest revision = %#v", latest)
	}
	if latest.ParentID != dest.Page.ID {
		t.Fatalf("latest parent id = %q, want %q", latest.ParentID, dest.Page.ID)
	}
}

func TestUpdatePageUseCase_TitleOnlyCreatesStructureRevision(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	page, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Original", Slug: "original", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	content := "same content"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: page.Page.ID, Version: page.Page.Version(), Title: page.Page.Title, Slug: page.Page.Slug, Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage(initial content) failed: %v", err)
	}

	beforeLatest, err := deps.revision.GetLatestRevision(page.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(before rename) failed: %v", err)
	}
	if beforeLatest == nil {
		t.Fatal("expected initial content revision")
	}

	updatedPage, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: page.Page.ID, Version: page.Page.Version(), Title: "Renamed Title", Slug: page.Page.Slug, Content: nil, Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("UpdatePage(title only) failed: %v", err)
	}
	if updatedPage.Page.Title != "Renamed Title" {
		t.Fatalf("updated title = %q", updatedPage.Page.Title)
	}

	afterLatest, err := deps.revision.GetLatestRevision(page.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(after rename) failed: %v", err)
	}
	if afterLatest == nil || afterLatest.ID == beforeLatest.ID {
		t.Fatalf("expected new revision for title-only change, got before=%#v after=%#v", beforeLatest, afterLatest)
	}
	if afterLatest.Type != revision.RevisionTypeStructureUpdate {
		t.Fatalf("latest revision type = %q", afterLatest.Type)
	}

	revisions, err := deps.revision.ListRevisions(page.Page.ID)
	if err != nil {
		t.Fatalf("ListRevisions failed: %v", err)
	}
	if len(revisions) != 3 {
		t.Fatalf("revision count = %d, want 3", len(revisions))
	}
}

func TestUpdatePageUseCase_TitleOnlyWithUnchangedContentCreatesStructureRevision(t *testing.T) {
	deps := newTestDeps(t)
	createUC := pages.NewCreatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())
	updateUC := pages.NewUpdatePageUseCase(deps.tree, deps.slug, deps.revision, deps.links, slog.Default())

	page, err := createUC.Execute(context.Background(), pages.CreatePageInput{
		UserID: "system", Title: "Original", Slug: "original", Kind: pageKind(),
	})
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	content := "same content"
	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: page.Page.ID, Version: page.Page.Version(), Title: page.Page.Title, Slug: page.Page.Slug, Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage(initial content) failed: %v", err)
	}

	beforeLatest, err := deps.revision.GetLatestRevision(page.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(before rename) failed: %v", err)
	}
	if beforeLatest == nil {
		t.Fatal("expected initial content revision")
	}

	if _, err := updateUC.Execute(context.Background(), pages.UpdatePageInput{
		UserID: "system", ID: page.Page.ID, Version: page.Page.Version(), Title: "Renamed Title", Slug: page.Page.Slug, Content: &content, Kind: pageKind(),
	}); err != nil {
		t.Fatalf("UpdatePage(title only with unchanged content) failed: %v", err)
	}

	afterLatest, err := deps.revision.GetLatestRevision(page.Page.ID)
	if err != nil {
		t.Fatalf("GetLatestRevision(after rename) failed: %v", err)
	}
	if afterLatest == nil || afterLatest.ID == beforeLatest.ID {
		t.Fatalf("expected new revision for title-only change, got before=%#v after=%#v", beforeLatest, afterLatest)
	}
	if afterLatest.Type != revision.RevisionTypeStructureUpdate {
		t.Fatalf("latest revision type = %q", afterLatest.Type)
	}
	if afterLatest.Title != "Renamed Title" {
		t.Fatalf("latest revision title = %q", afterLatest.Title)
	}
}
