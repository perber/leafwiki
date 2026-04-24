package tree

type Page struct {
	*PageNode
	Content string `json:"content"`
}

type PermalinkTarget struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Path string `json:"path"`
}
