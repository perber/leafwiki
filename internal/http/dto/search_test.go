package dto

import (
	"testing"
	"time"

	coresearch "github.com/perber/wiki/internal/search"
)

// GET /api/search used to serialize the core type directly; the dto mapping
// must produce the same bytes.
func TestToSearchResult_Marshal_MatchesCoreType(t *testing.T) {
	in := &coresearch.SearchResult{
		Limit:  20,
		Offset: 0,
		Count:  2,
		Items: []coresearch.SearchResultItem{
			{PageID: "p1", Title: "One", Path: "one", Kind: "page", Rank: 1.5, Excerpt: "<b>x</b>", Tags: []string{"go", "web"}},
			{PageID: "p2", Title: "Two", Path: "two", Kind: "section", Rank: 0.25, Excerpt: ""},
		},
		TagFacets: []coresearch.SearchTagFacet{{Tag: "go", Count: 1}},
	}

	got := marshalString(t, ToSearchResult(in))
	assertMarshalsTo(t, in, got)
}

func TestToSearchResult_EmptyListsAreArrays(t *testing.T) {
	tests := []struct {
		name string
		in   *coresearch.SearchResult
	}{
		{"nil slices", &coresearch.SearchResult{Limit: 20}},
		{"empty slices", &coresearch.SearchResult{Limit: 20, Items: []coresearch.SearchResultItem{}, TagFacets: []coresearch.SearchTagFacet{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMarshalsTo(t, ToSearchResult(tt.in), `{"limit":20,"offset":0,"count":0,"items":[],"tag_facets":[]}`)
		})
	}
}

func TestToSearchResult_Nil_MarshalsAsNull(t *testing.T) {
	assertMarshalsTo(t, ToSearchResult(nil), `null`)
}

func TestToIndexingStatus_Marshal_MatchesCoreType(t *testing.T) {
	finished := time.Date(2026, 10, 29, 12, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   *coresearch.IndexingStatus
	}{
		{"zero finished_at", &coresearch.IndexingStatus{Active: true, Indexed: 3, Failed: 1}},
		{"finished", &coresearch.IndexingStatus{Indexed: 10, FinishedAt: finished}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertMarshalsTo(t, ToIndexingStatus(tt.in), marshalString(t, tt.in))
		})
	}
}

func TestToIndexingStatus_Nil_MarshalsAsNull(t *testing.T) {
	assertMarshalsTo(t, ToIndexingStatus(nil), `null`)
}
