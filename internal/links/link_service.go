package links

import (
	"context"

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
	return b.IndexAllPagesContext(context.Background())
}

func (b *LinkService) IndexAllPagesContext(ctx context.Context) error {
	if !b.treeService.IsLoaded() {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := b.store.Clear(); err != nil {
		return err
	}

	var ids []string
	if err := b.treeService.WalkNodes(func(id string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	}); err != nil {
		return err
	}

	pages, errs := b.treeService.GetPages(ids)
	for i, page := range pages {
		if err := ctx.Err(); err != nil {
			return err
		}
		if errs[i] != nil {
			return errs[i]
		}
		targets := collectTargetsFromContent(b.treeService, page.CalculatePath(), page.Content)
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
	pageTitle := ""
	if page, err := b.treeService.GetPage(pageID); err == nil {
		pageTitle = page.Title
	}

	backlinks, err := b.store.GetBacklinksForPage(pageID)
	if err != nil {
		return nil, err
	}
	backlinks, err = b.mergeAmbiguousWikiLinksIntoBacklinks(pageID, pageTitle, backlinks)
	if err != nil {
		return nil, err
	}
	return toBacklinkResult(b.treeService, backlinks), err
}

func (b *LinkService) GetOutgoingLinksForPage(pageID string) (*OutgoingResult, error) {
	outgoingLinks, err := b.store.GetOutgoingLinksForPage(pageID)
	return toOutgoingLinkResult(b.treeService, outgoingLinks), err
}

func (b *LinkService) GetRefactorMatchesForPrefix(oldPrefix string) ([]RefactorLinkMatch, error) {
	return b.store.GetRefactorMatchesForPrefix(oldPrefix)
}

func (b *LinkService) GetRefactorSourcePageIDsForPrefix(oldPrefix string) ([]string, error) {
	return b.store.GetRefactorSourcePageIDsForPrefix(oldPrefix)
}

func (b *LinkService) GetRefactorSourcePageIDsForWikiLinkTitle(title string) ([]string, error) {
	return b.store.GetRefactorSourcePageIDsForWikiLinkTitle(title)
}

func (b *LinkService) UpdateRewrittenLinksAndHealForPages(pages []*tree.Page, rules []RewriteRule) error {
	outgoingByPageID, err := b.store.GetOutgoingLinksForPages(pageIDsForPages(pages))
	if err != nil {
		return err
	}

	updates := make([]PageLinkUpdate, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			continue
		}
		pagePath := normalizeWikiPath(page.CalculatePath())
		targets := rewriteResolvedTargets(pagePath, outgoingByPageID[page.ID], rules, b.treeService)
		updates = append(updates, PageLinkUpdate{
			FromPageID: page.ID,
			FromTitle:  page.Title,
			ToPath:     pagePath,
			Targets:    targets,
		})
	}

	if len(updates) == 0 {
		return nil
	}

	return b.store.ReplaceLinksAndHeal(updates)
}

func (b *LinkService) GetLinkStatusForPage(pageID string, pagePath string) (*LinkStatusResult, error) {
	pagePath = normalizeWikiPath(pagePath)
	page, err := b.treeService.GetPage(pageID)
	if err != nil {
		return nil, err
	}

	// 1) Valid inbound backlinks
	validBacklinks, err := b.store.GetBacklinksForPage(pageID)
	if err != nil {
		return nil, err
	}
	validBacklinks, err = b.mergeAmbiguousWikiLinksIntoBacklinks(pageID, page.Title, validBacklinks)
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
	// Split outgoing in broken/non-broken
	okOut := make([]OutgoingResultItem, 0, len(outgoings))
	brokenOut := make([]OutgoingResultItem, 0)
	for _, outgoing := range outgoings {
		item := toOutgoingResultItem(b.treeService, outgoing)
		if outgoing.Broken && b.isAmbiguousWikilinkOutgoing(outgoing) {
			item.Broken = false
			okOut = append(okOut, item)
			continue
		}
		if item.Broken {
			brokenOut = append(brokenOut, item)
		} else {
			okOut = append(okOut, item)
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

func (b *LinkService) mergeAmbiguousWikiLinksIntoBacklinks(pageID string, pageTitle string, backlinks []Backlink) ([]Backlink, error) {
	if pageTitle == "" {
		return backlinks, nil
	}

	matches := b.treeService.FindPagesByTitle(pageTitle)
	if len(matches) <= 1 {
		return backlinks, nil
	}

	isMatchingPage := false
	for _, match := range matches {
		if match != nil && match.ID == pageID {
			isMatchingPage = true
			break
		}
	}
	if !isMatchingPage {
		return backlinks, nil
	}

	ambiguousRefs, err := b.store.GetBrokenIncomingForPath(wikilinkSentinel(pageTitle))
	if err != nil {
		return nil, err
	}
	if len(ambiguousRefs) == 0 {
		return backlinks, nil
	}

	seen := make(map[string]struct{}, len(backlinks))
	merged := make([]Backlink, 0, len(backlinks)+len(ambiguousRefs))
	for _, backlink := range backlinks {
		key := backlink.FromPageID + "\x00" + backlink.ToPageID
		seen[key] = struct{}{}
		merged = append(merged, backlink)
	}

	for _, backlink := range ambiguousRefs {
		backlink.ToPageID = pageID
		backlink.Broken = false
		key := backlink.FromPageID + "\x00" + backlink.ToPageID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, backlink)
	}

	return merged, nil
}

func (b *LinkService) isAmbiguousWikilinkOutgoing(outgoing Outgoing) bool {
	if !outgoing.Broken || !IsWikilinkSentinel(outgoing.ToPath) {
		return false
	}

	return len(b.treeService.FindPagesByTitle(WikilinkTitleFromSentinel(outgoing.ToPath))) > 1
}

func (b *LinkService) UpdateLinksForPage(page *tree.Page, content string) error {
	targets := collectTargetsFromContent(b.treeService, page.CalculatePath(), content)
	return b.store.AddLinks(page.ID, page.Title, targets)
}

func (b *LinkService) UpdateLinksAndHealForPages(pages []*tree.Page) error {
	updates := make([]PageLinkUpdate, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			continue
		}
		pagePath := normalizeWikiPath(page.CalculatePath())
		targets := collectTargetsFromContent(b.treeService, pagePath, page.Content)
		updates = append(updates, PageLinkUpdate{
			FromPageID: page.ID,
			FromTitle:  page.Title,
			ToPath:     pagePath,
			Targets:    targets,
		})
	}

	if len(updates) == 0 {
		return nil
	}

	return b.store.ReplaceLinksAndHeal(updates)
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

// HealWikiLinksForPage heals broken [[Title]] sentinel records that target
// this page's title, but only when exactly one page with that title exists.
// If the title is shared by multiple pages the link is ambiguous and must
// remain as a broken sentinel.
func (b *LinkService) HealWikiLinksForPage(page *tree.Page) error {
	if len(b.treeService.FindPagesByTitle(page.Title)) != 1 {
		return nil
	}
	return b.store.HealWikiLinksForTitle(page.Title, page.ID)
}

// HealWikiLinksForTitleIfUnambiguous heals broken [[Title]] sentinels when
// exactly one page with that title now exists. Called after a page is deleted
// so that formerly ambiguous wikilinks become resolved if only one candidate
// remains.
func (b *LinkService) HealWikiLinksForTitleIfUnambiguous(title string) error {
	if title == "" {
		return nil
	}
	matches := b.treeService.FindPagesByTitle(title)
	if len(matches) != 1 {
		return nil
	}
	return b.store.HealWikiLinksForTitle(title, matches[0].ID)
}

func (b *LinkService) Close() error {
	if b.store == nil {
		return nil
	}
	return b.store.Close()
}

func pageIDsForPages(pages []*tree.Page) []string {
	ids := make([]string, 0, len(pages))
	for _, page := range pages {
		if page == nil {
			continue
		}
		ids = append(ids, page.ID)
	}
	return ids
}

func rewriteResolvedTargets(currentPath string, outgoings []Outgoing, rules []RewriteRule, treeService *tree.TreeService) []TargetLink {
	if len(outgoings) == 0 {
		return nil
	}

	paths := make([]string, 0, len(outgoings))
	for _, outgoing := range outgoings {
		if IsWikilinkSentinel(outgoing.ToPath) {
			// Title-based wiki-link sentinels are resolved by title, not path.
			// Skip path rewriting — they are healed separately by HealWikiLinksForPage.
			continue
		}
		targetPath := normalizeWikiPath(outgoing.ToPath)
		if rewritten, ok := applyRewriteRules(targetPath, rules); ok {
			targetPath = rewritten
		}
		paths = append(paths, targetPath)
	}

	return resolveTargetLinks(treeService, currentPath, paths)
}
