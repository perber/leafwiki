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
