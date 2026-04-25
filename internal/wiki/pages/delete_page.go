package pages

import (
	"context"
	"log/slog"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pagesave"
)

// DeletePageInput is the input for DeletePageUseCase.
type DeletePageInput struct {
	UserID    string
	ID        string
	Version   string
	Recursive bool
}

// DeletePageUseCase removes a page (and optionally its subtree) including assets, links, and revisions.
type DeletePageUseCase struct {
	tree         *tree.TreeService
	revision     *revision.Service
	assets       *assets.AssetService
	orchestrator *pagesave.PageSaveOrchestrator
	log          *slog.Logger
}

// NewDeletePageUseCase constructs a DeletePageUseCase.
func NewDeletePageUseCase(
	t *tree.TreeService,
	r *revision.Service,
	a *assets.AssetService,
	o *pagesave.PageSaveOrchestrator,
	log *slog.Logger,
) *DeletePageUseCase {
	return &DeletePageUseCase{tree: t, revision: r, assets: a, orchestrator: o, log: log}
}

// Execute deletes the page, cleaning up links (via orchestrator), assets, and revision data.
func (uc *DeletePageUseCase) Execute(_ context.Context, in DeletePageInput) error {
	if in.ID == "root" || in.ID == "" {
		return newPageRootOperationError("delete")
	}

	page, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return err
	}
	if err := requireCurrentPageVersion(page, in.Version); err != nil {
		return err
	}

	if in.Recursive {
		var subtreeIDs []string

		if uc.tree.IsLoaded() {
			node, err := uc.tree.FindPageByID(in.ID)
			if err == nil && node != nil {
				subtreeIDs = collectSubtreeIDs(node)
			}
		}
		if len(subtreeIDs) == 0 {
			subtreeIDs = []string{in.ID}
		}

		// Build affected pages list before deletion (paths are no longer reachable after).
		affectedPages := make([]*tree.Page, 0, len(subtreeIDs))
		for _, pid := range subtreeIDs {
			p, err := uc.tree.GetPage(pid)
			if err != nil {
				uc.log.Warn("failed to get page before recursive delete", "pageID", pid, "error", err)
				continue
			}
			affectedPages = append(affectedPages, p)
		}

		oldPath := page.CalculatePath()

		if err := uc.tree.DeleteNode(in.UserID, in.ID, true); err != nil {
			return err
		}

		uc.orchestrator.Run(pagesave.PageSaveEvent{
			Operation:     pagesave.PageOperationDelete,
			UserID:        in.UserID,
			Before:        page,
			OldPath:       oldPath,
			AffectedPages: affectedPages,
		})

		for _, p := range affectedPages {
			if err := uc.assets.DeleteAllAssetsForPage(p.PageNode); err != nil {
				uc.log.Warn("failed to delete assets for page", "pageID", p.ID, "error", err)
			}
		}

		return deleteRevisionData(uc.revision, subtreeIDs)
	}

	// Non-recursive delete.
	oldPath := page.CalculatePath()

	if err := uc.tree.DeleteNode(in.UserID, in.ID, false); err != nil {
		return err
	}

	uc.orchestrator.Run(pagesave.PageSaveEvent{
		Operation:     pagesave.PageOperationDelete,
		UserID:        in.UserID,
		Before:        page,
		OldPath:       oldPath,
		AffectedPages: []*tree.Page{page},
	})

	if err := uc.assets.DeleteAllAssetsForPage(page.PageNode); err != nil {
		uc.log.Warn("failed to delete assets for page", "pageID", page.ID, "error", err)
	}

	return deleteRevisionData(uc.revision, []string{in.ID})
}
