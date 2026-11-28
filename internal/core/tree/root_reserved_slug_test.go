package tree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestTreeService_CreateNode_RejectsRootReservedSlug(t *testing.T) {
	svc, _ := newLoadedService(t)

	for _, slug := range []string{"static", "Static"} {
		if _, err := svc.CreateNode("system", nil, "Static", slug, ptrKind(NodeKindPage)); !errors.Is(err, ErrSlugReserved) {
			t.Fatalf("CreateNode(%q) at root: expected ErrSlugReserved, got %v", slug, err)
		}
	}
}

func TestTreeService_CreateNode_AllowsRootReservedSlugBelowRoot(t *testing.T) {
	svc, _ := newLoadedService(t)

	docsID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode docs: %v", err)
	}
	if _, err := svc.CreateNode("system", docsID, "Static", "static", ptrKind(NodeKindPage)); err != nil {
		t.Fatalf("CreateNode static under docs: %v", err)
	}
}

func TestTreeService_UpdateNode_RejectsRenameToRootReservedSlug(t *testing.T) {
	svc, _ := newLoadedService(t)

	id, err := svc.CreateNode("system", nil, "Files", "files", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	err = svc.UpdateNode(UpdateNodeInput{UserID: "system", ID: *id, Title: "Files", Slug: "static", ExpectedVersion: VersionUnchecked})
	if !errors.Is(err, ErrSlugReserved) {
		t.Fatalf("expected ErrSlugReserved, got %v", err)
	}
}

func TestTreeService_MoveNode_RejectsRootReservedSlugToRoot(t *testing.T) {
	svc, _ := newLoadedService(t)

	docsID, err := svc.CreateNode("system", nil, "Docs", "docs", ptrKind(NodeKindSection))
	if err != nil {
		t.Fatalf("CreateNode docs: %v", err)
	}
	staticID, err := svc.CreateNode("system", docsID, "Static", "static", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode static: %v", err)
	}

	if err := svc.MoveNode("system", *staticID, "root", VersionUnchecked); !errors.Is(err, ErrSlugReserved) {
		t.Fatalf("expected ErrSlugReserved, got %v", err)
	}
}

func TestTreeService_EnsurePagePath_RejectsRootReservedFirstSegment(t *testing.T) {
	svc, _ := newLoadedService(t)

	if _, err := svc.EnsurePagePath("system", "static/child", "Child", ptrKind(NodeKindPage)); !errors.Is(err, ErrSlugReserved) {
		t.Fatalf("expected ErrSlugReserved, got %v", err)
	}
	if _, err := svc.EnsurePagePath("system", "docs/static", "Static", ptrKind(NodeKindPage)); err != nil {
		t.Fatalf("EnsurePagePath docs/static: %v", err)
	}
}

// Markdown files on disk are the source of truth: a top-level page named
// static that predates the reservation must still load and stay editable.
func TestTreeService_ExistingRootStaticPage_StaysEditableAndSortable(t *testing.T) {
	svc, tmpDir := newLoadedService(t)

	if err := os.MkdirAll(filepath.Join(tmpDir, "root"), 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "root", "static.md"), []byte("# Static\n"), 0o644); err != nil {
		t.Fatalf("write static.md: %v", err)
	}
	if err := svc.ReconstructTreeFromFS(); err != nil {
		t.Fatalf("ReconstructTreeFromFS: %v", err)
	}

	page, err := svc.FindPageByRoutePath("static")
	if err != nil {
		t.Fatalf("existing static page not loaded: %v", err)
	}

	content := "# Static\n\nedited\n"
	if err := svc.UpdateNode(UpdateNodeInput{UserID: "system", ID: page.ID, Title: "Static files", Slug: "static", Content: &content, ExpectedVersion: VersionUnchecked}); err != nil {
		t.Fatalf("UpdateNode on existing static page: %v", err)
	}

	otherID, err := svc.CreateNode("system", nil, "Other", "other", ptrKind(NodeKindPage))
	if err != nil {
		t.Fatalf("CreateNode other: %v", err)
	}
	if err := svc.SortPages("root", []string{*otherID, page.ID}); err != nil {
		t.Fatalf("reorder existing static page within root: %v", err)
	}
	if err := svc.UpdateNode(UpdateNodeInput{UserID: "system", ID: page.ID, Title: "Static files", Slug: "Static", ExpectedVersion: VersionUnchecked}); err != nil {
		t.Fatalf("case-only rename of existing static page: %v", err)
	}
}

func TestTreeService_LookupPagePath_RootReservedSlugIsNotCreatable(t *testing.T) {
	svc, _ := newLoadedService(t)

	lookup, err := svc.LookupPagePath("static/child")
	if err != nil {
		t.Fatalf("LookupPagePath: %v", err)
	}
	if lookup.CanCreate {
		t.Fatal("expected static/child at the root not to be creatable")
	}

	lookup, err = svc.LookupPagePath("docs/static")
	if err != nil {
		t.Fatalf("LookupPagePath: %v", err)
	}
	if !lookup.CanCreate {
		t.Fatal("expected docs/static to be creatable")
	}
}
