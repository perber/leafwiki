package search

type SearchResult struct {
	Limit     int                `json:"limit"`
	Offset    int                `json:"offset"`
	Count     int                `json:"count"`
	Items     []SearchResultItem `json:"items"`
	TagFacets []SearchTagFacet   `json:"tag_facets"`
}

type SearchResultItem struct {
	PageID  string   `json:"page_id"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Kind    string   `json:"kind"`
	Rank    float64  `json:"rank"`
	Excerpt string   `json:"excerpt"`
	Tags    []string `json:"tags,omitempty"`
}

type SearchTagFacet struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}
