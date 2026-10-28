package render

import (
	"bytes"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// wikiLinkParser turns [[Target]] and [[Target|Alias]] into a link or plain
// text. As an inline parser it never sees code spans or code blocks, so
// wikilinks inside code stay literal without a separate filter.
type wikiLinkParser struct{}

func (wikiLinkParser) Trigger() []byte { return []byte{'['} }

func (wikiLinkParser) Parse(_ ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) < 2 || line[1] != '[' {
		return nil
	}
	// Neither target nor alias may contain "]", so a match starting here ends
	// at the first "]]"; matching only up to it keeps lines with many "["
	// linear.
	end := bytes.Index(line, []byte("]]"))
	if end < 0 {
		return nil
	}
	links := markdown.ParseWikiLinks(string(line[:end+2]))
	if len(links) == 0 || links[0].Start != 0 {
		return nil
	}
	link := links[0]
	block.Advance(link.End)

	label := strings.TrimSpace(link.Alias)
	if label == "" {
		label = link.Target
	}
	textNode := ast.NewString([]byte(label))
	resolve := optionsFrom(pc).ResolveWikiLink
	if resolve == nil {
		return textNode
	}
	route, ok := resolve(link.Target)
	route = markdown.NormalizeRoutePath(route)
	if !ok || route == "" {
		return textNode
	}
	// An escaped absolute route; linkTransformer adds the base path like for
	// any other absolute link.
	n := ast.NewLink()
	n.Destination = []byte(escapeSegments(route))
	n.AppendChild(n, textNode)
	return n
}
