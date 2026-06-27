package tree

type Page struct {
	*PageNode
	Content    string                   `json:"content"`
	RawContent string                   `json:"-"`
	Tags       []string                 `json:"-"`
	Properties map[string]MetadataValue `json:"-"`
}

type PermalinkTarget struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Path string `json:"path"`
}

// Version returns a stable optimistic-lock token for the current page state.
func (p *Page) Version() string {
	if p == nil || p.PageNode == nil {
		return ""
	}
	return p.PageNode.Version()
}
