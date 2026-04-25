package pages

import (
	"context"
	"log/slog"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/wiki/pagesave"
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
	tree         *tree.TreeService
	orchestrator *pagesave.PageSaveOrchestrator
	log          *slog.Logger
}

// NewMovePageUseCase constructs a MovePageUseCase.
func NewMovePageUseCase(
	t *tree.TreeService,
	o *pagesave.PageSaveOrchestrator,
	log *slog.Logger,
) *MovePageUseCase {
	return &MovePageUseCase{tree: t, orchestrator: o, log: log}
}

// Execute moves the page and fires post-save side effects for the whole subtree.
func (uc *MovePageUseCase) Execute(_ context.Context, in MovePageInput) error {
	if in.ID == "root" || in.ID == "" {
		return newPageRootOperationError("move")
	}

	var subtreeIDs []string
	var beforePage *tree.Page

	if uc.tree.IsLoaded() {
		if node, err := uc.tree.FindPageByID(in.ID); err == nil && node != nil {
			subtreeIDs = collectSubtreeIDs(node)
			if p, err := uc.tree.GetPage(in.ID); err == nil {
				beforePage = p
			}
		}
	}
	if len(subtreeIDs) == 0 {
		subtreeIDs = []string{in.ID}
	}
	if beforePage == nil {
		p, err := uc.tree.GetPage(in.ID)
		if err != nil {
			return err
		}
		beforePage = p
	}

	if err := requireCurrentPageVersion(beforePage, in.Version); err != nil {
		return err
	}

	var oldPath string
	if beforePage != nil {
		oldPath = beforePage.CalculatePath()
	}

	if err := uc.tree.MoveNode(in.UserID, in.ID, in.ParentID); err != nil {
		return err
	}

	event := pagesave.PageSaveEvent{
		Operation: pagesave.PageOperationMove,
		UserID:    in.UserID,
		OldPath:   oldPath,
	}

	pages, errs := uc.tree.GetPages(subtreeIDs)
	for i, p := range pages {
		if errs[i] != nil {
			uc.log.Warn("failed to get page after move", "pageID", subtreeIDs[i], "error", errs[i])
			continue
		}
		event.AffectedPages = append(event.AffectedPages, p)
	}

	uc.orchestrator.Run(event)

	return nil
}
