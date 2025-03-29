package api

type Node struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Slug     string  `json:"slug"`
	Path     string  `json:"path"`
	Children []*Node `json:"children"`
}
