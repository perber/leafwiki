package pages

import (
	"context"
	"log"
	"log/slog"

	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// MovePageInput is the input for MovePageUseCase.
type MovePageInput struct {
	UserID   string
	ID       string
	Version  string
	ParentID string
}

// MovePageUseCase moves a page to a new parent, updating links and recording revisions.
type MovePageUseCase struct {
	tree     *tree.TreeService
	revision *revision.Service
	links    *links.LinkService
	log      *slog.Logger
}

// NewMovePageUseCase constructs a MovePageUseCase.
func NewMovePageUseCase(
	t *tree.TreeService,
	r *revision.Service,
	l *links.LinkService,
	log *slog.Logger,
) *MovePageUseCase {
	return &MovePageUseCase{tree: t, revision: r, links: l, log: log}
}

// Execute moves the page and updates link state across the whole subtree.
func (uc *MovePageUseCase) Execute(_ context.Context, in MovePageInput) error {
	if in.ID == "root" || in.ID == "" {
		return newPageRootOperationError("move")
	}

	var oldPrefix string
	var subtreeIDs []string

	if uc.tree.IsLoaded() {
		if node, err := uc.tree.FindPageByID(in.ID); err == nil && node != nil {
			oldPrefix = node.CalculatePath()
			subtreeIDs = collectSubtreeIDs(node)
		}
	}
	if len(subtreeIDs) == 0 {
		subtreeIDs = []string{in.ID}
		if p, err := uc.tree.GetPage(in.ID); err == nil && oldPrefix == "" {
			oldPrefix = p.CalculatePath()
		}
	}

	page, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return err
	}
	if err := requireCurrentPageVersion(page, in.Version); err != nil {
		return err
	}

	if err := uc.tree.MoveNode(in.UserID, in.ID, in.ParentID); err != nil {
		return err
	}

	if uc.links != nil && oldPrefix != "" {
		if err := uc.links.MarkLinksBrokenForPrefix(oldPrefix); err != nil {
			log.Printf("warning: could not mark links broken for prefix %s: %v", oldPrefix, err)
		}
	}

	for _, pid := range subtreeIDs {
		p, err := uc.tree.GetPage(pid)
		if err != nil {
			log.Printf("warning: failed to get page %s after move: %v", pid, err)
			continue
		}
		if uc.links != nil {
			if err := uc.links.UpdateLinksForPage(p, p.Content); err != nil {
				log.Printf("warning: failed to update links for page %s: %v", pid, err)
			}
			if err := uc.links.HealLinksForExactPath(p); err != nil {
				log.Printf("warning: failed to heal links for page %s: %v", pid, err)
			}
		}
	}

	if uc.revision != nil {
		for _, pid := range subtreeIDs {
			recordStructureRevision(uc.revision, uc.log, pid, in.UserID)
		}
	}

	return nil
}
