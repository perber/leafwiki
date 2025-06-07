package search

type SearchResult struct {
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
	Count  int                `json:"count"`
	Items  []SearchResultItem `json:"items"`
}

type SearchResultItem struct {
	PageID  string  `json:"page_id"`
	Title   string  `json:"title"`
	Path    string  `json:"path"`
	Rank    float64 `json:"rank"`
	Excerpt string  `json:"excerpt"`
}
