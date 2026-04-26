package pages

import (
	"github.com/perber/wiki/internal/core/revision"
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
