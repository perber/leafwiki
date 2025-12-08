package backlinks

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
}

var markdownParser = goldmark.New()

// extractLinksFromMarkdown extracts all links from the given markdown content.
func extractLinksFromMarkdown(content string) []string {
	links := []string{}
	reader := text.NewReader([]byte(content))
	doc := markdownParser.Parser().Parse(reader)

	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
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

	return links
}

// resolveURLPath resolves a relative link (href) against a currentPath,
// using browser-like URL semantics. Returns a path starting with "/".
func resolveURLPath(currentPath, href string) (string, error) {
	// Ensure currentPath starts with "/"
	if !strings.HasPrefix(currentPath, "/") {
		currentPath = "/" + currentPath
	}

	// Fake origin, we only care about the path resolution
	// This allows us to use the URL package to resolve paths correctly like a browser would
	base, err := url.Parse("https://leafwiki.com" + currentPath)
	if err != nil {
		return "", err
	}

	ref, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	resolved := base.ResolveReference(ref)
	return resolved.Path, nil
}

func normalizeLink(currentPath string, link string) string {
	if link == "" {
		return ""
	}

	resolvedPath, err := resolveURLPath(currentPath, link)
	if err != nil {
		return ""
	}

	// strip leading "/"
	return strings.TrimPrefix(resolvedPath, "/")
}

func resolveTargetLink(treeService *tree.TreeService, nodes []*tree.PageNode, normalizedLink string) *TargetLink {
	page, err := treeService.FindPageByRoutePath(nodes, normalizedLink)
	if err != nil || page == nil {
		return nil
	}

	return &TargetLink{
		TargetPageID:   page.ID,
		TargetPagePath: page.CalculatePath(),
	}
}

func resolveTargetLinks(tree *tree.TreeService, currentPath string, links []string) []TargetLink {
	root := tree.GetTree()
	if root == nil {
		return nil
	}
	var targetLinks []TargetLink
	for _, link := range links {
		normalized := normalizeLink(currentPath, link)
		if normalized == "" {
			continue
		}
		targetLink := resolveTargetLink(tree, root.Children, normalized)
		if targetLink != nil {
			targetLinks = append(targetLinks, *targetLink)
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
	root := tree.GetTree()
	if root == nil {
		return OutgoingResultItem{}
	}

	toPage, err := tree.FindPageByID(root.Children, outgoing.ToPageID)
	if err != nil {
		return OutgoingResultItem{}
	}

	return OutgoingResultItem{
		ToPageID:    outgoing.ToPageID,
		ToPageTitle: toPage.Title,
		ToPath:      toPage.CalculatePath(),
		FromPageID:  outgoing.FromPageID,
	}
}
