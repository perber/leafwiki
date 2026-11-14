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

// scopedTreeReader reads the navigation inside a scope. *scopedread.TreeReader
// implements it.
type scopedTreeReader interface {
	Tree(ctx context.Context, scope scopedread.ReadScope) (*scopedread.TreeNodeView, error)
}

// scopedAssetReader opens a page's asset inside a scope.
// *scopedread.AssetReader implements it.
type scopedAssetReader interface {
	Asset(ctx context.Context, scope scopedread.ReadScope, pageID, filename string) (*scopedread.AssetFile, error)
}

// resolveScope resolves token to its grant and the read scope the grant
// allows (ADR-0015). Every shared read starts here.
func resolveScope(ctx context.Context, links linkResolver, token string) (*sharing.Grant, scopedread.ReadScope, error) {
	grant, err := links.Resolve(ctx, token)
	if err != nil {
		return nil, scopedread.ReadScope{}, err
	}
	return grant, scopedread.ReadScope{RootID: grant.PageID, IncludeDescendants: grant.IncludeDescendants}, nil
}

// scopedReadError maps the error of a read through grant: anything the scope
// hides is share_link_not_found, like an unknown link; operational errors are
// wrapped with the link ID for the server log.
func scopedReadError(grant *sharing.Grant, what string, err error) error {
	if errors.Is(err, scopedread.ErrNotFound) || errors.Is(err, scopedread.ErrInvalidScope) {
		return errShareLinkNotFound()
	}
	return fmt.Errorf("failed to read shared %s for share link %s: %w", what, grant.ShareID, err)
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
	grant, scope, err := resolveScope(ctx, uc.links, in.Token)
	if err != nil {
		return nil, err
	}
	id := in.PageID
	if id == "" {
		id = grant.PageID
	}
	page, err := uc.pages.Page(ctx, scope, id)
	if err != nil {
		return nil, scopedReadError(grant, "page", err)
	}
	return &GetSharedPageOutput{Page: page}, nil
}

// ─── GetSharedTreeUseCase ────────────────────────────────────────────────────

// GetSharedTreeInput is the input for GetSharedTreeUseCase.
type GetSharedTreeInput struct {
	Token string
}

// GetSharedTreeOutput is the output of GetSharedTreeUseCase.
type GetSharedTreeOutput struct {
	Tree *scopedread.TreeNodeView
}

// GetSharedTreeUseCase reads the navigation of a share link: the shared page
// on top and, for a link that includes subpages, everything below it.
type GetSharedTreeUseCase struct {
	links linkResolver
	trees scopedTreeReader
}

// NewGetSharedTreeUseCase constructs a GetSharedTreeUseCase.
func NewGetSharedTreeUseCase(links linkResolver, trees scopedTreeReader) *GetSharedTreeUseCase {
	return &GetSharedTreeUseCase{links: links, trees: trees}
}

// Execute returns the tree, or share_link_not_found for an unknown, expired
// or revoked link and for a link whose page was deleted.
func (uc *GetSharedTreeUseCase) Execute(ctx context.Context, in GetSharedTreeInput) (*GetSharedTreeOutput, error) {
	grant, scope, err := resolveScope(ctx, uc.links, in.Token)
	if err != nil {
		return nil, err
	}
	view, err := uc.trees.Tree(ctx, scope)
	if err != nil {
		return nil, scopedReadError(grant, "tree", err)
	}
	return &GetSharedTreeOutput{Tree: view}, nil
}

// ─── GetSharedAssetUseCase ───────────────────────────────────────────────────

// GetSharedAssetInput is the input for GetSharedAssetUseCase.
type GetSharedAssetInput struct {
	Token    string
	PageID   string
	Filename string
}

// GetSharedAssetOutput is the output of GetSharedAssetUseCase. The caller
// closes Asset.Content.
type GetSharedAssetOutput struct {
	Asset *scopedread.AssetFile
}

// GetSharedAssetUseCase opens an asset through a share link: an asset of the
// shared page or, for a link that includes subpages, of a page below it.
type GetSharedAssetUseCase struct {
	links  linkResolver
	assets scopedAssetReader
}

// NewGetSharedAssetUseCase constructs a GetSharedAssetUseCase.
func NewGetSharedAssetUseCase(links linkResolver, assets scopedAssetReader) *GetSharedAssetUseCase {
	return &GetSharedAssetUseCase{links: links, assets: assets}
}

// Execute returns the open asset, or share_link_not_found for an unknown,
// expired or revoked link, a page outside the link's scope and a missing
// file alike.
func (uc *GetSharedAssetUseCase) Execute(ctx context.Context, in GetSharedAssetInput) (*GetSharedAssetOutput, error) {
	grant, scope, err := resolveScope(ctx, uc.links, in.Token)
	if err != nil {
		return nil, err
	}
	asset, err := uc.assets.Asset(ctx, scope, in.PageID, in.Filename)
	if err != nil {
		return nil, scopedReadError(grant, "asset", err)
	}
	return &GetSharedAssetOutput{Asset: asset}, nil
}
