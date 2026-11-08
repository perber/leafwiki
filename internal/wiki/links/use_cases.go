package links

import (
	"context"
	"errors"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	corelinks "github.com/perber/wiki/internal/links"
)

var ErrLinkServiceUnavailable = sharederrors.NewLocalizedError(
	ErrCodeLinkUnavailable,
	"Link service is unavailable",
	"link service is unavailable",
	nil,
)

// ─── GetLinkStatusUseCase ────────────────────────────────────────────────────

type GetLinkStatusInput struct {
	PageID string
}

type GetLinkStatusOutput struct {
	Status *corelinks.LinkStatusResult
}

type GetLinkStatusUseCase struct {
	links *corelinks.LinkService
	tree  *tree.TreeService
}

func NewGetLinkStatusUseCase(l *corelinks.LinkService, t *tree.TreeService) *GetLinkStatusUseCase {
	return &GetLinkStatusUseCase{links: l, tree: t}
}

func (uc *GetLinkStatusUseCase) Execute(_ context.Context, in GetLinkStatusInput) (*GetLinkStatusOutput, error) {
	if uc.links == nil {
		return nil, ErrLinkServiceUnavailable
	}
	page, err := uc.tree.GetPage(in.PageID)
	if err != nil {
		if errors.Is(err, tree.ErrPageNotFound) {
			return nil, sharederrors.NewLocalizedError(
				ErrCodeLinkPageNotFound,
				"Page not found",
				"page not found",
				err,
			)
		}
		return nil, err
	}
	status, err := uc.links.GetLinkStatusForPage(in.PageID, page.CalculatePath())
	if err != nil {
		return nil, err
	}
	return &GetLinkStatusOutput{Status: status}, nil
}

type GetBrokenLinksOutput struct {
	Links []corelinks.BrokenLink
}

type GetBrokenLinksUseCase struct {
	links *corelinks.LinkService
}

func NewGetBrokenLinksUseCase(
	l *corelinks.LinkService,
) *GetBrokenLinksUseCase {
	return &GetBrokenLinksUseCase{
		links: l,
	}
}

func (uc *GetBrokenLinksUseCase) Execute(
	_ context.Context,
) (*GetBrokenLinksOutput, error) {
	if uc.links == nil {
		return nil, ErrLinkServiceUnavailable
	}

	links, err := uc.links.GetBrokenLinks()
	if err != nil {
		return nil, err
	}

	if links == nil {
		links = []corelinks.BrokenLink{}
	}

	return &GetBrokenLinksOutput{
		Links: links,
	}, nil
}
