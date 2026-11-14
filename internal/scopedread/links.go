package scopedread

import (
	"errors"
	"net/url"
	"strings"

	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// LinkView is a link in a scoped page whose target is a page inside the
// scope. The shared view can't resolve links itself, because it has no route
// paths (TreeNodeView carries none), so it looks them up here. Links to
// anything else are not listed: the view renders them as plain text, the
// same whether the target is outside the scope, unknown or ambiguous.
type LinkView struct {
	// Kind is "href" for a Markdown link or "wikilink" for [[Target]].
	Kind string `json:"kind"`
	// Target is the lookup key. For an href: the destination without query
	// and fragment, percent-decoded (a client takes its rendered href,
	// drops "?…" and "#…" and applies decodeURIComponent per the same rule).
	// For a wikilink: the trimmed target of [[Target]] or [[Target|Alias]].
	Target string `json:"target"`
	PageID string `json:"pageId"`
}

const (
	linkKindHref     = "href"
	linkKindWikilink = "wikilink"
)

// maxScopedLinkTargets caps the distinct targets resolved per page read. Each
// costs a few locked tree reads and the content is the author's, while the
// reader may be anonymous; targets beyond the cap are not listed and show as
// plain text, like any unresolved link.
const maxScopedLinkTargets = 500

// linkSource is what resolving a page's links needs from the tree.
// *tree.TreeService implements it.
type linkSource interface {
	subtreeChecker
	ResolvePermalinkTarget(id string) (*tree.PermalinkTarget, error)
	RouteByPath(routePath string) (tree.RouteEntry, error)
	RoutesByTitle(title string) ([]tree.RouteEntry, error)
}

var _ linkSource = (*tree.TreeService)(nil)

// scopedLinks lists the links in content, the body of page pageID, that lead
// to a page inside scope. Targets are extracted by the link index's rules
// and each is resolved against the whole tree, then checked against the
// scope, so a target outside the scope looks exactly like one that doesn't
// exist.
//
// A [[Title]] deliberately departs from the index: titles are matched only
// among the pages inside the scope, and the "Folder/Title" path hint is tried
// when none of those match. Counting a same-titled page outside the scope
// would make the link ambiguous or suppress the hint, and so tell the holder
// that such a page exists. The normal view may therefore show a [[Title]] as
// ambiguous that the shared view resolves.
//
// The content, the page's path and the lookups are separate tree reads; a
// concurrent move can resolve a link against a slightly newer tree, and every
// listed ID is still checked against the scope at lookup time.
func scopedLinks(src linkSource, scope ReadScope, pageID, content string) ([]LinkView, error) {
	page, err := src.ResolvePermalinkTarget(pageID)
	if errors.Is(err, tree.ErrPageNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	r := linkResolver{src: src, scope: scope, out: []LinkView{}, seen: map[string]bool{}}
	for _, href := range links.ExtractMarkdownLinks(content) {
		target := href
		if decoded, err := url.PathUnescape(href); err == nil {
			target = decoded
		}
		if target == "" || !r.claim(linkKindHref, target) {
			continue
		}
		route, err := markdown.ResolveFolderRelative("/"+page.Path, href)
		if err != nil || route == "/" || route == "" {
			continue
		}
		if err := r.addByPath(linkKindHref, target, route); err != nil {
			return nil, err
		}
	}
	for _, target := range links.ExtractWikiLinkTargets(content) {
		if !r.claim(linkKindWikilink, target) {
			continue
		}
		if err := r.addWikilink(target); err != nil {
			return nil, err
		}
	}
	return r.out, nil
}

// linkResolver collects the in-scope links of one page.
type linkResolver struct {
	src   linkSource
	scope ReadScope
	out   []LinkView
	seen  map[string]bool
}

// claim reports whether target is new and within maxScopedLinkTargets, and
// records it.
func (r *linkResolver) claim(kind, target string) bool {
	key := kind + "\x00" + target
	if r.seen[key] || len(r.seen) >= maxScopedLinkTargets {
		return false
	}
	r.seen[key] = true
	return true
}

// inScope reports whether id is inside the scope; ErrNotFound means no,
// anything else is an operational error.
func (r *linkResolver) inScope(id string) (bool, error) {
	err := r.scope.check(r.src, id)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// addByPath lists target if the page at routePath is inside the scope.
func (r *linkResolver) addByPath(kind, target, routePath string) error {
	entry, err := r.src.RouteByPath(strings.TrimPrefix(routePath, "/"))
	if errors.Is(err, tree.ErrPageNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	ok, err := r.inScope(entry.ID)
	if ok {
		r.out = append(r.out, LinkView{Kind: kind, Target: target, PageID: entry.ID})
	}
	return err
}

// addWikilink lists target if exactly one page inside the scope carries that
// title, or, with none and a "/" in the target, if the path hint leads into
// the scope.
func (r *linkResolver) addWikilink(target string) error {
	matches, err := r.src.RoutesByTitle(target)
	if err != nil {
		return err
	}
	var hits []string
	for _, m := range matches {
		ok, err := r.inScope(m.ID)
		if err != nil {
			return err
		}
		if ok {
			hits = append(hits, m.ID)
		}
	}
	switch {
	case len(hits) == 1:
		r.out = append(r.out, LinkView{Kind: linkKindWikilink, Target: target, PageID: hits[0]})
	case len(hits) == 0 && strings.Contains(target, "/"):
		return r.addByPath(linkKindWikilink, target, target)
	}
	return nil
}
