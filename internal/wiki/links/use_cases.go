package links

import (
	"context"
	"errors"

	corelinks "github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/core/tree"
)

var ErrLinkServiceUnavailable = errors.New("link service not available")

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
		return nil, err
	}
	status, err := uc.links.GetLinkStatusForPage(in.PageID, page.CalculatePath())
	if err != nil {
		return nil, err
	}
	return &GetLinkStatusOutput{Status: status}, nil
}

// ─── GetBacklinksUseCase ─────────────────────────────────────────────────────

type GetBacklinksInput struct {
	PageID string
}

type GetBacklinksOutput struct {
	Result *corelinks.BacklinkResult
}

type GetBacklinksUseCase struct {
	links *corelinks.LinkService
}

func NewGetBacklinksUseCase(l *corelinks.LinkService) *GetBacklinksUseCase {
	return &GetBacklinksUseCase{links: l}
}

func (uc *GetBacklinksUseCase) Execute(_ context.Context, in GetBacklinksInput) (*GetBacklinksOutput, error) {
	if uc.links == nil {
		return nil, ErrLinkServiceUnavailable
	}
	result, err := uc.links.GetBacklinksForPage(in.PageID)
	if err != nil {
		return nil, err
	}
	return &GetBacklinksOutput{Result: result}, nil
}

// ─── GetOutgoingLinksUseCase ─────────────────────────────────────────────────

type GetOutgoingLinksInput struct {
	PageID string
}

type GetOutgoingLinksOutput struct {
	Result *corelinks.OutgoingResult
}

type GetOutgoingLinksUseCase struct {
	links *corelinks.LinkService
}

func NewGetOutgoingLinksUseCase(l *corelinks.LinkService) *GetOutgoingLinksUseCase {
	return &GetOutgoingLinksUseCase{links: l}
}

func (uc *GetOutgoingLinksUseCase) Execute(_ context.Context, in GetOutgoingLinksInput) (*GetOutgoingLinksOutput, error) {
	if uc.links == nil {
		return nil, ErrLinkServiceUnavailable
	}
	result, err := uc.links.GetOutgoingLinksForPage(in.PageID)
	if err != nil {
		return nil, err
	}
	return &GetOutgoingLinksOutput{Result: result}, nil
}

// ─── ReindexLinksUseCase ─────────────────────────────────────────────────────

type ReindexLinksUseCase struct {
	links *corelinks.LinkService
}

func NewReindexLinksUseCase(l *corelinks.LinkService) *ReindexLinksUseCase {
	return &ReindexLinksUseCase{links: l}
}

func (uc *ReindexLinksUseCase) Execute(_ context.Context) error {
	if uc.links == nil {
		return nil
	}
	return uc.links.IndexAllPages()
}
