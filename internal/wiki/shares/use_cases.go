package shares

import (
	"context"
	"errors"
	"fmt"

	"github.com/perber/wiki/internal/scopedread"
	"github.com/perber/wiki/internal/sharing"
)

// linkResolver turns a share token into what it grants. *sharing.ShareLinkService
// implements it; the token never goes further than this use case.
type linkResolver interface {
	Resolve(ctx context.Context, token string) (*sharing.Grant, error)
}

// scopedPageReader reads one page inside a scope. *scopedread.PageReader
// implements it.
type scopedPageReader interface {
	Page(ctx context.Context, scope scopedread.ReadScope, id string) (*scopedread.PageView, error)
}

// ─── GetSharedPageUseCase ────────────────────────────────────────────────────

// GetSharedPageInput is the input for GetSharedPageUseCase.
type GetSharedPageInput struct {
	Token  string
	PageID string // empty = the shared page itself
}

// GetSharedPageOutput is the output of GetSharedPageUseCase.
type GetSharedPageOutput struct {
	Page *scopedread.PageView
}

// GetSharedPageUseCase reads a page through a share link: the token resolves
// to a grant, the grant becomes the read scope (ADR-0015), and the page is
// read through that scope only.
type GetSharedPageUseCase struct {
	links linkResolver
	pages scopedPageReader
}

// NewGetSharedPageUseCase constructs a GetSharedPageUseCase.
func NewGetSharedPageUseCase(links linkResolver, pages scopedPageReader) *GetSharedPageUseCase {
	return &GetSharedPageUseCase{links: links, pages: pages}
}

// Execute returns the page, or share_link_not_found for an unknown, expired or
// revoked link and for a page outside the link's scope alike.
func (uc *GetSharedPageUseCase) Execute(ctx context.Context, in GetSharedPageInput) (*GetSharedPageOutput, error) {
	grant, err := uc.links.Resolve(ctx, in.Token)
	if err != nil {
		return nil, err
	}
	id := in.PageID
	if id == "" {
		id = grant.PageID
	}
	scope := scopedread.ReadScope{RootID: grant.PageID, IncludeDescendants: grant.IncludeDescendants}
	page, err := uc.pages.Page(ctx, scope, id)
	if errors.Is(err, scopedread.ErrNotFound) || errors.Is(err, scopedread.ErrInvalidScope) {
		return nil, errShareLinkNotFound()
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read shared page for share link %s: %w", grant.ShareID, err)
	}
	return &GetSharedPageOutput{Page: page}, nil
}
