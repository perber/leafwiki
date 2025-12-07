package backlinks

import (
	"github.com/perber/wiki/internal/core/tree"
)

type BacklinkService struct {
	storageDir  string
	treeService *tree.TreeService
	store       *BacklinksStore
}

func NewBacklinkService(storageDir string, treeService *tree.TreeService, store *BacklinksStore) *BacklinkService {
	return &BacklinkService{
		storageDir:  storageDir,
		treeService: treeService,
		store:       store,
	}
}

func (b *BacklinkService) IndexAllPages() error {
	root := b.treeService.GetTree()

	if root == nil {
		return nil
	}

	// Clear existing backlinks
	if err := b.store.Clear(); err != nil {
		return err
	}

	var indexPage func(node *tree.PageNode) error
	indexPage = func(node *tree.PageNode) error {
		if node.ID != "root" {
			page, err := b.treeService.GetPage(node.ID)
			if err != nil {
				return err
			}

			links := extractLinksFromMarkdown(page.Content)

			targets := resolveTargetLinks(b.treeService, page.CalculatePath(), links)

			if len(targets) > 0 {
				err = b.store.AddBacklinks(page.ID, page.Title, targets)
				if err != nil {
					return err
				}
			}
		}
		for _, child := range node.Children {
			if err := indexPage(child); err != nil {
				return err
			}
		}
		return nil
	}

	if err := indexPage(root); err != nil {
		return err
	}

	return nil
}

func (b *BacklinkService) ClearBacklinks() error {
	return b.store.Clear()
}

func (b *BacklinkService) GetBacklinksForPage(pageID string) (*BacklinkResult, error) {
	backlinks, err := b.store.GetBacklinksForPage(pageID)
	return toBacklinkResult(b.treeService, backlinks), err
}

func (b *BacklinkService) UpdateBacklinksForPage(page *tree.Page, content string) error {
	links := extractLinksFromMarkdown(content)

	targets := resolveTargetLinks(b.treeService, page.CalculatePath(), links)

	err := b.store.AddBacklinks(page.ID, page.Title, targets)
	if err != nil {
		return err
	}

	return nil
}

func (b *BacklinkService) RemoveBacklinksForPage(pageID string) error {
	return b.store.RemoveBacklinks(pageID)
}

func (b *BacklinkService) Close() error {
	if b.store == nil {
		return nil
	}
	return b.store.Close()
}
