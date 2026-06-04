package search

import (
	"context"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	coresearch "github.com/perber/wiki/internal/search"
	coretags "github.com/perber/wiki/internal/tags"
	"github.com/perber/wiki/internal/test_utils"
)

func setupSearchUseCaseForTags(t *testing.T) (*SearchUseCase, *coretags.TagsService, *tree.TreeService) {
	t.Helper()

	dir := t.TempDir()

	treeSvc := tree.NewTreeService(dir)
	if err := treeSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	index, err := coresearch.NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(index.Close, t) })

	tagsStore, err := coretags.NewTagsStore(dir)
	if err != nil {
		t.Fatalf("NewTagsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(tagsStore.Close, t) })

	tagsSvc := coretags.NewTagsService(tagsStore)
	return NewSearchUseCase(index, tagsSvc, treeSvc), tagsSvc, treeSvc
}

func pageKind() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}

func createTaggedPage(t *testing.T, treeSvc *tree.TreeService, title, slug string, tags []string) string {
	t.Helper()

	idPtr, err := treeSvc.CreateNode("system", nil, title, slug, pageKind())
	if err != nil {
		t.Fatalf("CreateNode %q: %v", slug, err)
	}

	content := "---\ntags:\n"
	for _, tag := range tags {
		content += "  - " + tag + "\n"
	}
	content += "---\n\n# Page body"

	if err := treeSvc.UpdateNode("system", *idPtr, title, slug, &content, tree.VersionUnchecked, true); err != nil {
		t.Fatalf("UpdateNode %q: %v", slug, err)
	}

	return *idPtr
}

func TestSearchUseCase_Execute_TagOnlySearchEscapesTitle(t *testing.T) {
	uc, tagsSvc, treeSvc := setupSearchUseCaseForTags(t)

	pageID := createTaggedPage(t, treeSvc, `<img src=x onerror="alert(1)">`, "tagged-title", []string{"docs"})

	page, err := treeSvc.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if err := tagsSvc.IndexPageContent(page.ID, page.RawContent); err != nil {
		t.Fatalf("IndexPageContent: %v", err)
	}

	out, err := uc.Execute(context.Background(), SearchInput{
		Tags:  []string{"docs"},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if out.Result == nil || len(out.Result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %#v", out.Result)
	}

	got := out.Result.Items[0].Title
	if got != "&lt;img src=x onerror=&#34;alert(1)&#34;&gt;" {
		t.Fatalf("escaped title = %q", got)
	}
}
