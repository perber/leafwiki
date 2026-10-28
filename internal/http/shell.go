package http

import (
	"context"
	"html"
	"html/template"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// The SPA shell is the HTML document served for every GET that no route
// matched (deep links into the SPA). Features shape it through two seams
// registered on FrontendConfig, see ADR-0017:
//
//   - ShellResponder answers the request itself (e.g. a redirect or another
//     representation) instead of the shell.
//   - ShellContributor adds a title, <head> markup and markup inside #root.
//
// Contract for both:
//   - Requests are always anonymous. The fallback route runs outside the base
//     route group, so remote-user and API-key authentication are never
//     resolved. Expose only publicly readable content and decline cheaply when
//     public access is off. Read pages through the same read use cases as the
//     API, never from core/tree directly.
//   - They run on every SPA GET, so keep them cheap.
//   - Never panic. Handle and log your own errors; on error return empty
//     parts (contributor) or false (responder).
//   - A responder whose decision depends on a request header adds the matching
//     Vary header itself, also when it declines. A contributor has no writer;
//     the first one whose parts depend on a header adds a Vary field to
//     ShellParts.
//   - A contributor that adds page-specific content sets ShellParts.NoCache,
//     so caches revalidate the shell instead of serving a stale page title.
//   - Registration order is the composition root's responsibility. The custom
//     stylesheet and the admin's InjectCodeInHeader are built-in contributors
//     that always come last, so admin code can override everything else.
//   - Status codes other than 200 for the shell (404/410) belong in ShellParts
//     once needed, not in a responder that re-renders the shell.

// ShellRequest describes the request the shell is rendered for.
type ShellRequest struct {
	Request *http.Request
	// RoutePath is the request path with BasePath stripped; it always starts
	// with "/". It is decoded but not cleaned ("..", "//" and a decoded %2F pass
	// through), so resolve it through the route lookup, never as a file path.
	RoutePath string
	BasePath  string
}

// ShellParts is what a contributor adds to the shell.
type ShellParts struct {
	// Title is plain text; the router escapes it. The first non-empty title in
	// registration order wins, otherwise the site name is used.
	Title string
	// Head is trusted markup appended to <head>; the contributor escapes it
	// (see MetaTag, MetaProperty, LinkTag).
	Head template.HTML
	// Body is trusted markup placed inside #root; the contributor escapes it.
	Body template.HTML
	// NoCache makes the shell response carry "Cache-Control: no-cache" when
	// any contributor sets it. Set it whenever the parts depend on the page
	// content, so a cache never serves an outdated title or description.
	NoCache bool
}

// ShellResponder may answer a shell request itself. It returns true when it
// wrote the response; then no later responder runs and no shell is rendered.
// When it returns false it may set headers only, never the status or a body,
// because the shell or the 404 is written after it.
type ShellResponder interface {
	RespondShell(w http.ResponseWriter, req ShellRequest) (handled bool)
}

// ShellResponderFunc adapts a function to ShellResponder.
type ShellResponderFunc func(w http.ResponseWriter, req ShellRequest) bool

func (f ShellResponderFunc) RespondShell(w http.ResponseWriter, req ShellRequest) bool {
	return f(w, req)
}

// ShellContributor adds parts to the rendered shell.
type ShellContributor interface {
	ContributeShell(ctx context.Context, req ShellRequest) ShellParts
}

// ShellContributorFunc adapts a function to ShellContributor.
type ShellContributorFunc func(ctx context.Context, req ShellRequest) ShellParts

func (f ShellContributorFunc) ContributeShell(ctx context.Context, req ShellRequest) ShellParts {
	return f(ctx, req)
}

// MetaTag returns <meta name=… content=…> with both attributes escaped.
func MetaTag(name, content string) template.HTML {
	return template.HTML(`<meta name="` + html.EscapeString(name) + `" content="` + html.EscapeString(content) + `">`)
}

// MetaProperty returns <meta property=… content=…> (Open Graph) with both attributes escaped.
func MetaProperty(property, content string) template.HTML {
	return template.HTML(`<meta property="` + html.EscapeString(property) + `" content="` + html.EscapeString(content) + `">`)
}

// LinkTag returns <link rel=… href=…> with both attributes escaped. Escaping
// only prevents breaking out of the attribute; the caller builds href.
func LinkTag(rel, href string) template.HTML {
	return template.HTML(`<link rel="` + html.EscapeString(rel) + `" href="` + html.EscapeString(href) + `">`)
}

// shellHandler serves the SPA fallback route.
type shellHandler struct {
	basePath       string
	getSiteName    func() string
	getFaviconFile func() string
	responders     []ShellResponder
	contributors   []ShellContributor // registered ones, then the built-ins
	embedded       bool               // index.html is served; without it gin's default 404 stays
	index          string
	indexErr       error
}

func newShellHandler(cfg FrontendConfig, opts RouterOptions, customStylesheetPath string) *shellHandler {
	contributors := append([]ShellContributor{}, cfg.ShellContributors...)
	contributors = append(contributors,
		headContributor(template.HTML(buildCustomStylesheetTag(opts.BasePath, customStylesheetPath))), // basePath unescaped here, behaviour predates the seam
		headContributor(template.HTML(opts.InjectCodeInHeader)),                                       // admin-provided header code is trusted by design
	)
	return &shellHandler{
		basePath:       opts.BasePath,
		getSiteName:    cfg.GetSiteName,
		getFaviconFile: cfg.GetFaviconFile,
		responders:     cfg.ShellResponders,
		contributors:   contributors,
	}
}

func headContributor(head template.HTML) ShellContributor {
	return ShellContributorFunc(func(context.Context, ShellRequest) ShellParts { return ShellParts{Head: head} })
}

// setIndex enables serving the shell from the embedded index.html.
func (h *shellHandler) setIndex(index []byte, err error) {
	h.embedded = true
	h.index = string(index)
	h.indexErr = err
}

func (h *shellHandler) serve(c *gin.Context) {
	routePath, ok := shellRoutePath(c.Request.URL.Path, h.basePath)
	if !ok || !isShellRequest(c.Request.Method, routePath) {
		if h.embedded {
			c.String(http.StatusNotFound, "Page not found")
		}
		return
	}
	req := ShellRequest{Request: c.Request, RoutePath: routePath, BasePath: h.basePath}

	// gin presets 404 on the fallback route; give responders stdlib semantics
	// (Write without WriteHeader means 200) and restore it when all decline.
	c.Status(http.StatusOK)
	for _, r := range h.responders {
		if r.RespondShell(c.Writer, req) {
			return
		}
	}
	c.Status(http.StatusNotFound)

	if !h.embedded {
		return
	}
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if h.indexErr != nil {
		return
	}
	doc, noCache := h.render(req)
	if noCache {
		c.Writer.Header().Set("Cache-Control", "no-cache")
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(doc))
}

// render returns the shell document and whether a contributor asked for
// Cache-Control: no-cache.
func (h *shellHandler) render(req ShellRequest) (doc string, noCache bool) {
	ctx := req.Request.Context()
	var title string
	var body strings.Builder
	var heads []template.HTML
	for _, contributor := range h.contributors {
		parts := contributor.ContributeShell(ctx, req)
		noCache = noCache || parts.NoCache
		if title == "" {
			title = parts.Title
		}
		body.WriteString(string(parts.Body))
		if parts.Head != "" {
			heads = append(heads, parts.Head)
		}
	}

	siteName := "LeafWiki"
	if h.getSiteName != nil {
		if name := h.getSiteName(); name != "" {
			siteName = name
		}
	}
	if title == "" {
		title = siteName
	}
	faviconFile := ""
	if h.getFaviconFile != nil {
		faviconFile = h.getFaviconFile()
	}

	// One pass, so contributed text is never scanned for placeholders.
	escapedBasePath := html.EscapeString(h.basePath)
	doc = strings.NewReplacer(
		"{{__PAGE_TITLE__}}", html.EscapeString(title),
		"{{__SITE_NAME__}}", html.EscapeString(siteName),
		"{{__BASE_PATH__}}", escapedBasePath,
		"{{__FAVICON_HREF__}}", html.EscapeString(BuildFrontendFaviconHref(h.basePath, faviconFile)),
		"{{__BODY__}}", body.String(),
		// Rewrite Vite's relative "./static/" asset references to absolute paths
		// so they resolve correctly when index.html is served for deep SPA routes.
		// Lazy chunks still use import.meta.url for their own path resolution.
		`"./static/`, `"`+escapedBasePath+`/static/`,
	).Replace(h.index)

	// One injection for all head parts: a literal </head> inside an earlier
	// part must not become the target for later ones. The separator keeps the
	// indentation that injecting part by part produced.
	parts := make([]string, 0, len(heads))
	for _, head := range heads {
		if strings.TrimSpace(string(head)) != "" {
			parts = append(parts, string(head))
		}
	}
	return injectIntoHead(doc, strings.Join(parts, "\n    ")), noCache
}

// shellRoutePath strips basePath from urlPath; ok is false outside basePath.
func shellRoutePath(urlPath, basePath string) (routePath string, ok bool) {
	if basePath == "" {
		return urlPath, true
	}
	if urlPath != basePath && !strings.HasPrefix(urlPath, basePath+"/") {
		return "", false
	}
	routePath = strings.TrimPrefix(urlPath, basePath)
	if routePath == "" {
		routePath = "/"
	}
	return routePath, true
}

func isShellRequest(method, routePath string) bool {
	return method == http.MethodGet &&
		!strings.HasPrefix(routePath, "/api") &&
		!strings.HasPrefix(routePath, "/assets") &&
		!strings.HasPrefix(routePath, "/static") &&
		!strings.HasPrefix(routePath, "/branding")
}
