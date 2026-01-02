package api

type Page struct {
	*Node
	Content string `json:"content"`
	Path    string `json:"path"`
}
