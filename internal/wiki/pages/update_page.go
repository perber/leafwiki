package pages

import (
	"context"
	"log"
	"log/slog"

	"github.com/perber/wiki/internal/core/revision"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// UpdatePageInput is the input for UpdatePageUseCase.
type UpdatePageInput struct {
	UserID  string
	ID      string
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
	tree     *tree.TreeService
	slug     *tree.SlugService
	revision *revision.Service
	links    *links.LinkService
	log      *slog.Logger
}

// NewUpdatePageUseCase constructs an UpdatePageUseCase.
func NewUpdatePageUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	r *revision.Service,
	l *links.LinkService,
	log *slog.Logger,
) *UpdatePageUseCase {
	return &UpdatePageUseCase{tree: t, slug: s, revision: r, links: l, log: log}
}

// Execute validates, updates the node, maintains link indexes, and records a revision.
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

	before, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return nil, err
	}
	oldTitle := before.Title
	oldPrefix := before.CalculatePath()
	renameOrPathChange := in.Slug != before.Slug

	var subtreeIDs []string
	if renameOrPathChange {
		subtreeIDs = collectSubtreeIDs(before.PageNode)
		if len(subtreeIDs) == 0 {
			subtreeIDs = []string{in.ID}
		}
	}

	if err = uc.tree.UpdateNode(in.UserID, in.ID, in.Title, in.Slug, in.Content); err != nil {
		return nil, err
	}

	after, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return nil, err
	}
	contentChanged := before.Content != after.Content
	titleChanged := oldTitle != after.Title

	if uc.links != nil {
		if renameOrPathChange {
			if oldPrefix != "" {
				if err := uc.links.MarkLinksBrokenForPrefix(oldPrefix); err != nil {
					log.Printf("warning: could not mark links broken for prefix %s: %v", oldPrefix, err)
				}
			}
			for _, pid := range subtreeIDs {
				p, err := uc.tree.GetPage(pid)
				if err != nil {
					log.Printf("warning: failed to get page %s for healing links: %v", pid, err)
					continue
				}
				if err := uc.links.UpdateLinksForPage(p, p.Content); err != nil {
					log.Printf("warning: failed to update links for page %s: %v", pid, err)
				}
				if err := uc.links.HealLinksForExactPath(p); err != nil {
					log.Printf("warning: failed to heal links for page %s: %v", p.ID, err)
				}
			}
		} else {
			if in.Content != nil {
				if err := uc.links.UpdateLinksForPage(after, *in.Content); err != nil {
					log.Printf("warning: failed to update links for page %s: %v", after.ID, err)
				}
			}
			if err := uc.links.HealLinksForExactPath(after); err != nil {
				log.Printf("warning: failed to heal links for page %s: %v", after.ID, err)
			}
		}
	}

	if uc.revision != nil {
		if renameOrPathChange {
			for _, pid := range subtreeIDs {
				if contentChanged && pid == in.ID {
					continue
				}
				recordStructureRevision(uc.revision, uc.log, pid, in.UserID)
			}
		} else if titleChanged && !contentChanged {
			recordStructureRevision(uc.revision, uc.log, in.ID, in.UserID)
		}
		if contentChanged {
			recordContentRevision(uc.revision, uc.log, in.ID, in.UserID, "")
		}
	}

	return &UpdatePageOutput{Page: after}, nil
}
