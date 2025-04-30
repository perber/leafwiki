package api

import "github.com/perber/wiki/internal/core/tree"

type Page struct {
	*tree.PageNode
	Content string `json:"content"`
	Path    string `json:"path"`
}
