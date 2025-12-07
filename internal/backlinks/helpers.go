package backlinks

import (
	"path"
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

func normalizeLink(currentPath string, link string) string {
	if link == "" {
		return ""
	}

	var resolved string

	// Absolute link: "/foo/bar"
	if strings.HasPrefix(link, "/") {
		resolved = path.Clean(link[1:])
	} else {
		// Relative link: "../", "./", "child"
		basePathSegments := strings.Split(currentPath, "/")
		segments := strings.Split(link, "/")
		for _, segment := range segments {
			if segment == ".." {
				if len(basePathSegments) > 0 {
					basePathSegments = basePathSegments[:len(basePathSegments)-1]
				}
			} else if segment != "." {
				basePathSegments = append(basePathSegments, segment)
			}
		}
		resolved = path.Clean(strings.Join(basePathSegments, "/"))
	}

	return resolved
}

func resolveTargetLink(treeService *tree.TreeService, nodes []*tree.PageNode, normalizedLink string) *TargetLink {
	page, err := treeService.FindPageByRoutePath(nodes, normalizedLink[1:])
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
