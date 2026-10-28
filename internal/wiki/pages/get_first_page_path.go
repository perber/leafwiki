package pages

import (
	"context"

	"github.com/perber/wiki/internal/core/tree"
)

// GetFirstPagePathInput is the input for GetFirstPagePathUseCase.
type GetFirstPagePathInput struct{}

// GetFirstPagePathOutput is the result of GetFirstPagePathUseCase.
type GetFirstPagePathOutput struct {
	// RoutePath is the URL route path of the page (e.g. "docs"), without a
	// leading slash.
	RoutePath string
}

// GetFirstPagePathUseCase finds the page the wiki opens at its root: the first
// top-level page or section in navigation order, as the SPA's root redirect
// does. Like the page-view reads, it is not a visibility layer.
type GetFirstPagePathUseCase struct {
	tree *tree.TreeService
}

// NewGetFirstPagePathUseCase constructs a GetFirstPagePathUseCase.
func NewGetFirstPagePathUseCase(t *tree.TreeService) *GetFirstPagePathUseCase {
	return &GetFirstPagePathUseCase{tree: t}
}

// Execute returns the first page's route path; tree.ErrPageNotFound (empty
// tree) and tree.ErrTreeNotLoaded are passed through.
func (uc *GetFirstPagePathUseCase) Execute(_ context.Context, _ GetFirstPagePathInput) (*GetFirstPagePathOutput, error) {
	routePath, err := uc.tree.FirstRootChildPath()
	if err != nil {
		return nil, err
	}
	return &GetFirstPagePathOutput{RoutePath: routePath}, nil
}
