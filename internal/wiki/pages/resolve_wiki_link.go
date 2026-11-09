package pages

import (
	"context"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// ResolveWikiLinkInput is the input for ResolveWikiLinkUseCase.
type ResolveWikiLinkInput struct {
	// Target is the text inside [[...]] before an alias.
	Target string
}

// ResolveWikiLinkOutput is the page a wikilink points at.
type ResolveWikiLinkOutput struct {
	// RoutePath starts with "/" (e.g. "/docs/setup").
	RoutePath string
}

// ResolveWikiLinkUseCase resolves a [[target]] to the one page it points at,
// by the rule stored links follow, e.g. for the crawler body. Like the
// page-view reads, it is not a visibility layer.
type ResolveWikiLinkUseCase struct {
	tree *tree.TreeService
}

// NewResolveWikiLinkUseCase constructs a ResolveWikiLinkUseCase.
func NewResolveWikiLinkUseCase(t *tree.TreeService) *ResolveWikiLinkUseCase {
	return &ResolveWikiLinkUseCase{tree: t}
}

// Execute returns the target page's route, or nil for an unknown or
// ambiguous target and before the tree is loaded.
func (uc *ResolveWikiLinkUseCase) Execute(_ context.Context, in ResolveWikiLinkInput) (*ResolveWikiLinkOutput, error) {
	routePath, ok := links.ResolveWikiLinkTarget(uc.tree, in.Target)
	if !ok {
		return nil, nil
	}
	return &ResolveWikiLinkOutput{RoutePath: routePath}, nil
}
