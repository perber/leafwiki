package wiki

import (
	"fmt"
	"sort"

	verrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

const (
	PageRefactorKindRename = "rename"
	PageRefactorKindMove   = "move"
)

type PageRefactorPreviewRequest struct {
	Kind        string  `json:"kind"`
	Title       string  `json:"title,omitempty"`
	Slug        string  `json:"slug,omitempty"`
	Content     *string `json:"content,omitempty"`
	NewParentID *string `json:"parentId,omitempty"`
}

type ApplyPageRefactorRequest struct {
	PageRefactorPreviewRequest
	RewriteLinks bool `json:"rewriteLinks"`
}

type PageRefactorPreview struct {
	Kind          string                     `json:"kind"`
	PageID        string                     `json:"pageId"`
	OldPath       string                     `json:"oldPath"`
	NewPath       string                     `json:"newPath"`
	AffectedPages []PageRefactorAffectedPage `json:"affectedPages"`
	Counts        PageRefactorPreviewCounts  `json:"counts"`
	Warnings      []string                   `json:"warnings"`
}

type PageRefactorPreviewCounts struct {
	AffectedPages int `json:"affectedPages"`
	MatchedLinks  int `json:"matchedLinks"`
}

type PageRefactorAffectedPage struct {
	FromPageID   string   `json:"fromPageId"`
	FromTitle    string   `json:"fromTitle"`
	FromPath     string   `json:"fromPath"`
	MatchedPaths []string `json:"matchedPaths"`
	Warnings     []string `json:"warnings"`
}

type pathChangeSnapshot struct {
	PageID   string
	OldPath  string
	Content  string
	RootPage bool
}

func (w *Wiki) PreviewPageRefactor(id string, req PageRefactorPreviewRequest) (*PageRefactorPreview, error) {
	page, err := w.tree.GetPage(id)
	if err != nil {
		return nil, err
	}

	oldPath := page.CalculatePath()
	newPath, err := w.computeRefactorTargetPath(page, req)
	if err != nil {
		return nil, err
	}

	excludePageIDs := subtreeIDSet(page.PageNode)
	affectedPages, matchedLinks, err := w.getRefactorAffectedPages(oldPath, excludePageIDs)
	if err != nil {
		return nil, err
	}

	warnings := collectPreviewWarnings(affectedPages)

	return &PageRefactorPreview{
		Kind:          req.Kind,
		PageID:        id,
		OldPath:       oldPath,
		NewPath:       newPath,
		AffectedPages: affectedPages,
		Counts: PageRefactorPreviewCounts{
			AffectedPages: len(affectedPages),
			MatchedLinks:  matchedLinks,
		},
		Warnings: ensureStrings(warnings),
	}, nil
}

func (w *Wiki) ApplyPageRefactor(userID string, id string, req ApplyPageRefactorRequest) (*tree.Page, error) {
	preview, err := w.PreviewPageRefactor(id, req.PageRefactorPreviewRequest)
	if err != nil {
		return nil, err
	}

	snapshots, err := w.capturePathChangeSnapshots(id, req)
	if err != nil {
		return nil, err
	}

	if req.RewriteLinks {
		rewriteRules := []links.RewriteRule{{
			OldPath: preview.OldPath,
			NewPath: preview.NewPath,
		}}
		if err := w.rewriteRefactorAffectedPages(userID, preview.AffectedPages, rewriteRules); err != nil {
			return nil, err
		}
	}

	switch req.Kind {
	case PageRefactorKindRename:
		kind := tree.NodeKindPage
		updated, err := w.UpdatePage(userID, id, req.Title, req.Slug, req.Content, &kind)
		if err != nil {
			return nil, err
		}
		if err := w.rewritePathChangedSubtree(userID, snapshots, preview.OldPath, preview.NewPath); err != nil {
			return nil, err
		}
		return w.tree.GetPage(updated.ID)
	case PageRefactorKindMove:
		parentID := ""
		if req.NewParentID != nil {
			parentID = *req.NewParentID
		}
		if err := w.MovePage(userID, id, parentID); err != nil {
			return nil, err
		}
		if err := w.rewritePathChangedSubtree(userID, snapshots, preview.OldPath, preview.NewPath); err != nil {
			return nil, err
		}
		return w.tree.GetPage(id)
	default:
		return nil, fmt.Errorf("unsupported refactor kind: %s", req.Kind)
	}
}

func (w *Wiki) computeRefactorTargetPath(page *tree.Page, req PageRefactorPreviewRequest) (string, error) {
	switch req.Kind {
	case PageRefactorKindRename:
		ve := verrors.NewValidationErrors()
		if req.Title == "" {
			ve.Add("title", "Title must not be empty")
		}
		if err := w.slug.IsValidSlug(req.Slug); err != nil {
			ve.Add("slug", err.Error())
		}
		if ve.HasErrors() {
			return "", ve
		}
		parentPath := ""
		if page.Parent != nil {
			parentPath = page.Parent.CalculatePath()
		}
		if parentPath == "" {
			return "/" + req.Slug, nil
		}
		return parentPath + "/" + req.Slug, nil
	case PageRefactorKindMove:
		parentID := ""
		if req.NewParentID != nil {
			parentID = *req.NewParentID
		}
		parentPath, err := w.resolveParentPath(parentID)
		if err != nil {
			return "", err
		}
		if parentPath == "" {
			return "/" + page.Slug, nil
		}
		return parentPath + "/" + page.Slug, nil
	default:
		return "", fmt.Errorf("unsupported refactor kind: %s", req.Kind)
	}
}

func (w *Wiki) resolveParentPath(parentID string) (string, error) {
	if parentID == "" || parentID == "root" {
		return "", nil
	}
	parent, err := w.tree.GetPage(parentID)
	if err != nil {
		return "", err
	}
	return parent.CalculatePath(), nil
}

func (w *Wiki) getRefactorAffectedPages(oldPath string, excludeIDs map[string]struct{}) ([]PageRefactorAffectedPage, int, error) {
	if w.links == nil {
		return nil, 0, nil
	}

	matches, err := w.links.GetRefactorMatchesForPrefix(oldPath)
	if err != nil {
		return nil, 0, err
	}

	grouped := make(map[string]*PageRefactorAffectedPage)
	totalMatches := 0
	for _, match := range matches {
		if _, excluded := excludeIDs[match.FromPageID]; excluded {
			continue
		}

		fromPath := ""
		page, err := w.tree.GetPage(match.FromPageID)
		if err == nil && page != nil {
			fromPath = page.CalculatePath()
		}

		item, ok := grouped[match.FromPageID]
		if !ok {
			item = &PageRefactorAffectedPage{
				FromPageID: match.FromPageID,
				FromTitle:  match.FromTitle,
				FromPath:   fromPath,
			}
			grouped[match.FromPageID] = item
		}

		if !containsString(item.MatchedPaths, match.ToPath) {
			item.MatchedPaths = append(item.MatchedPaths, match.ToPath)
		}
		totalMatches++
	}

	items := make([]PageRefactorAffectedPage, 0, len(grouped))
	for _, item := range grouped {
		if err := w.populateAffectedPageWarnings(item, oldPath); err != nil {
			return nil, 0, err
		}
		sort.Strings(item.MatchedPaths)
		sort.Strings(item.Warnings)
		item.MatchedPaths = ensureStrings(item.MatchedPaths)
		item.Warnings = ensureStrings(item.Warnings)
		items = append(items, *item)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].FromTitle == items[j].FromTitle {
			return items[i].FromPath < items[j].FromPath
		}
		return items[i].FromTitle < items[j].FromTitle
	})

	return items, totalMatches, nil
}

func (w *Wiki) rewriteRefactorAffectedPages(userID string, affectedPages []PageRefactorAffectedPage, rules []links.RewriteRule) error {
	engine := links.NewMarkdownRefactorEngine()
	for _, affectedPage := range affectedPages {
		page, err := w.tree.GetPage(affectedPage.FromPageID)
		if err != nil {
			return err
		}

		result := engine.Rewrite(page.Content, page.CalculatePath(), rules)
		if result.Count() == 0 || result.Content == page.Content {
			continue
		}

		updatedContent := result.Content
		if err := w.tree.UpdateNode(userID, page.ID, page.Title, page.Slug, &updatedContent); err != nil {
			return err
		}

		if w.links != nil {
			updatedPage, err := w.tree.GetPage(page.ID)
			if err != nil {
				return err
			}
			if err := w.links.UpdateLinksForPage(updatedPage, updatedContent); err != nil {
				return err
			}
		}
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (w *Wiki) populateAffectedPageWarnings(page *PageRefactorAffectedPage, oldPath string) error {
	rewriteRules := []links.RewriteRule{{
		OldPath: oldPath,
		NewPath: oldPath,
	}}

	sourcePage, err := w.tree.GetPage(page.FromPageID)
	if err != nil {
		return err
	}

	engine := links.NewMarkdownRefactorEngine()
	result := engine.Rewrite(sourcePage.Content, sourcePage.CalculatePath(), rewriteRules)
	for _, warning := range result.Warnings {
		if !containsString(page.Warnings, warning.Message) {
			page.Warnings = append(page.Warnings, warning.Message)
		}
	}

	return nil
}

func collectPreviewWarnings(pages []PageRefactorAffectedPage) []string {
	var warnings []string
	for _, page := range pages {
		for _, warning := range page.Warnings {
			if !containsString(warnings, warning) {
				warnings = append(warnings, warning)
			}
		}
	}
	sort.Strings(warnings)
	return ensureStrings(warnings)
}

func ensureStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func subtreeIDSet(node *tree.PageNode) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, id := range collectSubtreeIDs(node) {
		ids[id] = struct{}{}
	}
	return ids
}

func (w *Wiki) capturePathChangeSnapshots(id string, req ApplyPageRefactorRequest) ([]pathChangeSnapshot, error) {
	page, err := w.tree.GetPage(id)
	if err != nil {
		return nil, err
	}

	subtreeIDs := collectSubtreeIDs(page.PageNode)
	if len(subtreeIDs) == 0 {
		subtreeIDs = []string{id}
	}

	snapshots := make([]pathChangeSnapshot, 0, len(subtreeIDs))
	for _, pageID := range subtreeIDs {
		currentPage, err := w.tree.GetPage(pageID)
		if err != nil {
			return nil, err
		}

		content := currentPage.Content
		if pageID == id && req.Content != nil {
			content = *req.Content
		}

		snapshots = append(snapshots, pathChangeSnapshot{
			PageID:   currentPage.ID,
			OldPath:  currentPage.CalculatePath(),
			Content:  content,
			RootPage: pageID == id,
		})
	}

	return snapshots, nil
}

func (w *Wiki) rewritePathChangedSubtree(userID string, snapshots []pathChangeSnapshot, oldPath string, newPath string) error {
	if len(snapshots) == 0 {
		return nil
	}

	engine := links.NewMarkdownRefactorEngine()
	rules := []links.RewriteRule{{
		OldPath: oldPath,
		NewPath: newPath,
	}}

	for _, snapshot := range snapshots {
		currentPage, err := w.tree.GetPage(snapshot.PageID)
		if err != nil {
			return err
		}

		result := engine.RewriteRelativeLinksForPathChange(snapshot.Content, snapshot.OldPath, currentPage.CalculatePath(), rules)
		if result.Count() == 0 && snapshot.Content == currentPage.Content {
			continue
		}

		updatedContent := result.Content
		if updatedContent == currentPage.Content {
			continue
		}

		if err := w.tree.UpdateNode(userID, currentPage.ID, currentPage.Title, currentPage.Slug, &updatedContent); err != nil {
			return err
		}

		if w.links != nil {
			updatedPage, err := w.tree.GetPage(currentPage.ID)
			if err != nil {
				return err
			}
			if err := w.links.UpdateLinksForPage(updatedPage, updatedContent); err != nil {
				return err
			}
			if err := w.links.HealLinksForExactPath(updatedPage); err != nil {
				return err
			}
		}
	}

	return nil
}
