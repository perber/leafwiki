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
	if !b.treeService.IsLoaded() {
		return nil
	}

	if err := b.store.Clear(); err != nil {
		return err
	}

	var ids []string
	if err := b.treeService.WalkNodes(func(id string) error {
		ids = append(ids, id)
		return nil
	}); err != nil {
		return err
	}

	pages, errs := b.treeService.GetPages(ids)
	for i, page := range pages {
		if errs[i] != nil {
			return errs[i]
		}
		links := extractLinksFromMarkdown(page.Content)
		targets := resolveTargetLinks(b.treeService, page.CalculatePath(), links)
		if err := b.store.AddLinks(page.ID, page.Title, targets); err != nil {
			return err
		}
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

func (b *LinkService) GetRefactorMatchesForPrefix(oldPrefix string) ([]RefactorLinkMatch, error) {
	return b.store.GetRefactorMatchesForPrefix(oldPrefix)
}

func (b *LinkService) GetLinkStatusForPage(pageID string, pagePath string) (*LinkStatusResult, error) {
	pagePath = normalizeWikiPath(pagePath)

	// 1) Valid inbound backlinks
	validBacklinks, err := b.store.GetBacklinksForPage(pageID)
	if err != nil {
		return nil, err
	}
	validBacklinksResult := toBacklinkResult(b.treeService, validBacklinks)

	// 2) Broken inbound
	brokenIncoming, err := b.store.GetBrokenIncomingForPath(pagePath)
	if err != nil {
		return nil, err
	}
	brokenIncomingResult := toBacklinkResult(b.treeService, brokenIncoming)

	// 3) Outgoings
	outgoings, err := b.store.GetOutgoingLinksForPage(pageID)
	if err != nil {
		return nil, err
	}
	outgoingResult := toOutgoingLinkResult(b.treeService, outgoings)

	// Split outgoing in broken/non-broken
	okOut := make([]OutgoingResultItem, 0, len(outgoingResult.Outgoings))
	brokenOut := make([]OutgoingResultItem, 0)
	for _, it := range outgoingResult.Outgoings {
		if it.Broken {
			brokenOut = append(brokenOut, it)
		} else {
			okOut = append(okOut, it)
		}
	}

	return &LinkStatusResult{
		Backlinks:       validBacklinksResult.Backlinks,
		BrokenIncoming:  brokenIncomingResult.Backlinks,
		Outgoings:       okOut,
		BrokenOutgoings: brokenOut,
		Counts: LinkStatusCounts{
			Backlinks:       len(validBacklinksResult.Backlinks),
			BrokenIncoming:  len(brokenIncomingResult.Backlinks),
			Outgoings:       len(okOut),
			BrokenOutgoings: len(brokenOut),
		},
	}, nil
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

// DeleteOutgoingLinksForPage removes all outgoing link records for a page.
func (b *LinkService) DeleteOutgoingLinksForPage(pageID string) error {
	return b.store.DeleteOutgoingLinks(pageID)
}

// MarkIncomingLinksBrokenForPage marks all incoming links pointing to pageID as broken.
func (b *LinkService) MarkIncomingLinksBrokenForPage(pageID string) error {
	return b.store.MarkIncomingLinksBroken(pageID)
}

// MarkLinksBrokenForPath marks links pointing to an exact path as broken.
func (b *LinkService) MarkLinksBrokenForPath(toPath string) error {
	toPath = normalizeWikiPath(toPath)
	return b.store.MarkLinksBrokenForPath(toPath)
}

// MarkLinksBrokenForPrefix marks all links under a prefix as broken (subtree move/delete).
func (b *LinkService) MarkLinksBrokenForPrefix(prefix string) error {
	prefix = normalizeWikiPath(prefix)
	return b.store.MarkLinksBrokenForPrefix(prefix)
}

func (b *LinkService) HealLinksForExactPath(page *tree.Page) error {
	toPath := normalizeWikiPath(page.CalculatePath())
	return b.store.HealLinksForPath(toPath, page.ID)
}

func (b *LinkService) Close() error {
	if b.store == nil {
		return nil
	}
	return b.store.Close()
}
