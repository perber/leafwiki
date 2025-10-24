package tree

type Page struct {
	*PageNode
	Content string `json:"content"`
}
