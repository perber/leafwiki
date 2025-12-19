package links

import (
	"net/url"
	"strings"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

type TargetLink struct {
	TargetPageID   string
	TargetPagePath string
	Broken         bool
}

var markdownParser = goldmark.New()

// extractLinksFromMarkdown extracts all links from the given markdown content.
func extractLinksFromMarkdown(content string) []string {
	links := []string{}
	reader := text.NewReader([]byte(content))
	doc := markdownParser.Parser().Parse(reader)

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if link, ok := n.(*ast.Link); ok && entering {
			// ignore external links
			dest := string(link.Destination)
			if strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") || strings.HasPrefix(dest, "mailto:") || strings.HasPrefix(dest, "#") {
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

			links = append(links, dest)
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return []string{}
	}

	return links
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

	base, err := url.Parse("https://leafwiki.com" + folderBase)
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
	root := tree.GetTree()
	if root == nil {
		return nil
	}

	var targetLinks []TargetLink

	for _, link := range links {
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
		page, err := tree.FindPageByRoutePath(root.Children, normalizedForLookup)
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
	root := tree.GetTree()
	if root == nil {
		return BacklinkResultItem{}
	}

	page, err := tree.FindPageByID(root.Children, backlink.FromPageID)
	if err != nil {
		return BacklinkResultItem{}
	}

	return BacklinkResultItem{
		FromPageID: backlink.FromPageID,
		FromTitle:  backlink.FromTitle,
		FromPath:   page.CalculatePath(),
		ToPageID:   backlink.ToPageID,
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
	item := OutgoingResultItem{
		ToPageID:   outgoing.ToPageID,
		ToPath:     outgoing.ToPath,
		Broken:     outgoing.Broken,
		FromPageID: outgoing.FromPageID,
	}

	if outgoing.ToPageID == "" {
		return item
	}

	root := tree.GetTree()
	if root == nil {
		return item
	}

	toPage, err := tree.FindPageByID(root.Children, outgoing.ToPageID)
	if err != nil || toPage == nil {
		item.Broken = true
		return item
	}

	item.ToPageTitle = toPage.Title
	item.ToPath = toPage.CalculatePath()
	item.Broken = false
	return item
}
