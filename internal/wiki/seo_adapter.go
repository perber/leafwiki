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
	rootTarget := seo.NewGetRootTargetUseCase(seoFirstPageReader{uc: wikipages.NewGetFirstPagePathUseCase(w.tree)})
	return []httpinternal.ShellResponder{
		public.Responder(seo.NewRootRedirect(rootTarget, w.log.With("component", "SEO"))),
	}
}

// seoShellContributors builds the seo slice's shell contributors (ADR-0018),
// all gated on public access by one seo.PublicOnly.
func (w *Wiki) seoShellContributors() []httpinternal.ShellContributor {
	public := seo.NewPublicOnly(w.publicAccess)
	pageMeta := seo.NewGetPageMetaUseCase(seoPageReader{uc: wikipages.NewGetPageViewByPathUseCase(w.tree)}, w.siteName, w.publicURL)
	return []httpinternal.ShellContributor{
		public.Contributor(seo.NewPageMeta(pageMeta, w.log.With("component", "SEO"))),
	}
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
