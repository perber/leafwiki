package links

import (
	"context"
	"strings"

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
		// Ambiguous links are not broken: they resolve to every matching page
		// and are listed as backlinks there.
		if outgoing.State == LinkBroken {
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

	ambiguousRefs, err := b.store.GetAmbiguousIncomingForTitle(pageTitle)
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

// ReconcileTitle recomputes the state of every [[title]] wiki-link record
// against the current tree (see classifyTitleMatches). Call it whenever the
// set of pages carrying this title changed: page created, restored, renamed
// (old and new title) or deleted. Every wiki-link to a title shares one
// "wikilink:<title>" key, so this is a single UPDATE and never needs the
// source pages.
func (b *LinkService) ReconcileTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	matches := b.treeService.FindPagesByTitle(title)
	pageID := ""
	if len(matches) == 1 {
		pageID = matches[0].ID
	}
	return b.store.ReconcileWikiLinksForTitle(title, classifyTitleMatches(len(matches)), pageID)
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

// rewriteResolvedTargets rebuilds a page's link rows after a path/title
// rewrite. ReplaceLinksAndHeal replaces ALL rows of the page, so every stored
// outgoing link must come back out of here or its row is lost.
//
//   - Path links are rewritten by the path rules and re-resolved.
//   - Wiki-link sentinels are title-based, never path-rewritten (that would
//     mangle them into "/wikilink:..."). A rename rule (OldTitle → NewTitle)
//     already rewrote [[OldTitle]] in the content, so the row follows it. The
//     title is then re-resolved against the current tree.
func rewriteResolvedTargets(currentPath string, outgoings []Outgoing, rules []RewriteRule, treeService *tree.TreeService) []TargetLink {
	if len(outgoings) == 0 {
		return nil
	}

	paths := make([]string, 0, len(outgoings))
	var wikiTitles []string
	for _, outgoing := range outgoings {
		if IsWikilinkSentinel(outgoing.ToPath) {
			wikiTitles = append(wikiTitles, rewriteWikiTitle(WikilinkTitleFromSentinel(outgoing.ToPath), rules))
			continue
		}
		targetPath := normalizeWikiPath(outgoing.ToPath)
		if rewritten, ok := applyRewriteRules(targetPath, rules); ok {
			targetPath = rewritten
		}
		paths = append(paths, targetPath)
	}

	result := resolveTargetLinks(treeService, currentPath, paths)
	return append(result, resolveWikiLinkTargets(treeService, wikiTitles)...)
}

// rewriteWikiTitle applies a rename rule's OldTitle → NewTitle to a wiki-link
// title, matching case-insensitively like the content rewrite does.
func rewriteWikiTitle(title string, rules []RewriteRule) string {
	for _, rule := range rules {
		if rule.OldTitle != "" && rule.OldTitle != rule.NewTitle && strings.EqualFold(rule.OldTitle, title) {
			return rule.NewTitle
		}
	}
	return title
}

func (b *LinkService) GetBrokenLinks() ([]BrokenLink, error) {
	links, err := b.store.GetBrokenLinks()
	if err != nil {
		return nil, err
	}

	for i := range links {
		page, err := b.treeService.GetPage(links[i].FromPageID)
		if err != nil {
			return nil, err
		}

		links[i].FromPath = page.CalculatePath()
	}

	return links, nil
}
