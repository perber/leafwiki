package seo

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"

	httpinternal "github.com/perber/wiki/internal/http"
	"github.com/perber/wiki/internal/publicaccess"
)

// PublicOnly gates this package's shell registrations and routes on public access. The
// setting is read per request, so toggling it at runtime takes effect
// immediately.
type PublicOnly struct {
	gate publicaccess.ReadGate
}

// NewPublicOnly returns the gate; a nil ReadGate counts as public access off.
func NewPublicOnly(gate publicaccess.ReadGate) PublicOnly {
	return PublicOnly{gate: gate}
}

// Responder wraps r so it runs only while public access is on. Otherwise the
// request is declined before r runs, so nothing is read.
func (p PublicOnly) Responder(r httpinternal.ShellResponder) httpinternal.ShellResponder {
	return httpinternal.ShellResponderFunc(func(w http.ResponseWriter, req httpinternal.ShellRequest) bool {
		return p.enabled() && r.RespondShell(w, req)
	})
}

// Contributor wraps c so it runs only while public access is on. Otherwise it
// contributes nothing and c does not run, so nothing is read.
func (p PublicOnly) Contributor(c httpinternal.ShellContributor) httpinternal.ShellContributor {
	return httpinternal.ShellContributorFunc(func(ctx context.Context, req httpinternal.ShellRequest) httpinternal.ShellParts {
		if !p.enabled() {
			return httpinternal.ShellParts{}
		}
		return c.ContributeShell(ctx, req)
	})
}

// Handler returns a handler for the slice's own routes that runs on while
// public access is on and off otherwise. off must not read pages.
func (p PublicOnly) Handler(on, off http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if p.enabled() {
			on(w, r)
			return
		}
		off(w, r)
	}
}

func (p PublicOnly) enabled() bool {
	return p.gate != nil && p.gate.Enabled()
}

// rootRedirect answers the wiki root with a 302 to the first page, so search
// engines find content at "/" instead of an empty SPA shell.
type rootRedirect struct {
	target *GetRootTargetUseCase
	log    *slog.Logger
}

// NewRootRedirect returns the shell responder for the wiki root.
func NewRootRedirect(target *GetRootTargetUseCase, log *slog.Logger) httpinternal.ShellResponder {
	return &rootRedirect{target: target, log: log}
}

func (r *rootRedirect) RespondShell(w http.ResponseWriter, req httpinternal.ShellRequest) bool {
	if req.RoutePath != "/" {
		return false
	}
	out, err := r.target.Execute(req.Request.Context(), GetRootTargetInput{BasePath: req.BasePath})
	if err != nil {
		// Swallowed here: the visitor gets the shell, whose SPA redirects itself.
		r.log.Warn("root redirect skipped, serving the shell", "error", err)
		return false
	}
	if out.Location == "" {
		return false
	}
	w.Header().Set("Location", out.Location)
	w.WriteHeader(http.StatusFound)
	return true
}

// pageMarkdown answers a page route with the page's Markdown when the
// request's Accept header prefers text/markdown over text/html.
type pageMarkdown struct {
	markdown *GetPageMarkdownUseCase
	log      *slog.Logger
}

// NewPageMarkdown returns the shell responder for the Markdown representation
// of pages.
func NewPageMarkdown(markdown *GetPageMarkdownUseCase, log *slog.Logger) httpinternal.ShellResponder {
	return &pageMarkdown{markdown: markdown, log: log}
}

func (p *pageMarkdown) RespondShell(w http.ResponseWriter, req httpinternal.ShellRequest) bool {
	// The response depends on Accept whether this handles it or the shell is
	// served, so caches must keep the variants apart (ADR-0017 contract).
	httpinternal.AddVary(w.Header(), "Accept")
	out, err := p.markdown.Execute(req.Request.Context(), GetPageMarkdownInput{
		RoutePath: req.RoutePath,
		Accept:    strings.Join(req.Request.Header.Values("Accept"), ","),
	})
	if err != nil {
		// Swallowed here: the visitor gets the shell instead of the Markdown.
		p.log.Warn("page markdown skipped, serving the shell", "error", err)
		return false
	}
	if out == nil {
		return false
	}
	h := w.Header()
	h.Set("Content-Type", "text/markdown; charset=utf-8")
	// The body is raw user content; never let a browser sniff it as HTML.
	h.Set("X-Content-Type-Options", "nosniff")
	// Opened directly in a browser, the content runs no script and reaches no
	// storage of the wiki's origin, whatever a client does with the type.
	h.Set("Content-Security-Policy", "sandbox")
	// The content changes with every edit, and a variant must not outlive
	// public access being turned off.
	h.Set("Cache-Control", "no-cache")
	// Set explicitly so HEAD reports the length GET sends.
	h.Set("Content-Length", strconv.Itoa(len(out.Markdown)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, out.Markdown)
	return true
}

// pageMeta adds the page title, description, Open Graph tags and (with
// --public-url) the canonical link to the shell of a page route, so search
// engines and link previews see what the page is about. A route without a
// page gets only a robots noindex tag.
type pageMeta struct {
	meta *GetPageMetaUseCase
	log  *slog.Logger
}

// NewPageMeta returns the shell contributor for page meta tags.
func NewPageMeta(meta *GetPageMetaUseCase, log *slog.Logger) httpinternal.ShellContributor {
	return &pageMeta{meta: meta, log: log}
}

func (p *pageMeta) ContributeShell(ctx context.Context, req httpinternal.ShellRequest) httpinternal.ShellParts {
	out, err := p.meta.Execute(ctx, GetPageMetaInput{RoutePath: req.RoutePath, BasePath: req.BasePath})
	if err != nil {
		// Swallowed here: the visitor still gets the shell, only without page meta.
		p.log.Warn("page meta skipped, serving the plain shell", "error", err)
		return httpinternal.ShellParts{}
	}
	if out == nil {
		return httpinternal.ShellParts{}
	}
	if out.NoIndex {
		return httpinternal.ShellParts{
			Head: httpinternal.MetaTag("robots", "noindex"),
			// Whether a page exists there changes with every edit.
			NoCache: true,
		}
	}
	var head strings.Builder
	write := func(tag template.HTML) { head.WriteString(string(tag)) }
	if out.Description != "" {
		write(httpinternal.MetaTag("description", out.Description))
	}
	if out.CanonicalURL != "" {
		write(httpinternal.LinkTag("canonical", out.CanonicalURL))
	}
	write(httpinternal.MetaProperty("og:type", "website"))
	write(httpinternal.MetaProperty("og:title", out.PageTitle))
	if out.Description != "" {
		write(httpinternal.MetaProperty("og:description", out.Description))
	}
	write(httpinternal.MetaProperty("og:site_name", out.SiteName))
	if out.CanonicalURL != "" {
		write(httpinternal.MetaProperty("og:url", out.CanonicalURL))
	}
	return httpinternal.ShellParts{
		Title: out.Title,
		Head:  template.HTML(head.String()), // every tag is built by the escaping helpers above
		// The title and description change with the page; caches must revalidate.
		NoCache: true,
	}
}

// crawlerBody puts the rendered page into #root for search-engine crawlers,
// so they see the content without running the SPA (ADR-0018). The SPA's
// createRoot replaces it once it runs.
type crawlerBody struct {
	body       *GetCrawlerBodyUseCase
	log        *slog.Logger
	busyLogged atomic.Bool
}

// NewCrawlerBody returns the shell contributor for the crawler body.
func NewCrawlerBody(body *GetCrawlerBodyUseCase, log *slog.Logger) httpinternal.ShellContributor {
	return &crawlerBody{body: body, log: log}
}

func (c *crawlerBody) ContributeShell(ctx context.Context, req httpinternal.ShellRequest) httpinternal.ShellParts {
	// Whether the shell carries the page depends on User-Agent, on every
	// route: caches must keep the variants apart and revalidate them.
	parts := httpinternal.ShellParts{Vary: []string{"User-Agent"}, NoCache: true}
	out, err := c.body.Execute(ctx, GetCrawlerBodyInput{
		RoutePath: req.RoutePath,
		BasePath:  req.BasePath,
		UserAgent: req.Request.UserAgent(),
	})
	switch {
	case errors.Is(err, errRendersBusy):
		// Swallowed here: the crawler gets the plain shell. Logged once per
		// process, a crawl can hit this on every request.
		if !c.busyLogged.Swap(true) {
			c.log.Warn("crawler body skipped, too many renders running, serving the plain shell; further skips are not logged",
				"max_parallel_renders", maxParallelRenders)
		}
	case err != nil:
		// Swallowed here: the crawler gets the plain shell. A failed render
		// is cached, so this logs once per page version; the error carries
		// no page content.
		c.log.Warn("crawler body skipped, serving the plain shell", "route", req.RoutePath, "error", err)
	case out != nil:
		parts.Body = template.HTML(out.HTML) // escaped and sanitized by the use case
	}
	return parts
}
