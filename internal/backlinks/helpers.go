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

func normalizeLink(currentPath string, link string) string {
	if link == "" {
		return ""
	}

	resolvedPath, err := resolveURLPath(currentPath, link)
	if err != nil || resolvedPath == "" {
		return ""
	}

	// strip leading "/" for your tree lookup
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
