package links

import (
	"github.com/perber/wiki/internal/core/tree"
)

type LinkService struct {
	storageDir  string
	treeService *tree.TreeService
	store       *LinksStore
}

func NewLinkService(storageDir string, treeService *tree.TreeService, store *LinksStore) *LinkService {
	return &LinkService{
		storageDir:  storageDir,
		treeService: treeService,
		store:       store,
	}
}

func (b *LinkService) IndexAllPages() error {
	root := b.treeService.GetTree()

	if root == nil {
		return nil
	}

	// Clear existing links
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

			err = b.store.AddLinks(page.ID, page.Title, targets)
			if err != nil {
				return err
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

func (b *LinkService) ClearLinks() error {
	return b.store.Clear()
}

func (b *LinkService) GetBacklinksForPage(pageID string) (*BacklinkResult, error) {
	backlinks, err := b.store.GetBacklinksForPage(pageID)
	return toBacklinkResult(b.treeService, backlinks), err
}

func (b *LinkService) GetOutgoingLinksForPage(pageID string) (*OutgoingResult, error) {
	outgoingLinks, err := b.store.GetOutgoingLinksForPage(pageID)
	return toOutgoingLinkResult(b.treeService, outgoingLinks), err
}

func (b *LinkService) UpdateLinksForPage(page *tree.Page, content string) error {
	links := extractLinksFromMarkdown(content)

	targets := resolveTargetLinks(b.treeService, page.CalculatePath(), links)

	err := b.store.AddLinks(page.ID, page.Title, targets)
	if err != nil {
		return err
	}

	return nil
}

func (b *LinkService) RemoveLinksForPage(pageID string) error {
	return b.store.RemoveLinks(pageID)
}

func (b *LinkService) HealOnPageCreate(page *tree.Page) error {
	toPath := normalizeWikiPath(page.CalculatePath())
	return b.store.HealLinksForPath(toPath, page.ID)
}

func (b *LinkService) Close() error {
	if b.store == nil {
		return nil
	}
	return b.store.Close()
}
