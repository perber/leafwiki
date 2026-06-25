package links

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// wikiLinkRe matches [[Target]] and [[Target|Alias]] syntax.
// Capture group 1 is the target (title or path hint).
// \S as first character rejects bash-style [[ -n ... ]] conditionals (which always
// start with a space after [[) without affecting real wiki-link titles.
var wikiLinkRe = regexp.MustCompile(`\[\[(\S[^\]|#\n]*?)(?:\|[^\]\n]+?)?\]\]`)

// wikilinkSentinelPrefix is the to_path prefix used for broken wiki-link
// records in the link store. It must never collide with real wiki route paths
// (whose segments match ^[a-zA-Z0-9]+(-[a-zA-Z0-9]+)*$).
const wikilinkSentinelPrefix = "wikilink:"

func wikilinkSentinel(target string) string {
	return wikilinkSentinelPrefix + target
}

// IsWikilinkSentinel reports whether a to_path value is a wiki-link sentinel
// (i.e. stored as "wikilink:Title" rather than a real route path).
func IsWikilinkSentinel(toPath string) bool {
	return strings.HasPrefix(toPath, wikilinkSentinelPrefix)
}

// WikilinkTitleFromSentinel extracts the title from a sentinel path.
func WikilinkTitleFromSentinel(toPath string) string {
	return strings.TrimPrefix(toPath, wikilinkSentinelPrefix)
}

type TargetLink struct {
	TargetPageID   string
	TargetPagePath string
	Broken         bool
}

var markdownParser = goldmark.New()

func isAssetLinkDestination(dest string) bool {
	dest = strings.TrimSpace(dest)
	dest = strings.TrimPrefix(dest, "<")
	dest = strings.TrimSuffix(dest, ">")

	return strings.HasPrefix(dest, "/assets/") || strings.HasPrefix(dest, "assets/")
}

// extractLinksFromMarkdown extracts all links from the given markdown content.
func extractLinksFromMarkdown(content string) []string {
	links := []string{}
	reader := text.NewReader([]byte(content))
	doc := markdownParser.Parser().Parse(reader)

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, ok := n.(*ast.Link); ok && entering {
			// ignore external links
			dest := string(link.Destination)
			lower := strings.ToLower(dest)
			if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "#") {
				return ast.WalkContinue, nil
			}
			// strip hash fragments
			if idx := strings.Index(dest, "#"); idx != -1 {
				dest = dest[:idx]
			}
			// strip query parameters
			if idx := strings.Index(dest, "?"); idx != -1 {
				dest = dest[:idx]
			}
			if isAssetLinkDestination(dest) {
				return ast.WalkContinue, nil
			}

			links = append(links, dest)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return []string{}
	}

	return links
}

// extractWikiLinksFromMarkdown returns all target strings from [[Target]] and
// [[Target|Alias]] syntax found in content. The returned strings are the raw
// target values (may be a title or a "Folder/Title" path hint).
func extractWikiLinksFromMarkdown(content string) []string {
	excluded := NewMarkdownRefactorEngine().collectExcludedRanges(content)
	matches := wikiLinkRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	targets := make([]string, 0, len(matches))
	for _, m := range matches {
		if isExcludedOffset(m[0], excluded) || m[2] == -1 {
			continue
		}
		target := strings.TrimSpace(content[m[2]:m[3]])
		if target == "" {
			continue
		}
		if _, dup := seen[target]; dup {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

// resolveWikiLinkTargets resolves [[Title]] and [[Folder/Title]] targets.
//
// Targets containing "/" are first tried as direct route-path lookups
// ([[Folder/Title]] → /folder/title). If the path lookup fails, the target
// is retried as a full title (handles titles like "C/C++"). A single title
// match resolves the link; zero or N>1 matches produce a broken sentinel
// stored as "wikilink:<target>" so the healing infrastructure can later
// find and fix the record when a matching page is created.
func resolveWikiLinkTargets(treeService *tree.TreeService, targets []string) []TargetLink {
	if !treeService.IsLoaded() || len(targets) == 0 {
		return nil
	}

	var result []TargetLink
	for _, target := range targets {
		if strings.Contains(target, "/") {
			routePath := strings.TrimPrefix(target, "/")
			page, err := treeService.FindPageByRoutePath(routePath)
			if err == nil && page != nil {
				result = append(result, TargetLink{
					TargetPageID:   page.ID,
					TargetPagePath: normalizeWikiPath(page.CalculatePath()),
					Broken:         false,
				})
				continue
			}
			// Path lookup failed — fall through to title lookup so that
			// titles containing "/" (e.g. "C/C++") can still be resolved.
			pages := treeService.FindPagesByTitle(target)
			if len(pages) == 1 {
				result = append(result, TargetLink{
					TargetPageID:   pages[0].ID,
					TargetPagePath: wikilinkSentinel(target),
					Broken:         false,
				})
				continue
			}
			// Store as a normal broken route path so HealLinksForExactPath
			// can heal it when the page is later created at that path.
			result = append(result, TargetLink{
				Broken:         true,
				TargetPagePath: "/" + routePath,
			})
			continue
		}

		// Pure title-based lookup.
		pages := treeService.FindPagesByTitle(target)
		if len(pages) == 1 {
			result = append(result, TargetLink{
				TargetPageID:   pages[0].ID,
				TargetPagePath: wikilinkSentinel(target),
				Broken:         false,
			})
		} else {
			// 0 matches (not found) or N>1 (ambiguous) → broken sentinel.
			result = append(result, TargetLink{
				Broken:         true,
				TargetPagePath: wikilinkSentinel(target),
			})
		}
	}
	return result
}

// collectTargetsFromContent extracts and resolves all link targets from a page's
// content — both standard Markdown links and [[Title]] wiki-link syntax.
func collectTargetsFromContent(treeService *tree.TreeService, pagePath string, content string) []TargetLink {
	mdLinks := extractLinksFromMarkdown(content)
	mdTargets := resolveTargetLinks(treeService, pagePath, mdLinks)

	wikiTargets := extractWikiLinksFromMarkdown(content)
	wikiResolved := resolveWikiLinkTargets(treeService, wikiTargets)

	return append(mdTargets, wikiResolved...)
}

// normalizeWikiPath normalizes a wiki path:
// - removes query/hash (if any)
// - ensures leading "/"
// - removes trailing "/" (except root "/")
func normalizeWikiPath(p string) string {
	if p == "" {
		return ""
	}

	// strip hash/query defensively (caller already does, but keep consistent)
	if i := strings.Index(p, "#"); i != -1 {
		p = p[:i]
	}
	if i := strings.Index(p, "?"); i != -1 {
		p = p[:i]
	}

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// collapse multiple slashes
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}

	// strip trailing slash except root
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}

	return p
}

// resolveURLPath resolves href against currentPath using "page is folder" semantics.
func resolveURLPath(currentPath, href string) (string, error) {
	currentPath = normalizeWikiPath(currentPath)

	// treat currentPath as folder by forcing trailing slash
	folderBase := currentPath
	if !strings.HasSuffix(folderBase, "/") {
		folderBase += "/"
	}

	base, err := url.Parse("https://example.com" + folderBase)
	if err != nil {
		return "", err
	}

	ref, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	resolved := base.ResolveReference(ref)

	// normalize result path (strip trailing slash etc.)
	return normalizeWikiPath(resolved.Path), nil
}

func resolveTargetLinks(tree *tree.TreeService, currentPath string, links []string) []TargetLink {
	if !tree.IsLoaded() {
		return nil
	}

	var targetLinks []TargetLink

	for _, link := range links {
		if isAssetLinkDestination(link) {
			continue
		}

		// resolve link against current path
		resolvedPath, err := resolveURLPath(currentPath, link)
		if err != nil || resolvedPath == "" {
			continue
		}

		// normalize for lookup (by stripping leading "/")
		normalizedForLookup := strings.TrimPrefix(resolvedPath, "/")
		if normalizedForLookup == "" {
			continue
		}

		// find page by route path
		page, err := tree.FindPageByRoutePath(normalizedForLookup)
		if err == nil && page != nil {
			// found page
			targetLinks = append(targetLinks, TargetLink{
				TargetPageID:   page.ID,
				TargetPagePath: resolvedPath,
				Broken:         false,
			})
		} else {
			// not found, broken link
			targetLinks = append(targetLinks, TargetLink{
				TargetPageID:   "",
				TargetPagePath: resolvedPath,
				Broken:         true,
			})
		}
	}

	return targetLinks
}

func toBacklinkResult(tree *tree.TreeService, backlinks []Backlink) *BacklinkResult {
	var items []BacklinkResultItem
	for _, backlink := range backlinks {
		item := toBacklinkResultItem(tree, backlink)
		items = append(items, item)
	}
	return &BacklinkResult{
		Backlinks: items,
		Count:     len(items),
	}
}

func toBacklinkResultItem(tree *tree.TreeService, backlink Backlink) BacklinkResultItem {
	if !tree.IsLoaded() {
		return BacklinkResultItem{}
	}

	page, err := tree.FindPageByID(backlink.FromPageID)
	if err != nil {
		return BacklinkResultItem{}
	}

	return BacklinkResultItem{
		FromPageID: backlink.FromPageID,
		FromTitle:  backlink.FromTitle,
		FromPath:   page.CalculatePath(),
		ToPageID:   backlink.ToPageID,
		Broken:     backlink.Broken,
	}
}

func toOutgoingLinkResult(tree *tree.TreeService, outgoings []Outgoing) *OutgoingResult {
	var items []OutgoingResultItem
	for _, outgoing := range outgoings {
		item := toOutgoingResultItem(tree, outgoing)
		items = append(items, item)
	}
	return &OutgoingResult{
		Outgoings: items,
		Count:     len(items),
	}
}

func toOutgoingResultItem(tree *tree.TreeService, outgoing Outgoing) OutgoingResultItem {
	displayPath := outgoing.ToPath
	if IsWikilinkSentinel(outgoing.ToPath) {
		displayPath = WikilinkTitleFromSentinel(outgoing.ToPath)
	}
	item := OutgoingResultItem{
		ToPageID:   outgoing.ToPageID,
		ToPath:     displayPath,
		Broken:     outgoing.Broken,
		FromPageID: outgoing.FromPageID,
	}

	if outgoing.ToPageID == "" {
		return item
	}

	if !tree.IsLoaded() {
		return item
	}

	toPage, err := tree.FindPageByID(outgoing.ToPageID)
	if err != nil || toPage == nil {
		return item
	}

	item.ToPageTitle = toPage.Title
	return item
}
