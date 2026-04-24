package pages

import (
	"context"
	"log"
	"log/slog"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

// DeletePageInput is the input for DeletePageUseCase.
type DeletePageInput struct {
	UserID    string
	ID        string
	Recursive bool
}

// DeletePageUseCase removes a page (and optionally its subtree) including assets, links, and revisions.
type DeletePageUseCase struct {
	tree     *tree.TreeService
	revision *revision.Service
	links    *links.LinkService
	assets   *assets.AssetService
	log      *slog.Logger
}

// NewDeletePageUseCase constructs a DeletePageUseCase.
func NewDeletePageUseCase(
	t *tree.TreeService,
	r *revision.Service,
	l *links.LinkService,
	a *assets.AssetService,
	log *slog.Logger,
) *DeletePageUseCase {
	return &DeletePageUseCase{tree: t, revision: r, links: l, assets: a, log: log}
}

// Execute deletes the page, cleaning up links, assets, and revision data.
func (uc *DeletePageUseCase) Execute(_ context.Context, in DeletePageInput) error {
	if in.ID == "root" || in.ID == "" {
		return newPageRootOperationError("delete")
	}

	page, err := uc.tree.GetPage(in.ID)
	if err != nil {
		return err
	}

	if in.Recursive {
		var subtreeIDs []string
		var oldPrefix string

		if uc.tree.IsLoaded() {
			node, err := uc.tree.FindPageByID(in.ID)
			if err == nil && node != nil {
				subtreeIDs = collectSubtreeIDs(node)
				oldPrefix = node.CalculatePath()
			}
		}
		if len(subtreeIDs) == 0 || oldPrefix == "" {
			subtreeIDs = []string{in.ID}
			oldPrefix = page.CalculatePath()
		}

		if err := uc.tree.DeleteNode(in.UserID, in.ID, true); err != nil {
			return err
		}

		if uc.links != nil {
			for _, pid := range subtreeIDs {
				if err := uc.links.DeleteOutgoingLinksForPage(pid); err != nil {
					log.Printf("warning: could not delete outgoing links for page %s: %v", pid, err)
				}
			}
			if oldPrefix != "" {
				if err := uc.links.MarkLinksBrokenForPrefix(oldPrefix); err != nil {
					log.Printf("warning: could not mark links broken for prefix %s: %v", oldPrefix, err)
				}
			}
		}

		for _, pid := range subtreeIDs {
			if err := uc.assets.DeleteAllAssetsForPage(&tree.PageNode{ID: pid}); err != nil {
				log.Printf("warning: could not delete assets for page %s: %v", pid, err)
			}
		}

		return deleteRevisionData(uc.revision, subtreeIDs)
	}

	// non-recursive
	if err := uc.tree.DeleteNode(in.UserID, in.ID, false); err != nil {
		return err
	}

	if uc.links != nil {
		if err := uc.links.DeleteOutgoingLinksForPage(in.ID); err != nil {
			log.Printf("warning: could not delete outgoing links for page %s: %v", in.ID, err)
		}
		if err := uc.links.MarkIncomingLinksBrokenForPage(in.ID); err != nil {
			log.Printf("warning: could not mark incoming links broken for page %s: %v", in.ID, err)
		}
		if err := uc.links.MarkLinksBrokenForPath(page.CalculatePath()); err != nil {
			log.Printf("warning: could not mark links broken for path %s: %v", page.CalculatePath(), err)
		}
	}

	if err := uc.assets.DeleteAllAssetsForPage(page.PageNode); err != nil {
		log.Printf("warning: could not delete assets for page %s: %v", page.ID, err)
	}

	return deleteRevisionData(uc.revision, []string{in.ID})
}
