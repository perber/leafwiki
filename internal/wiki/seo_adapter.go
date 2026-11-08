package wiki

import (
	"context"
	"errors"

	"github.com/perber/wiki/internal/core/tree"
	httpinternal "github.com/perber/wiki/internal/http"
	wikipages "github.com/perber/wiki/internal/wiki/pages"
	"github.com/perber/wiki/internal/wiki/seo"
)

// seoShellResponders builds the seo slice's shell responders (ADR-0018), all
// gated on public access by one seo.PublicOnly.
func (w *Wiki) seoShellResponders() []httpinternal.ShellResponder {
	public := seo.NewPublicOnly(w.publicAccess)
	log := w.log.With("component", "SEO")
	rootTarget := seo.NewGetRootTargetUseCase(seoFirstPageReader{uc: wikipages.NewGetFirstPagePathUseCase(w.tree)})
	pageMarkdown := seo.NewGetPageMarkdownUseCase(seoPageReader{uc: wikipages.NewGetPageViewByPathUseCase(w.tree)})
	return []httpinternal.ShellResponder{
		// The root redirect comes first: "/" is a 302 for Markdown clients too.
		public.Responder(seo.NewRootRedirect(rootTarget, log)),
		public.Responder(seo.NewPageMarkdown(pageMarkdown, log)),
	}
}

// seoShellContributors builds the seo slice's shell contributors (ADR-0018),
// all gated on public access by one seo.PublicOnly.
func (w *Wiki) seoShellContributors() []httpinternal.ShellContributor {
	public := seo.NewPublicOnly(w.publicAccess)
	pageMeta := seo.NewGetPageMetaUseCase(
		seoPageReader{uc: wikipages.NewGetPageViewByPathUseCase(w.tree)},
		seoPermalinkResolver{queries: wikipages.NewPageQueries(w.tree)},
		w.siteName, w.publicURL,
	)
	return []httpinternal.ShellContributor{
		public.Contributor(seo.NewPageMeta(pageMeta, w.log.With("component", "SEO"))),
	}
}

// seoRoutes builds the seo slice's own routes (robots.txt, sitemap.xml),
// gated on public access by one seo.PublicOnly.
func (w *Wiki) seoRoutes() *seo.Routes {
	return seo.NewRoutes(seo.RoutesConfig{
		Public:  seo.NewPublicOnly(w.publicAccess),
		Robots:  seo.NewGetRobotsUseCase(w.publicURL),
		Sitemap: seo.NewBuildSitemapUseCase(seoPageRouteLister{uc: wikipages.NewListPageRoutesUseCase(w.tree)}, w.publicURL),
		Log:     w.log.With("component", "SEO"),
	})
}

// seoFirstPageReader implements seo.FirstPageReader with the pages slice's
// read use case; seo must not import pages.
type seoFirstPageReader struct {
	uc *wikipages.GetFirstPagePathUseCase
}

func (r seoFirstPageReader) FirstPage(ctx context.Context) (*seo.FirstPage, error) {
	out, err := r.uc.Execute(ctx, wikipages.GetFirstPagePathInput{})
	if errors.Is(err, tree.ErrPageNotFound) || errors.Is(err, tree.ErrTreeNotLoaded) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &seo.FirstPage{RoutePath: out.RoutePath}, nil
}

// seoPageReader implements seo.PageReader with the pages slice's read use
// case; seo must not import pages.
type seoPageReader struct {
	uc *wikipages.GetPageViewByPathUseCase
}

func (r seoPageReader) PageByRoutePath(ctx context.Context, routePath string) (*seo.Page, error) {
	view, err := r.uc.Execute(ctx, wikipages.GetPageViewByPathInput{RoutePath: routePath})
	if errors.Is(err, tree.ErrPageNotFound) || errors.Is(err, tree.ErrTreeNotLoaded) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &seo.Page{Title: view.Page.Title, Body: view.Page.Content, Properties: view.Properties}, nil
}

// seoPermalinkResolver implements seo.PermalinkResolver with the pages
// slice's permalink lookup; seo must not import pages.
type seoPermalinkResolver struct {
	queries *wikipages.PageQueries
}

func (r seoPermalinkResolver) ResolvePermalink(_ context.Context, id string) (*seo.PermalinkTarget, error) {
	target, err := r.queries.ResolvePermalink(id)
	if errors.Is(err, tree.ErrPageNotFound) || errors.Is(err, tree.ErrTreeNotLoaded) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &seo.PermalinkTarget{RoutePath: target.Path}, nil
}

// seoPageRouteLister implements seo.PageRouteLister with the pages slice's
// read use case; seo must not import pages. A public wiki shows every page to
// anonymous readers, so every page and section is listed (ADR-0018).
type seoPageRouteLister struct {
	uc *wikipages.ListPageRoutesUseCase
}

func (l seoPageRouteLister) ListPageRoutes(ctx context.Context, limit int) ([]seo.PageRoute, bool, error) {
	out, err := l.uc.Execute(ctx, wikipages.ListPageRoutesInput{Limit: limit})
	if err != nil {
		return nil, false, err
	}
	routes := make([]seo.PageRoute, len(out.Routes))
	for i, r := range out.Routes {
		routes[i] = seo.PageRoute{RoutePath: r.RoutePath, UpdatedAt: r.UpdatedAt}
	}
	return routes, out.Truncated, nil
}
