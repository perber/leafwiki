package wiki

import (
	"fmt"

	"github.com/perber/wiki/internal/core/tree"
)

type Wiki struct {
	tree *tree.TreeService
	slug *tree.SlugService
}

func NewWiki(storageDir string) (*Wiki, error) {
	treeService := tree.NewTreeService(storageDir)

	if err := treeService.LoadTree(); err != nil {
		return nil, err
	}

	return &Wiki{
		tree: treeService,
		slug: tree.NewSlugService(),
	}, nil
}

func (w *Wiki) CreatePage(parentID *string, title string) (*tree.Page, error) {
	var parent *tree.PageNode
	if parentID == nil {
		parent = w.tree.GetTree()
	} else {
		var err error
		parent, err = w.tree.FindPageByID(w.tree.GetTree().Children, *parentID)
		if err != nil {
			return nil, err
		}
	}

	slug := w.slug.GenerateUniqueSlug(parent, title)

	if err := w.tree.CreatePage(parentID, title, slug); err != nil {
		return nil, err
	}

	created, err := w.tree.FindPageByID(w.tree.GetTree().Children, slug)
	if err != nil {
		return nil, err
	}
	return &tree.Page{PageNode: created}, nil
}

func (w *Wiki) GetPage(id string) (*tree.Page, error) {
	return w.tree.GetPage(id)
}

func (w *Wiki) MovePage(id, parentID string) error {
	return w.tree.MovePage(id, parentID)
}

func (w *Wiki) DeletePage(id string, recursive bool) error {
	return w.tree.DeletePage(id, recursive)
}

func (w *Wiki) GetTree() *tree.PageNode {
	return w.tree.GetTree()
}

func (w *Wiki) UpdatePage(id, title, slug, content string) error {
	return w.tree.UpdatePage(id, title, slug, content)
}

func (w *Wiki) FindByPath(route string) (*tree.Page, error) {
	return w.tree.FindPageByRoutePath(w.tree.GetTree().Children, route)
}

func (w *Wiki) SuggestSlug(parentID string, title string) (string, error) {
	parent, err := w.tree.FindPageByID(w.tree.GetTree().Children, parentID)
	if err != nil {
		return "", fmt.Errorf("parent not found: %w", err)
	}

	return w.slug.GenerateUniqueSlug(parent, title), nil
}
