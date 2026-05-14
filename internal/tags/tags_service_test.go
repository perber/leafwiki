package tags

import (
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

// ─── ExtractTagsFromContent ──────────────────────────────────────────────────

func TestExtractTagsFromContent_BlockListSyntax(t *testing.T) {
	content := "---\ntags:\n  - react\n  - typescript\n---\n\n# Page"
	got := ExtractTagsFromContent(content)
	assertStringSliceEqual(t, got, []string{"react", "typescript"})
}

func TestExtractTagsFromContent_InlineListSyntax(t *testing.T) {
	content := "---\ntags: [react, typescript]\n---\n\n# Page"
	got := ExtractTagsFromContent(content)
	assertStringSliceEqual(t, got, []string{"react", "typescript"})
}

func TestExtractTagsFromContent_NormalizesToLowercase(t *testing.T) {
	content := "---\ntags:\n  - React\n  - TypeScript\n  - GO\n---\n"
	got := ExtractTagsFromContent(content)
	assertStringSliceEqual(t, got, []string{"react", "typescript", "go"})
}

func TestExtractTagsFromContent_DeduplicatesCaseInsensitive(t *testing.T) {
	content := "---\ntags:\n  - react\n  - React\n  - REACT\n---\n"
	got := ExtractTagsFromContent(content)
	if len(got) != 1 {
		t.Errorf("expected 1 deduplicated tag, got %d: %v", len(got), got)
	}
	if len(got) > 0 && got[0] != "react" {
		t.Errorf("got[0] = %q, want 'react'", got[0])
	}
}

func TestExtractTagsFromContent_TrimsWhitespace(t *testing.T) {
	content := "---\ntags:\n  - \" react \"\n  - \" go \"\n---\n"
	got := ExtractTagsFromContent(content)
	for _, tag := range got {
		if tag != trimmed(tag) {
			t.Errorf("tag %q has surrounding whitespace", tag)
		}
	}
}

func TestExtractTagsFromContent_NoFrontmatterReturnsNil(t *testing.T) {
	content := "# Page\n\nJust content, no frontmatter."
	got := ExtractTagsFromContent(content)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestExtractTagsFromContent_EmptyContentReturnsNil(t *testing.T) {
	got := ExtractTagsFromContent("")
	if got != nil {
		t.Errorf("expected nil for empty content, got %v", got)
	}
}

func TestExtractTagsFromContent_FrontmatterWithoutTagsFieldReturnsNil(t *testing.T) {
	content := "---\ntitle: My Page\nauthor: Alice\n---\n\n# Content"
	got := ExtractTagsFromContent(content)
	if got != nil {
		t.Errorf("expected nil when tags key absent, got %v", got)
	}
}

func TestExtractTagsFromContent_EmptyTagsListReturnsNil(t *testing.T) {
	content := "---\ntags: []\n---\n\n# Content"
	got := ExtractTagsFromContent(content)
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestExtractTagsFromContent_SkipsEmptyTagEntries(t *testing.T) {
	content := "---\ntags:\n  - react\n  - \"\"\n  - typescript\n---\n"
	got := ExtractTagsFromContent(content)
	for _, tag := range got {
		if tag == "" {
			t.Errorf("empty tag should be filtered out")
		}
	}
	if len(got) != 2 {
		t.Errorf("expected 2 non-empty tags, got %d: %v", len(got), got)
	}
}

func TestExtractTagsFromContent_TagsKeyIsCaseInsensitive(t *testing.T) {
	// Frontmatter keys like "Tags" or "TAGS" should still be found.
	content := "---\nTags:\n  - react\n---\n"
	got := ExtractTagsFromContent(content)
	if len(got) != 1 || got[0] != "react" {
		t.Errorf("expected [react] for upper-case Tags key, got %v", got)
	}
}

// ─── TagsService integration (with real tree + store) ────────────────────────

func setupTagsService(t *testing.T) (*TagsService, *tree.TreeService) {
	t.Helper()

	dir := t.TempDir()
	ts := tree.NewTreeService(dir)
	if err := ts.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	store, err := NewTagsStore(dir)
	if err != nil {
		t.Fatalf("NewTagsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(store.Close, t) })

	return NewTagsService(ts, store), ts
}

func pageKind() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}

func createPageWithTags(t *testing.T, ts *tree.TreeService, title, slug string, tags []string) string {
	t.Helper()

	idPtr, err := ts.CreateNode("system", nil, title, slug, pageKind())
	if err != nil {
		t.Fatalf("CreateNode %q: %v", slug, err)
	}

	fm := "---\ntags:\n"
	for _, tag := range tags {
		fm += "  - " + tag + "\n"
	}
	fm += "---\n\n# " + title

	if err := ts.UpdateNode("system", *idPtr, title, slug, &fm, tree.VersionUnchecked, true); err != nil {
		t.Fatalf("UpdateNode %q: %v", slug, err)
	}

	return *idPtr
}

func TestTagsService_IndexAllPages_BuildsIndex(t *testing.T) {
	svc, ts := setupTagsService(t)

	id1 := createPageWithTags(t, ts, "Page React", "react-page", []string{"react", "typescript"})
	id2 := createPageWithTags(t, ts, "Page Go", "go-page", []string{"go"})

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	pageIDs, err := svc.GetPageIDsByTags([]string{"react"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if len(pageIDs) != 1 || pageIDs[0] != id1 {
		t.Errorf("expected [%s], got %v", id1, pageIDs)
	}

	pageIDs2, err := svc.GetPageIDsByTags([]string{"go"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if len(pageIDs2) != 1 || pageIDs2[0] != id2 {
		t.Errorf("expected [%s], got %v", id2, pageIDs2)
	}
}

func TestTagsService_IndexAllPages_IsIdempotent(t *testing.T) {
	svc, ts := setupTagsService(t)
	createPageWithTags(t, ts, "Page A", "page-a", []string{"go"})

	for i := 0; i < 3; i++ {
		if err := svc.IndexAllPages(); err != nil {
			t.Fatalf("IndexAllPages (run %d): %v", i, err)
		}
	}

	allTags, err := svc.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(allTags) != 1 || allTags[0].Tag != "go" || allTags[0].Count != 1 {
		t.Errorf("expected [{go 1}], got %v", allTags)
	}
}

func TestTagsService_IndexAllPages_PagesWithoutTagsAreSkipped(t *testing.T) {
	svc, ts := setupTagsService(t)

	idPtr, err := ts.CreateNode("system", nil, "No Tags Page", "no-tags", pageKind())
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	content := "# No Tags Page\n\nNo frontmatter."
	if err := ts.UpdateNode("system", *idPtr, "No Tags Page", "no-tags", &content, tree.VersionUnchecked, false); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	allTags, err := svc.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(allTags) != 0 {
		t.Errorf("expected no tags indexed for page without frontmatter, got %v", allTags)
	}
}

func TestTagsService_IndexAllPages_NormalizesTagsToLowercase(t *testing.T) {
	svc, ts := setupTagsService(t)
	createPageWithTags(t, ts, "Mixed Case", "mixed", []string{"React", "TypeScript"})

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	allTags, err := svc.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}

	for _, tc := range allTags {
		if tc.Tag != lower(tc.Tag) {
			t.Errorf("tag %q is not lowercase", tc.Tag)
		}
	}
}

func TestTagsService_IndexAllPages_ReadsTagsFromRawFrontmatter(t *testing.T) {
	svc, ts := setupTagsService(t)
	pageID := createPageWithTags(t, ts, "Tagged Page", "tagged-page", []string{"react"})

	page, err := ts.GetPage(pageID)
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if got := ExtractTagsFromContent(page.Content); got != nil {
		t.Fatalf("expected parsed page content to exclude frontmatter tags, got %v", got)
	}

	if err := svc.IndexAllPages(); err != nil {
		t.Fatalf("IndexAllPages: %v", err)
	}

	pageIDs, err := svc.GetPageIDsByTags([]string{"react"})
	if err != nil {
		t.Fatalf("GetPageIDsByTags: %v", err)
	}
	if len(pageIDs) != 1 || pageIDs[0] != pageID {
		t.Fatalf("expected [%s], got %v", pageID, pageIDs)
	}
}

func TestTagsService_SetAndDeleteTagsForPage(t *testing.T) {
	svc, _ := setupTagsService(t)

	if err := svc.SetTagsForPage("page-x", []string{"go", "test"}); err != nil {
		t.Fatalf("SetTagsForPage: %v", err)
	}
	if err := svc.DeleteTagsForPage("page-x"); err != nil {
		t.Fatalf("DeleteTagsForPage: %v", err)
	}

	allTags, err := svc.GetAllTags("", 50)
	if err != nil {
		t.Fatalf("GetAllTags: %v", err)
	}
	if len(allTags) != 0 {
		t.Errorf("expected no tags after delete, got %v", allTags)
	}
}

func TestTagsService_GetTagsForPages_ReturnsCorrectTags(t *testing.T) {
	svc, _ := setupTagsService(t)

	_ = svc.SetTagsForPage("p1", []string{"go", "testing"})
	_ = svc.SetTagsForPage("p2", []string{"typescript"})

	got, err := svc.GetTagsForPages([]string{"p1", "p2"})
	if err != nil {
		t.Fatalf("GetTagsForPages: %v", err)
	}

	assertStringSliceEqual(t, got["p1"], []string{"go", "testing"})
	assertStringSliceEqual(t, got["p2"], []string{"typescript"})
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func trimmed(s string) string {
	result := s
	for len(result) > 0 && (result[0] == ' ' || result[0] == '\t') {
		result = result[1:]
	}
	for len(result) > 0 && (result[len(result)-1] == ' ' || result[len(result)-1] == '\t') {
		result = result[:len(result)-1]
	}
	return result
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
