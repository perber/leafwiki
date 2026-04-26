package pages

import (
	"context"
	"log/slog"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pagesave"
)

// UpdatePageInput is the input for UpdatePageUseCase.
type UpdatePageInput struct {
	UserID  string
	ID      string
	Version string
	Title   string
	Slug    string
	Content *string
	Kind    *tree.NodeKind
}

// UpdatePageOutput is the output of UpdatePageUseCase.
type UpdatePageOutput struct {
	Page *tree.Page
}

// UpdatePageUseCase updates an existing page's content and/or structure.
type UpdatePageUseCase struct {
	tree         *tree.TreeService
	slug         *tree.SlugService
	orchestrator *pagesave.PageSaveOrchestrator
	log          *slog.Logger
}

// NewUpdatePageUseCase constructs an UpdatePageUseCase.
func NewUpdatePageUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	o *pagesave.PageSaveOrchestrator,
	log *slog.Logger,
) *UpdatePageUseCase {
	return &UpdatePageUseCase{tree: t, slug: s, orchestrator: o, log: log}
}

// Execute validates, updates the node, and fires post-save side effects.
func (uc *UpdatePageUseCase) Execute(_ context.Context, in UpdatePageInput) (*UpdatePageOutput, error) {
	ve := sharederrors.NewValidationErrors()
	if in.Title == "" {
		ve.Add("title", "Title must not be empty")
	}
	if err := uc.slug.IsValidSlug(in.Slug); err != nil {
		ve.Add("slug", err.Error())
	}
	if ve.HasErrors() {
		return nil, ve
	}

	in.Version = sanitizeClientVersion(in.Version)

	before, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return nil, err
	}

	slugChanged := in.Slug != before.Slug
	oldPath := before.CalculatePath()
	// Snapshot mutable fields before UpdateNode mutates the live tree node.
	oldTitle := before.Title
	oldContent := before.Content

	var subtreeIDs []string
	if slugChanged {
		subtreeIDs = collectSubtreeIDs(before.PageNode)
		if len(subtreeIDs) == 0 {
			subtreeIDs = []string{in.ID}
		}
	}

	if err = uc.tree.UpdateNode(in.UserID, in.ID, in.Title, in.Slug, in.Content, in.Version); err != nil {
		return nil, err
	}

	after, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return nil, err
	}

	contentChanged := oldContent != after.Content
	titleChanged := oldTitle != after.Title

	event := pagesave.PageSaveEvent{
		Operation:      pagesave.PageOperationUpdate,
		UserID:         in.UserID,
		After:          after,
		OldPath:        oldPath,
		ContentChanged: contentChanged,
		SlugChanged:    slugChanged,
		TitleChanged:   titleChanged,
	}

	if slugChanged {
		pages, errs := uc.tree.GetPages(subtreeIDs)
		for i, p := range pages {
			if errs[i] != nil {
				uc.log.Warn("failed to get page for affected list", "pageID", subtreeIDs[i], "error", errs[i])
				continue
			}
			event.AffectedPages = append(event.AffectedPages, p)
		}
	}

	uc.orchestrator.Run(event)

	return &UpdatePageOutput{Page: after}, nil
}
