package pages

import (
	"context"

	"github.com/perber/wiki/internal/core/tree"
)

// ListPageRoutesInput is the input for ListPageRoutesUseCase.
type ListPageRoutesInput struct {
	// Limit caps the number of routes; 0 means no limit.
	Limit int
}

// ListPageRoutesOutput is the result of ListPageRoutesUseCase.
type ListPageRoutesOutput struct {
	// Routes are all pages and sections, depth-first in navigation order.
	Routes []tree.RouteEntry
	// Truncated reports that the wiki has more routes than Limit.
	Truncated bool
}

// ListPageRoutesUseCase lists the route of every page and section, e.g. for
// the sitemap. Like the page-view reads, it is not a visibility layer.
type ListPageRoutesUseCase struct {
	tree *tree.TreeService
}

// NewListPageRoutesUseCase constructs a ListPageRoutesUseCase.
func NewListPageRoutesUseCase(t *tree.TreeService) *ListPageRoutesUseCase {
	return &ListPageRoutesUseCase{tree: t}
}

// Execute returns the routes; tree.ErrTreeNotLoaded is passed through.
func (uc *ListPageRoutesUseCase) Execute(_ context.Context, in ListPageRoutesInput) (*ListPageRoutesOutput, error) {
	routes, truncated, err := uc.tree.PageRoutes(in.Limit)
	if err != nil {
		return nil, err
	}
	return &ListPageRoutesOutput{Routes: routes, Truncated: truncated}, nil
}
