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

	wiki := &Wiki{
		tree: treeService,
		slug: tree.NewSlugService(),
	}

	// Ensure the welcome page exists
	if err := wiki.EnsureWelcomePage(); err != nil {
		return nil, err
	}

	return wiki, nil
}

func (w *Wiki) EnsureWelcomePage() error {
	_, err := w.tree.GetPage("root")
	if err == nil {
		return nil
	}

	if len(w.tree.GetTree().Children != 0) {
		return nil
	}

	_, err = w.CreatePage(nil, "Welcome to Leaf Wiki", "welcome-to-leaf-wiki")
	if err != nil {
		return err
	}

	return nil
}

func (w *Wiki) CreatePage(parentID *string, title string, slug string) (*tree.Page, error) {
	// Check if the parentID exists
	if parentID != nil && *parentID != "" {
		var err error
		_, err = w.tree.FindPageByID(w.tree.GetTree().Children, *parentID)
		if err != nil {
			return nil, err
		}
	}

	id, err := w.tree.CreatePage(parentID, title, slug)
	if err != nil {
		return nil, err
	}

	return w.tree.GetPage(*id)
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

func (w *Wiki) UpdatePage(id, title, slug, content string) (*tree.Page, error) {
	err := w.tree.UpdatePage(id, title, slug, content)
	if err != nil {
		return nil, err
	}

	return w.tree.GetPage(id)
}

func (w *Wiki) FindByPath(route string) (*tree.Page, error) {
	return w.tree.FindPageByRoutePath(w.tree.GetTree().Children, route)
}

func (w *Wiki) SuggestSlug(parentID string, title string) (string, error) {
	// if no parentID is set or it's the root page
	// We don't need to look for a page id
	if parentID == "" || parentID == "root" {
		return w.slug.GenerateUniqueSlug(w.tree.GetTree(), title), nil
	}

	parent, err := w.tree.FindPageByID(w.tree.GetTree().Children, parentID)
	if err != nil {
		return "", fmt.Errorf("parent not found: %w", err)
	}

	return w.slug.GenerateUniqueSlug(parent, title), nil
}
