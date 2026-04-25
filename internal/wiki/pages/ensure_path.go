package pages

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pagesave"
)

// EnsurePathInput is the input for EnsurePathUseCase.
type EnsurePathInput struct {
	UserID      string
	TargetPath  string
	TargetTitle string
	Kind        *tree.NodeKind
}

// EnsurePathOutput is the output of EnsurePathUseCase.
type EnsurePathOutput struct {
	Page *tree.Page
}

// EnsurePathUseCase ensures a full path exists, creating intermediate nodes as needed.
type EnsurePathUseCase struct {
	tree         *tree.TreeService
	slug         *tree.SlugService
	orchestrator *pagesave.PageSaveOrchestrator
	log          *slog.Logger
}

// NewEnsurePathUseCase constructs an EnsurePathUseCase.
func NewEnsurePathUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	o *pagesave.PageSaveOrchestrator,
	log *slog.Logger,
) *EnsurePathUseCase {
	return &EnsurePathUseCase{tree: t, slug: s, orchestrator: o, log: log}
}

// Execute ensures the path exists and returns the final node.
func (uc *EnsurePathUseCase) Execute(_ context.Context, in EnsurePathInput) (*EnsurePathOutput, error) {
	ve := sharederrors.NewValidationErrors()

	cleanPath := strings.Trim(strings.TrimSpace(in.TargetPath), "/")
	if cleanPath == "" {
		ve.Add("path", "Path must not be empty")
	}

	cleanTitle := strings.TrimSpace(in.TargetTitle)
	if cleanTitle == "" {
		ve.Add("title", "Title must not be empty")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	lookup, err := uc.tree.LookupPagePath(cleanPath)
	if err != nil {
		return nil, err
	}

	if lookup.Exists {
		page, err := uc.tree.GetPage(*lookup.Segments[len(lookup.Segments)-1].ID)
		if err != nil {
			return nil, err
		}
		return &EnsurePathOutput{Page: page}, nil
	}

	for _, segment := range lookup.Segments {
		if !segment.Exists {
			if err := uc.slug.IsValidSlug(segment.Slug); err != nil {
				ve.Add("path", fmt.Sprintf("Invalid slug '%s': %s", segment.Slug, err.Error()))
			}
		}
	}
	if ve.HasErrors() {
		return nil, ve
	}

	result, err := uc.tree.EnsurePagePath(in.UserID, cleanPath, cleanTitle, in.Kind)
	if err != nil {
		return nil, err
	}

	page, err := uc.tree.GetPage(result.Page.ID)
	if err != nil {
		return nil, err
	}

	for _, n := range result.Created {
		p, err := uc.tree.GetPage(n.ID)
		if err != nil {
			uc.log.Warn("failed to get page for post-create processing", "pageID", n.ID, "error", err)
			continue
		}
		uc.orchestrator.Run(pagesave.PageSaveEvent{
			Operation: pagesave.PageOperationCreate,
			UserID:    in.UserID,
			After:     p,
			Summary:   "page created via ensure path",
		})
	}

	return &EnsurePathOutput{Page: page}, nil
}
