package markdown

import (
	"net/url"
	"regexp"
	"strings"
)

// wikiLinkRe matches [[Target]] and [[Target|Alias]] syntax.
// Capture group 1 is the target (title or path hint), group 2 the optional alias.
// \S as first character rejects bash-style [[ -n ... ]] conditionals (which always
// start with a space after [[) without affecting real wiki-link titles.
var wikiLinkRe = regexp.MustCompile(`\[\[(\S[^\]|#\n]*?)(?:\|([^\]\n]+?))?\]\]`)

// WikiLink is one [[Target]] or [[Target|Alias]] occurrence in markdown content.
type WikiLink struct {
	// Target is the title or "Folder/Title" path hint, with surrounding whitespace trimmed.
	Target string
	// Alias is the text after "|", or "" when there is none.
	Alias string
	// Start and End are the byte offsets of the whole [[...]] match in the content.
	Start, End int
}

// ParseWikiLinks returns every wiki link in content in document order, duplicates
// included. It does not know about code spans or blocks: callers that must skip
// links inside code filter by Start.
func ParseWikiLinks(content string) []WikiLink {
	matches := wikiLinkRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}
	links := make([]WikiLink, 0, len(matches))
	for _, m := range matches {
		link := WikiLink{
			Target: strings.TrimSpace(content[m[2]:m[3]]),
			Start:  m[0],
			End:    m[1],
		}
		if m[4] != -1 {
			link.Alias = content[m[4]:m[5]]
		}
		links = append(links, link)
	}
	return links
}

// NormalizeRoutePath normalizes a wiki route path:
// - removes query/hash (if any)
// - ensures leading "/"
// - collapses repeated "/"
// - removes trailing "/" (except root "/")
//
// The empty string stays empty.
func NormalizeRoutePath(p string) string {
	if p == "" {
		return ""
	}

	// strip hash/query defensively (callers usually already do)
	if i := strings.Index(p, "#"); i != -1 {
		p = p[:i]
	}
	if i := strings.Index(p, "?"); i != -1 {
		p = p[:i]
	}

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}

	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}

	return p
}

// ResolveFolderRelative resolves href against currentPath using "page is folder"
// semantics: a relative href on page /docs points below it (/docs/<href>), not
// next to it. The result is normalized with NormalizeRoutePath.
func ResolveFolderRelative(currentPath, href string) (string, error) {
	currentPath = NormalizeRoutePath(currentPath)

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

	return NormalizeRoutePath(resolved.Path), nil
}
