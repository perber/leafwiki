package search

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	coretags "github.com/perber/wiki/internal/tags"
)

func setupExcerptPages(tb testing.TB, count int) (*SearchUseCase, []string) {
	tb.Helper()
	dir := tb.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "root"), 0755); err != nil {
		tb.Fatal(err)
	}
	store, err := coretags.NewTagsStore(dir)
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		if err := store.Close(); err != nil {
			tb.Error(err)
		}
	})
	tags := coretags.NewTagsService(store)
	ids := make([]string, count)
	for i := range ids {
		ids[i] = fmt.Sprintf("p-%04d", i)
		raw := fmt.Sprintf("---\nleafwiki_id: %s\nleafwiki_title: Page %04d\nleafwiki_created_at: 2026-01-01T00:00:00Z\nleafwiki_updated_at: 2026-01-01T00:00:00Z\nleafwiki_creator_id: system\nleafwiki_last_author_id: system\ntags: [docs]\n---\n\nUnique excerpt %04d", ids[i], i, i)
		if err := os.WriteFile(filepath.Join(dir, "root", fmt.Sprintf("page-%04d.md", i)), []byte(raw), 0644); err != nil {
			tb.Fatal(err)
		}
		if err := tags.IndexPageContent(ids[i], raw); err != nil {
			tb.Fatal(err)
		}
	}
	svc := tree.NewTreeService(dir)
	if err := svc.LoadTree(); err != nil {
		tb.Fatal(err)
	}
	return NewSearchUseCase(nil, tags, svc), ids
}
func TestSearchUseCase_SearchByTags_PaginationPreservesExcerptsAndFacets(t *testing.T) {
	uc, ids := setupExcerptPages(t, 5)
	for _, tc := range []struct{ offset, limit, want int }{{1, 2, 2}, {10, 2, 0}, {-1, 0, 5}} {
		out, err := uc.searchByTags(ids, tc.offset, tc.limit)
		if err != nil {
			t.Fatal(err)
		}
		if out.Result.Count != 5 || len(out.Result.Items) != tc.want {
			t.Fatalf("got %#v", out.Result)
		}
		if len(out.Result.TagFacets) != 1 || out.Result.TagFacets[0].Count != 5 {
			t.Fatalf("facets %#v", out.Result.TagFacets)
		}
		for _, item := range out.Result.Items {
			suffix := strings.TrimPrefix(item.PageID, "p-")
			if !strings.Contains(item.Excerpt, "Unique excerpt "+suffix) {
				t.Fatalf("wrong excerpt %+v", item)
			}
		}
	}
}
func BenchmarkSearchByTagsPagedExcerpts(b *testing.B) {
	for _, count := range []int{100, 1000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			uc, ids := setupExcerptPages(b, count)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := uc.searchByTags(ids, 0, 20); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
