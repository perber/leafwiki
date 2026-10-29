package dto

import (
	"time"

	coresearch "github.com/perber/wiki/internal/search"
)

// SearchResult is returned by the search endpoint.
type SearchResult struct {
	Limit     int                `json:"limit"`
	Offset    int                `json:"offset"`
	Count     int                `json:"count"`
	Items     []SearchResultItem `json:"items"`
	TagFacets []SearchTagFacet   `json:"tag_facets"`
}

// SearchResultItem is one hit of a search.
type SearchResultItem struct {
	PageID  string   `json:"page_id"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Kind    string   `json:"kind"`
	Rank    float64  `json:"rank"`
	Excerpt string   `json:"excerpt"`
	Tags    []string `json:"tags,omitempty"`
}

// SearchTagFacet counts the matches that carry one tag.
type SearchTagFacet struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// ToSearchResult converts a search result; nil stays nil, and Items and
// TagFacets become empty slices so the response is [] rather than null.
func ToSearchResult(in *coresearch.SearchResult) *SearchResult {
	if in == nil {
		return nil
	}

	items := make([]SearchResultItem, len(in.Items))
	for i, item := range in.Items {
		items[i] = SearchResultItem{
			PageID:  item.PageID,
			Title:   item.Title,
			Path:    item.Path,
			Kind:    item.Kind,
			Rank:    item.Rank,
			Excerpt: item.Excerpt,
			Tags:    item.Tags,
		}
	}

	facets := make([]SearchTagFacet, len(in.TagFacets))
	for i, facet := range in.TagFacets {
		facets[i] = SearchTagFacet{Tag: facet.Tag, Count: facet.Count}
	}

	return &SearchResult{
		Limit:     in.Limit,
		Offset:    in.Offset,
		Count:     in.Count,
		Items:     items,
		TagFacets: facets,
	}
}

// IndexingStatus is returned by the search status endpoint.
type IndexingStatus struct {
	Active     bool      `json:"active"`
	Indexed    int       `json:"indexed"`
	Failed     int       `json:"failed"`
	FinishedAt time.Time `json:"finished_at"`
}

// ToIndexingStatus converts an indexing status; nil stays nil. The core
// struct holds a mutex, so the caller must pass the lock-free copy from
// IndexingStatus.Snapshot, never the live status the indexer writes to.
func ToIndexingStatus(in *coresearch.IndexingStatus) *IndexingStatus {
	if in == nil {
		return nil
	}
	return &IndexingStatus{
		Active:     in.Active,
		Indexed:    in.Indexed,
		Failed:     in.Failed,
		FinishedAt: in.FinishedAt,
	}
}
