package search

import (
	"context"
	"strings"
	"testing"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
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

	if err := treeSvc.UpdateNode(tree.UpdateNodeInput{UserID: "system", ID: *idPtr, Title: title, Slug: slug, Content: &content, ExpectedVersion: tree.VersionUnchecked, PreserveFrontmatter: true}); err != nil {
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

// TestSearchUseCase_Execute_RejectsOverLongQueries pins the query caps: at
// most maxQueryRunes runes after trimming and maxQueryTerms search terms.
// The use case runs without an index, so a query that passes the caps gets
// search_unavailable and one that is rejected proves the check runs before
// any SQL.
func TestSearchUseCase_Execute_RejectsOverLongQueries(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		tags     []string
		wantCode string
	}{
		{name: "many short words", query: strings.Repeat("pi ", 50), wantCode: ErrCodeSearchQueryTooLong},
		{name: "12 terms pass", query: strings.TrimSpace(strings.Repeat("pi ", 12)), wantCode: ErrCodeSearchUnavailable},
		{name: "13 terms fail", query: strings.TrimSpace(strings.Repeat("pi ", 13)), wantCode: ErrCodeSearchQueryTooLong},
		{name: "terms split by punctuation count", query: strings.Repeat("a,", 13), wantCode: ErrCodeSearchQueryTooLong},
		{name: "256 runes pass", query: strings.Repeat("a", 256), wantCode: ErrCodeSearchUnavailable},
		{name: "257 runes fail", query: strings.Repeat("a", 257), wantCode: ErrCodeSearchQueryTooLong},
		{name: "length counts runes, not bytes", query: strings.Repeat("ü", 256), wantCode: ErrCodeSearchUnavailable},
		{name: "surrounding spaces don't count", query: "   " + strings.Repeat("a", 256) + "   ", wantCode: ErrCodeSearchUnavailable},
		{name: "tag-only search is unaffected", tags: []string{"docs"}, wantCode: ErrCodeSearchUnavailable},
		{name: "whitespace-only query is not too long", query: strings.Repeat(" ", 300), wantCode: ErrCodeSearchUnavailable},
		{name: "257 multi-byte runes fail", query: strings.Repeat("ü", 257), wantCode: ErrCodeSearchQueryTooLong},
		{name: "256 emoji pass the rune cap", query: strings.Repeat("🙂", 256), wantCode: ErrCodeSearchUnavailable},
		{name: "257 emoji fail the rune cap", query: strings.Repeat("🙂", 257), wantCode: ErrCodeSearchQueryTooLong},
		{name: "12 terms across field kinds pass", query: `(a OR b) NOT title:c,d "e f g" h* i,j k l`, wantCode: ErrCodeSearchUnavailable},
		{name: "13 terms across field kinds fail", query: `(a OR b) NOT title:c,d "e f g" h* i,j k l m`, wantCode: ErrCodeSearchQueryTooLong},
		{name: "13 column filter words fail", query: "title:a,b,c,d,e,f,g,h,i,j,k,l,m", wantCode: ErrCodeSearchQueryTooLong},
		{name: "13 words in one quoted phrase fail", query: `"a b c d e f g h i j k l m"`, wantCode: ErrCodeSearchQueryTooLong},
		{name: "boolean keywords don't count", query: "a OR b OR c OR d OR e OR f OR g OR h OR i OR j OR k OR l", wantCode: ErrCodeSearchUnavailable},
		{name: "over-long query with tags is still rejected", query: strings.Repeat("pi ", 13), tags: []string{"docs"}, wantCode: ErrCodeSearchQueryTooLong},
	}
	uc := NewSearchUseCase(nil, nil, nil)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), SearchInput{Query: tc.query, Tags: tc.tags, Limit: 10})
			loc, ok := sharederrors.AsLocalizedError(err)
			if !ok {
				t.Fatalf("Execute error = %v, want a LocalizedError with code %s", err, tc.wantCode)
			}
			if loc.Code != tc.wantCode {
				t.Errorf("Execute error code = %s, want %s", loc.Code, tc.wantCode)
			}
		})
	}
}
