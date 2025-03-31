package api

type Node struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Slug     string  `json:"slug"`
	Path     string  `json:"path"`
	Position int     `json:"position"`
	Children []*Node `json:"children"`
}
