package importer

import (
	"mime/multipart"

	"github.com/perber/wiki/internal/core/tree"
)

type ImporterWiki interface {
	TreeHash() string
	LookupPagePath(path string) (*tree.PathLookup, error)
	EnsurePath(userID string, targetPath string, title string, kind *tree.NodeKind) (*tree.Page, error)
	UpdatePage(userID string, id, title, slug string, content *string, kind *tree.NodeKind) (*tree.Page, error)
	UploadAsset(pageID string, file multipart.File, filename string, maxBytes int64) (string, error)
}
