package importer

import "github.com/perber/wiki/internal/core/tree"

type ImporterWiki interface {
	TreeHash() string
	LookupPagePath(path string) (*tree.PathLookup, error)
	EnsurePath(userID string, targetPath string, title string, kind *tree.NodeKind) (*tree.Page, error)
	UpdatePage(userID string, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error)
}
