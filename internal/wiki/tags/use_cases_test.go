package tags

import (
	"context"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	coretags "github.com/perber/wiki/internal/tags"
	"github.com/perber/wiki/internal/test_utils"
)

// ─── test helpers ─────────────────────────────────────────────────────────────

func setupUseCases(t *testing.T) (*GetPagesByTagsUseCase, *coretags.TagsService, *tree.TreeService) {
	t.Helper()

	dir := t.TempDir()
	ts := tree.NewTreeService(dir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	store, err := coretags.NewTagsStore(dir)
	if err != nil {
		t.Fatalf("NewTagsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })

	svc := coretags.NewTagsService(ts, store)
	uc := NewGetPagesByTagsUseCase(svc, ts, nil)
	return uc, svc, ts
}

func createAndIndexPage(t *testing.T, ts *tree.TreeService, svc *coretags.TagsService, title, slug string, tags []string, body string) string {
	t.Helper()

	kind := tree.NodeKindPage
	idPtr, err := ts.CreateNode("system", nil, title, slug, &kind)
	if err != nil {
		t.Fatalf("CreateNode %q: %v", slug, err)
	}

	fm := "---\ntags:\n"
	for _, tag := range tags {
		fm += "  - " + tag + "\n"
	}
	fm += "---\n\n" + body

	if err := ts.UpdateNode("system", *idPtr, title, slug, &fm, tree.VersionUnchecked, true); err != nil {
		t.Fatalf("UpdateNode %q: %v", slug, err)
	}

	raw, err := ts.ReadPageRaw(*idPtr)
	if err != nil {
		t.Fatalf("ReadPageRaw %q: %v", *idPtr, err)
	}
	if err := svc.IndexPageContent(*idPtr, raw); err != nil {
		t.Fatalf("IndexPageContent %q: %v", *idPtr, err)
	}

	return *idPtr
}

// ─── GetPagesByTagsUseCase ─────────────────────────────────────────────────────

func TestGetPagesByTagsUseCase_ReturnsMatchingPages(t *testing.T) {
	uc, svc, ts := setupUseCases(t)

	id1 := createAndIndexPage(t, ts, svc, "React Guide", "react-guide", []string{"react", "frontend"}, "React guide body.")
	createAndIndexPage(t, ts, svc, "Go Handbook", "go-handbook", []string{"go", "backend"}, "Go handbook body.")

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{"react"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(out.Pages))
	}
	if out.Pages[0].ID != id1 {
		t.Errorf("page ID = %q, want %q", out.Pages[0].ID, id1)
	}
}

func TestGetPagesByTagsUseCase_ExcerptComesFromDB_NoDiskRead(t *testing.T) {
	uc, svc, ts := setupUseCases(t)

	createAndIndexPage(t, ts, svc, "Excerpt Page", "excerpt-page", []string{"docs"}, "This is the excerpt content.")

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{"docs"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(out.Pages))
	}
	if out.Pages[0].Excerpt == "" {
		t.Error("excerpt should be non-empty (served from DB, not disk)")
	}
	if out.Pages[0].Excerpt != "This is the excerpt content." {
		t.Errorf("excerpt = %q", out.Pages[0].Excerpt)
	}
}

func TestGetPagesByTagsUseCase_ANDLogic(t *testing.T) {
	uc, svc, ts := setupUseCases(t)

	id1 := createAndIndexPage(t, ts, svc, "Both Tags", "both", []string{"react", "typescript"}, "body")
	createAndIndexPage(t, ts, svc, "Only React", "only-react", []string{"react"}, "body")

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{"react", "typescript"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 1 || out.Pages[0].ID != id1 {
		t.Errorf("expected only %q, got %v", id1, out.Pages)
	}
}

func TestGetPagesByTagsUseCase_EmptyTagsReturnsEmpty(t *testing.T) {
	uc, _, _ := setupUseCases(t)

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 0 {
		t.Errorf("expected no pages, got %d", len(out.Pages))
	}
}

func TestGetPagesByTagsUseCase_NormalizesInputTags(t *testing.T) {
	uc, svc, ts := setupUseCases(t)

	createAndIndexPage(t, ts, svc, "Go Page", "go-page", []string{"go"}, "body")

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{"GO", " go "}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 1 {
		t.Errorf("expected 1 page for normalized tag, got %d", len(out.Pages))
	}
}

func TestGetPagesByTagsUseCase_NoMatchReturnsEmpty(t *testing.T) {
	uc, svc, ts := setupUseCases(t)

	createAndIndexPage(t, ts, svc, "Go Page", "go-page", []string{"go"}, "body")

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{"rust"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 0 {
		t.Errorf("expected no pages, got %d", len(out.Pages))
	}
}

func TestGetPagesByTagsUseCase_PageTagsReturnedInResult(t *testing.T) {
	uc, svc, ts := setupUseCases(t)

	createAndIndexPage(t, ts, svc, "Multi Tag", "multi", []string{"go", "testing", "backend"}, "body")

	out, err := uc.Execute(context.Background(), GetPagesByTagsInput{Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(out.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(out.Pages))
	}
	if len(out.Pages[0].Tags) != 3 {
		t.Errorf("expected 3 tags on result, got %v", out.Pages[0].Tags)
	}
}
