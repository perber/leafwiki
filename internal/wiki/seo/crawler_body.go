package seo

import (
	"container/list"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"io"
	"strings"
	"sync"

	"github.com/perber/wiki/internal/render"
)

// ─── GetCrawlerBodyUseCase ───────────────────────────────────────────────────

// ChildPage is a direct child of a page, as the crawler body links to it.
type ChildPage struct {
	Title string
	// RoutePath is the child's route path without a leading "/".
	RoutePath string
}

// CrawlerBodyReader reads what the crawler body needs besides the page: its
// children and the targets of its wikilinks.
type CrawlerBodyReader interface {
	// ChildPages returns the page's direct children in navigation order; none
	// when it can't tell (tree not loaded yet).
	ChildPages(ctx context.Context, pageID string) ([]ChildPage, error)
	// ResolveWikiLink returns the route path of the one page a [[target]]
	// points at; ok is false for an unknown or ambiguous target.
	ResolveWikiLink(ctx context.Context, target string) (routePath string, ok bool)
}

// GetCrawlerBodyInput is the input for GetCrawlerBodyUseCase.
type GetCrawlerBodyInput struct {
	// RoutePath is the shell request's route path, starting with "/".
	RoutePath string
	// BasePath is the URL prefix the wiki is served under ("" or e.g. "/wiki").
	BasePath  string
	UserAgent string
}

// GetCrawlerBodyOutput is the page as HTML for #root.
type GetCrawlerBodyOutput struct {
	// HTML is trusted markup: the title and child links are escaped here, the
	// page content is sanitized by internal/render.
	HTML string
}

const (
	// maxParallelRenders caps concurrent renders; any client can claim to be
	// a crawler, and a render can take most of a second on small hardware.
	maxParallelRenders = 2
	// maxRenderCacheBytes bounds the rendered HTML kept between requests.
	maxRenderCacheBytes = 16 << 20
)

// errRendersBusy means maxParallelRenders renders are running; the request
// gets the plain shell instead of waiting.
var errRendersBusy = errors.New("seo: too many crawler body renders running")

// errRenderFailed replaces a render error other than render.ErrTooComplex.
var errRenderFailed = errors.New("seo: crawler body render failed")

// GetCrawlerBodyUseCase renders the page a search-engine crawler asks for,
// so the shell carries its content before (or without) the SPA running.
type GetCrawlerBodyUseCase struct {
	pages   PageReader
	reader  CrawlerBodyReader
	render  func(content string, opt render.Options) (string, error)
	renders chan struct{}
	cache   *renderCache
}

// NewGetCrawlerBodyUseCase constructs a GetCrawlerBodyUseCase.
func NewGetCrawlerBodyUseCase(pages PageReader, reader CrawlerBodyReader) *GetCrawlerBodyUseCase {
	return &GetCrawlerBodyUseCase{
		pages:   pages,
		reader:  reader,
		render:  render.RenderHTML,
		renders: make(chan struct{}, maxParallelRenders),
		cache:   &renderCache{entries: map[renderKey]*list.Element{}},
	}
}

// Execute returns the page's body, or nil when the user agent is no search
// engine, the route is the root or a permalink (then nothing is read), no
// page is there, the page failed to render before, or the client went away
// before the render. The first render
// failure and a refused render (too many running) are returned as errors.
func (uc *GetCrawlerBodyUseCase) Execute(ctx context.Context, in GetCrawlerBodyInput) (*GetCrawlerBodyOutput, error) {
	routePath := pageRoutePath(in.RoutePath)
	if routePath == "" || !isSearchEngineCrawler(in.UserAgent) {
		return nil, nil
	}
	// A permalink shows its target page in the SPA; its canonical link sends
	// search engines there, so it gets no body of its own (ADR-0018).
	if _, ok := permalinkID(routePath); ok {
		return nil, nil
	}
	page, err := uc.pages.PageByRoutePath(ctx, routePath)
	if errors.Is(err, ErrPageNotFound) {
		return nil, nil
	}
	if err != nil || page == nil {
		return nil, err
	}
	children, err := uc.reader.ChildPages(ctx, page.ID)
	if err != nil {
		return nil, err
	}
	content, ok, err := uc.renderContent(ctx, in.BasePath, routePath, page.Body)
	if err != nil || !ok {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("<article><h1>" + html.EscapeString(page.Title) + "</h1>" + content + "</article>")
	var items strings.Builder
	for _, child := range children {
		href, err := wikiPath(in.BasePath, child.RoutePath)
		if err != nil {
			continue
		}
		items.WriteString(`<li><a href="` + html.EscapeString(href) + `">` + html.EscapeString(child.Title) + `</a></li>`)
	}
	if items.Len() > 0 {
		b.WriteString("<nav><ul>" + items.String() + "</ul></nav>")
	}
	return &GetCrawlerBodyOutput{HTML: b.String()}, nil
}

// renderContent returns the page content as sanitized HTML from the cache
// or a new render. ok is false for a page whose render failed before and
// when the client went away before the render.
func (uc *GetCrawlerBodyUseCase) renderContent(ctx context.Context, basePath, routePath, content string) (string, bool, error) {
	key := renderKeyOf(basePath, routePath, content)
	if cached := uc.cache.get(key); cached != nil && uc.linksUnchanged(ctx, cached.links) {
		return cached.html, !cached.failed, nil
	}
	select {
	case uc.renders <- struct{}{}:
		defer func() { <-uc.renders }()
	default:
		return "", false, errRendersBusy
	}
	if ctx.Err() != nil {
		// The client went away: don't spend the slot on a render nobody
		// receives, and cache nothing.
		return "", false, nil
	}
	links := map[string]wikiLinkTarget{}
	rendered, err := uc.render(content, render.Options{
		BasePath:  basePath,
		RoutePath: "/" + routePath,
		ResolveWikiLink: func(target string) (string, bool) {
			routePath, ok := uc.reader.ResolveWikiLink(ctx, target)
			links[target] = wikiLinkTarget{routePath: routePath, ok: ok}
			return routePath, ok
		},
	})
	uc.cache.put(key, &renderedPage{html: rendered, failed: err != nil, links: links})
	if errors.Is(err, render.ErrTooComplex) {
		return "", false, err
	}
	if err != nil {
		// The cause may quote page markup; it must not reach the log.
		return "", false, errRenderFailed
	}
	return rendered, true, nil
}

// linksUnchanged reports whether every wikilink still resolves as it did when
// the page was rendered; a moved or renamed target page means a new render.
func (uc *GetCrawlerBodyUseCase) linksUnchanged(ctx context.Context, links map[string]wikiLinkTarget) bool {
	for target, was := range links {
		routePath, ok := uc.reader.ResolveWikiLink(ctx, target)
		if (wikiLinkTarget{routePath: routePath, ok: ok}) != was {
			return false
		}
	}
	return true
}

// searchEngineCrawlers are lower-case substrings of the user agents of the
// search engines that get a rendered body. Anyone can send them; the body
// is the same public content the SPA shows, so that is harmless.
// google-inspectiontool is Search Console's URL Inspection and the Rich
// Results Test, so site owners can check what Google sees.
var searchEngineCrawlers = []string{"googlebot", "google-inspectiontool", "bingbot", "duckduckbot", "yandex", "baiduspider", "applebot"}

func isSearchEngineCrawler(userAgent string) bool {
	userAgent = strings.ToLower(userAgent)
	for _, crawler := range searchEngineCrawlers {
		if strings.Contains(userAgent, crawler) {
			return true
		}
	}
	return false
}

// renderKey identifies a page version: base path, route path (relative
// links resolve below it) and content.
type renderKey [sha256.Size]byte

func renderKeyOf(basePath, routePath, content string) renderKey {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%d:%s%d:%s", len(basePath), basePath, len(routePath), routePath)
	_, _ = io.WriteString(h, content)
	var key renderKey
	h.Sum(key[:0])
	return key
}

type wikiLinkTarget struct {
	routePath string
	ok        bool
}

// renderedPage is one cached render; it is never changed once cached.
type renderedPage struct {
	html   string
	failed bool
	// links are the wikilink targets as they resolved during the render.
	links map[string]wikiLinkTarget
}

func (p *renderedPage) cost() int {
	n := len(p.html) + 128
	for target, link := range p.links {
		n += len(target) + len(link.routePath) + 64
	}
	return n
}

// renderCache keeps rendered pages up to maxRenderCacheBytes and drops the
// least recently used first.
type renderCache struct {
	mu      sync.Mutex
	entries map[renderKey]*list.Element // values are *cacheEntry
	order   list.List                   // front: least recently used
	size    int
}

type cacheEntry struct {
	key  renderKey
	page *renderedPage
}

func (c *renderCache) get(key renderKey) *renderedPage {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.entries[key]
	if !ok {
		return nil
	}
	c.order.MoveToBack(el)
	return el.Value.(*cacheEntry).page
}

func (c *renderCache) put(key renderKey, page *renderedPage) {
	cost := page.cost()
	if cost > maxRenderCacheBytes {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		entry := el.Value.(*cacheEntry)
		c.size -= entry.page.cost()
		entry.page = page
		c.order.MoveToBack(el)
	} else {
		c.entries[key] = c.order.PushBack(&cacheEntry{key: key, page: page})
	}
	c.size += cost
	for c.size > maxRenderCacheBytes {
		oldest := c.order.Remove(c.order.Front()).(*cacheEntry)
		delete(c.entries, oldest.key)
		c.size -= oldest.page.cost()
	}
}
