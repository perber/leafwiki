package pages

import (
	"context"
	"log/slog"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pagesave"
)

// CreatePageInput is the input for CreatePageUseCase.
type CreatePageInput struct {
	UserID   string
	ParentID *string
	Title    string
	Slug     string
	Kind     *tree.NodeKind
}

// CreatePageOutput is the output of CreatePageUseCase.
type CreatePageOutput struct {
	Page *tree.Page
}

// CreatePageUseCase creates a new page in the tree and fires post-save side effects.
type CreatePageUseCase struct {
	tree        *tree.TreeService
	slug        *tree.SlugService
	orchestrator *pagesave.PageSaveOrchestrator
	log         *slog.Logger
}

// NewCreatePageUseCase constructs a CreatePageUseCase.
func NewCreatePageUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	o *pagesave.PageSaveOrchestrator,
	log *slog.Logger,
) *CreatePageUseCase {
	return &CreatePageUseCase{tree: t, slug: s, orchestrator: o, log: log}
}

// Execute validates input, creates the page node, and fires post-save side effects.
func (uc *CreatePageUseCase) Execute(_ context.Context, in CreatePageInput) (*CreatePageOutput, error) {
	ve := sharederrors.NewValidationErrors()

	if in.Title == "" {
		ve.Add("title", "Title must not be empty")
	}
	if in.Kind == nil {
		ve.Add("kind", "Kind must be specified")
	}
	if in.Kind != nil && *in.Kind != tree.NodeKindPage && *in.Kind != tree.NodeKindSection {
		ve.Add("kind", "Kind must be either 'page' or 'section'")
	}
	if err := uc.slug.IsValidSlug(in.Slug); err != nil {
		ve.Add("slug", err.Error())
	}
	if ve.HasErrors() {
		return nil, ve
	}

	if in.ParentID != nil && *in.ParentID != "" {
		if _, err := uc.tree.FindPageByID(*in.ParentID); err != nil {
			return nil, err
		}
	}

	id, err := uc.tree.CreateNode(in.UserID, in.ParentID, in.Title, in.Slug, in.Kind)
	if err != nil {
		return nil, err
	}

	page, err := uc.tree.GetPage(*id)
	if err != nil {
		return nil, err
	}

	uc.orchestrator.Run(pagesave.PageSaveEvent{
		Operation: pagesave.PageOperationCreate,
		UserID:    in.UserID,
		After:     page,
		Summary:   "page created",
	})

	return &CreatePageOutput{Page: page}, nil
}
