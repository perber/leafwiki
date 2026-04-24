package pages

import (
	"log/slog"

	"github.com/perber/wiki/internal/core/revision"
	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
)

// collectSubtreeIDs returns all page IDs within a subtree (excluding "root").
func collectSubtreeIDs(node *tree.PageNode) []string {
	var ids []string
	var walk func(n *tree.PageNode)
	walk = func(n *tree.PageNode) {
		if n == nil {
			return
		}
		if n.ID != "root" {
			ids = append(ids, n.ID)
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(node)
	return ids
}

// recordContentRevision records a content revision; errors are logged but not fatal.
func recordContentRevision(svc *revision.Service, log *slog.Logger, pageID, userID, summary string) {
	if svc == nil {
		return
	}
	if _, _, err := svc.RecordContentUpdate(pageID, userID, summary); err != nil {
		log.Warn("failed to record content revision", "pageID", pageID, "error", err)
	}
}

// recordStructureRevision records a structure revision; errors are logged but not fatal.
func recordStructureRevision(svc *revision.Service, log *slog.Logger, pageID, userID string) {
	if svc == nil {
		return
	}
	if _, _, err := svc.RecordStructureChange(pageID, userID, ""); err != nil {
		log.Warn("failed to record structure revision", "pageID", pageID, "error", err)
	}
}

// deleteRevisionData removes all revision data for a list of page IDs.
func deleteRevisionData(svc *revision.Service, pageIDs []string) error {
	if svc == nil {
		return nil
	}
	for _, id := range pageIDs {
		if err := svc.DeletePageData(id); err != nil {
			return err
		}
	}
	return nil
}

func requireCurrentPageVersion(page *tree.Page, expectedVersion string) error {
	if page == nil {
		return nil
	}
	if expectedVersion == "" {
		return sharederrors.NewLocalizedError(
			ErrCodePageVersionRequired,
			"Page version is required",
			"page version is required",
			nil,
		)
	}
	if page.Version() != expectedVersion {
		return sharederrors.NewLocalizedError(
			ErrCodePageVersionConflict,
			"Page was changed by another request",
			"page was changed by another request",
			nil,
		)
	}
	return nil
}
