package pages

import (
	"context"
	"log"
	"log/slog"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/links"
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

// CreatePageUseCase creates a new page in the tree and records the initial revision.
// It will later serve as a transaction boundary.
type CreatePageUseCase struct {
	tree     *tree.TreeService
	slug     *tree.SlugService
	revision *revision.Service
	links    *links.LinkService
	log      *slog.Logger
}

// NewCreatePageUseCase constructs a CreatePageUseCase.
func NewCreatePageUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	r *revision.Service,
	l *links.LinkService,
	log *slog.Logger,
) *CreatePageUseCase {
	return &CreatePageUseCase{tree: t, slug: s, revision: r, links: l, log: log}
}

// Execute validates input, creates the page node, heals links, and records a revision.
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

	if uc.links != nil {
		if err := uc.links.HealLinksForExactPath(page); err != nil {
			log.Printf("warning: failed to heal links for page %s: %v", page.ID, err)
		}
	}

	// Fix: CreatePage was missing revision recording in the original wiki.go implementation.
	// Every other mutating operation (UpdatePage, CopyPage, MovePage) records a revision,
	// so the absence here was a bug. Recording "page created" gives the revision history
	// a baseline snapshot and makes RestorePage behaviour consistent.
	if uc.revision != nil {
		recordContentRevision(uc.revision, uc.log, page.ID, in.UserID, "page created")
	}

	return &CreatePageOutput{Page: page}, nil
}
