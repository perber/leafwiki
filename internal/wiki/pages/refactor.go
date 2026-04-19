package pages

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
)

const (
	RefactorKindRename = "rename"
	RefactorKindMove   = "move"
)

// RefactorPreviewInput is the input for PreviewPageRefactorUseCase.
type RefactorPreviewInput struct {
	PageID      string
	Kind        string
	Title       string
	Slug        string
	Content     *string
	NewParentID *string
}

// RefactorPreview is the result of a refactor preview operation.
type RefactorPreview struct {
	Kind          string                 `json:"kind"`
	PageID        string                 `json:"pageId"`
	OldPath       string                 `json:"oldPath"`
	NewPath       string                 `json:"newPath"`
	AffectedPages []RefactorAffectedPage `json:"affectedPages"`
	Counts        RefactorPreviewCounts  `json:"counts"`
	Warnings      []string               `json:"warnings"`
}

// RefactorPreviewCounts holds aggregated counts for the preview.
type RefactorPreviewCounts struct {
	AffectedPages int `json:"affectedPages"`
	MatchedLinks  int `json:"matchedLinks"`
}

// RefactorAffectedPage describes a page that has links affected by the refactor.
type RefactorAffectedPage struct {
	FromPageID   string   `json:"fromPageId"`
	FromTitle    string   `json:"fromTitle"`
	FromPath     string   `json:"fromPath"`
	MatchedPaths []string `json:"matchedPaths"`
	Warnings     []string `json:"warnings"`
}

// RefactorApplyInput extends the preview with apply options.
type RefactorApplyInput struct {
	UserID string
	RefactorPreviewInput
	RewriteLinks bool
}

// PreviewPageRefactorUseCase computes what would change if a refactor is applied.
type PreviewPageRefactorUseCase struct {
	tree  *tree.TreeService
	slug  *tree.SlugService
	links *links.LinkService
	log   *slog.Logger
}

// NewPreviewPageRefactorUseCase constructs a PreviewPageRefactorUseCase.
func NewPreviewPageRefactorUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	l *links.LinkService,
	log *slog.Logger,
) *PreviewPageRefactorUseCase {
	return &PreviewPageRefactorUseCase{tree: t, slug: s, links: l, log: log}
}

// Execute computes the refactor preview without making changes.
func (uc *PreviewPageRefactorUseCase) Execute(_ context.Context, in RefactorPreviewInput) (*RefactorPreview, error) {
	page, err := uc.tree.GetPage(in.PageID)
	if err != nil {
		return nil, err
	}

	oldPath := page.CalculatePath()
	newPath, err := uc.computeTargetPath(page, in)
	if err != nil {
		return nil, err
	}

	excludeIDs := subtreeIDSet(page.PageNode)
	affectedPages, matchedLinks, err := uc.getAffectedPages(oldPath, excludeIDs)
	if err != nil {
		return nil, err
	}

	return &RefactorPreview{
		Kind:          in.Kind,
		PageID:        in.PageID,
		OldPath:       oldPath,
		NewPath:       newPath,
		AffectedPages: affectedPages,
		Counts: RefactorPreviewCounts{
			AffectedPages: len(affectedPages),
			MatchedLinks:  matchedLinks,
		},
		Warnings: collectPreviewWarnings(affectedPages),
	}, nil
}

func (uc *PreviewPageRefactorUseCase) computeTargetPath(page *tree.Page, in RefactorPreviewInput) (string, error) {
	switch in.Kind {
	case RefactorKindRename:
		ve := sharederrors.NewValidationErrors()
		if in.Title == "" {
			ve.Add("title", "Title must not be empty")
		}
		if err := uc.slug.IsValidSlug(in.Slug); err != nil {
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
			return "/" + in.Slug, nil
		}
		return parentPath + "/" + in.Slug, nil

	case RefactorKindMove:
		parentID := ""
		if in.NewParentID != nil {
			parentID = *in.NewParentID
		}
		parentPath, err := uc.resolveParentPath(parentID)
		if err != nil {
			return "", err
		}
		if parentPath == "" {
			return "/" + page.Slug, nil
		}
		return parentPath + "/" + page.Slug, nil

	default:
		return "", fmt.Errorf("unsupported refactor kind: %s", in.Kind)
	}
}

func (uc *PreviewPageRefactorUseCase) resolveParentPath(parentID string) (string, error) {
	if parentID == "" || parentID == "root" {
		return "", nil
	}
	parent, err := uc.tree.GetPage(parentID)
	if err != nil {
		return "", err
	}
	return parent.CalculatePath(), nil
}

func (uc *PreviewPageRefactorUseCase) getAffectedPages(oldPath string, excludeIDs map[string]struct{}) ([]RefactorAffectedPage, int, error) {
	if uc.links == nil {
		return nil, 0, nil
	}
	matches, err := uc.links.GetRefactorMatchesForPrefix(oldPath)
	if err != nil {
		return nil, 0, err
	}

	grouped := make(map[string]*RefactorAffectedPage)
	totalMatches := 0
	for _, match := range matches {
		if _, excluded := excludeIDs[match.FromPageID]; excluded {
			continue
		}
		fromPath := ""
		if page, err := uc.tree.GetPage(match.FromPageID); err == nil && page != nil {
			fromPath = page.CalculatePath()
		}
		item, ok := grouped[match.FromPageID]
		if !ok {
			item = &RefactorAffectedPage{
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

	engine := links.NewMarkdownRefactorEngine()
	items := make([]RefactorAffectedPage, 0, len(grouped))
	for _, item := range grouped {
		sourcePage, err := uc.tree.GetPage(item.FromPageID)
		if err != nil {
			return nil, 0, err
		}
		rules := []links.RewriteRule{{OldPath: oldPath, NewPath: oldPath}}
		result := engine.Rewrite(sourcePage.Content, sourcePage.CalculatePath(), rules)
		for _, w := range result.Warnings {
			if !containsString(item.Warnings, w.Message) {
				item.Warnings = append(item.Warnings, w.Message)
			}
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

// ─── ApplyPageRefactorUseCase ────────────────────────────────────────────────

// ApplyPageRefactorUseCase applies a rename or move with optional link rewriting.
type ApplyPageRefactorUseCase struct {
	tree     *tree.TreeService
	slug     *tree.SlugService
	revision *revision.Service
	links    *links.LinkService
	log      *slog.Logger
	preview  *PreviewPageRefactorUseCase
}

// NewApplyPageRefactorUseCase constructs an ApplyPageRefactorUseCase.
func NewApplyPageRefactorUseCase(
	t *tree.TreeService,
	s *tree.SlugService,
	r *revision.Service,
	l *links.LinkService,
	log *slog.Logger,
) *ApplyPageRefactorUseCase {
	return &ApplyPageRefactorUseCase{
		tree:     t,
		slug:     s,
		revision: r,
		links:    l,
		log:      log,
		preview:  NewPreviewPageRefactorUseCase(t, s, l, log),
	}
}

// Execute applies the refactor operation to the page tree.
func (uc *ApplyPageRefactorUseCase) Execute(ctx context.Context, in RefactorApplyInput) (*tree.Page, error) {
	prev, err := uc.preview.Execute(ctx, in.RefactorPreviewInput)
	if err != nil {
		return nil, err
	}

	snapshots, err := uc.captureSnapshots(in)
	if err != nil {
		return nil, err
	}

	if in.RewriteLinks {
		rules := []links.RewriteRule{{OldPath: prev.OldPath, NewPath: prev.NewPath}}
		if err := uc.rewriteAffectedPages(in.UserID, prev.AffectedPages, rules); err != nil {
			return nil, err
		}
	}

	switch in.Kind {
	case RefactorKindRename:
		updateUC := NewUpdatePageUseCase(uc.tree, uc.slug, uc.revision, uc.links, uc.log)
		updated, err := updateUC.Execute(ctx, UpdatePageInput{
			UserID:  in.UserID,
			ID:      in.PageID,
			Title:   in.Title,
			Slug:    in.Slug,
			Content: in.Content,
			Kind:    kindPage(),
		})
		if err != nil {
			return nil, err
		}
		if err := uc.rewritePathChangedSubtree(in.UserID, snapshots, prev.OldPath, prev.NewPath); err != nil {
			return nil, err
		}
		return uc.tree.GetPage(updated.Page.ID)

	case RefactorKindMove:
		parentID := ""
		if in.NewParentID != nil {
			parentID = *in.NewParentID
		}
		moveUC := NewMovePageUseCase(uc.tree, uc.revision, uc.links, uc.log)
		if err := moveUC.Execute(ctx, MovePageInput{UserID: in.UserID, ID: in.PageID, ParentID: parentID}); err != nil {
			return nil, err
		}
		if err := uc.rewritePathChangedSubtree(in.UserID, snapshots, prev.OldPath, prev.NewPath); err != nil {
			return nil, err
		}
		return uc.tree.GetPage(in.PageID)

	default:
		return nil, fmt.Errorf("unsupported refactor kind: %s", in.Kind)
	}
}

type pathChangeSnapshot struct {
	PageID   string
	OldPath  string
	Content  string
	RootPage bool
}

func (uc *ApplyPageRefactorUseCase) captureSnapshots(in RefactorApplyInput) ([]pathChangeSnapshot, error) {
	page, err := uc.tree.GetPage(in.PageID)
	if err != nil {
		return nil, err
	}
	ids := collectSubtreeIDs(page.PageNode)
	if len(ids) == 0 {
		ids = []string{in.PageID}
	}
	snapshots := make([]pathChangeSnapshot, 0, len(ids))
	for _, id := range ids {
		p, err := uc.tree.GetPage(id)
		if err != nil {
			return nil, err
		}
		content := p.Content
		if id == in.PageID && in.Content != nil {
			content = *in.Content
		}
		snapshots = append(snapshots, pathChangeSnapshot{
			PageID: p.ID, OldPath: p.CalculatePath(), Content: content, RootPage: id == in.PageID,
		})
	}
	return snapshots, nil
}

func (uc *ApplyPageRefactorUseCase) rewriteAffectedPages(userID string, affected []RefactorAffectedPage, rules []links.RewriteRule) error {
	engine := links.NewMarkdownRefactorEngine()
	for _, ap := range affected {
		page, err := uc.tree.GetPage(ap.FromPageID)
		if err != nil {
			return err
		}
		result := engine.Rewrite(page.Content, page.CalculatePath(), rules)
		if result.Count() == 0 || result.Content == page.Content {
			continue
		}
		content := result.Content
		if err := uc.tree.UpdateNode(userID, page.ID, page.Title, page.Slug, &content); err != nil {
			return err
		}
		if uc.revision != nil {
			recordContentRevision(uc.revision, uc.log, page.ID, userID, "")
		}
		if uc.links != nil {
			updated, err := uc.tree.GetPage(page.ID)
			if err != nil {
				return err
			}
			if err := uc.links.UpdateLinksForPage(updated, content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (uc *ApplyPageRefactorUseCase) rewritePathChangedSubtree(userID string, snapshots []pathChangeSnapshot, oldPath, newPath string) error {
	engine := links.NewMarkdownRefactorEngine()
	rules := []links.RewriteRule{{OldPath: oldPath, NewPath: newPath}}
	for _, snap := range snapshots {
		current, err := uc.tree.GetPage(snap.PageID)
		if err != nil {
			return err
		}
		result := engine.RewriteRelativeLinksForPathChange(snap.Content, snap.OldPath, current.CalculatePath(), rules)
		if result.Count() == 0 && snap.Content == current.Content {
			continue
		}
		content := result.Content
		if content == current.Content {
			continue
		}
		if err := uc.tree.UpdateNode(userID, current.ID, current.Title, current.Slug, &content); err != nil {
			return err
		}
		if uc.revision != nil {
			recordContentRevision(uc.revision, uc.log, current.ID, userID, "")
		}
		if uc.links != nil {
			updated, err := uc.tree.GetPage(current.ID)
			if err != nil {
				return err
			}
			if err := uc.links.UpdateLinksForPage(updated, content); err != nil {
				return err
			}
			if err := uc.links.HealLinksForExactPath(updated); err != nil {
				return err
			}
		}
	}
	return nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func subtreeIDSet(node *tree.PageNode) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, id := range collectSubtreeIDs(node) {
		ids[id] = struct{}{}
	}
	return ids
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func ensureStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func collectPreviewWarnings(pages []RefactorAffectedPage) []string {
	var warnings []string
	for _, p := range pages {
		for _, w := range p.Warnings {
			if !containsString(warnings, w) {
				warnings = append(warnings, w)
			}
		}
	}
	sort.Strings(warnings)
	return ensureStrings(warnings)
}

func kindPage() *tree.NodeKind {
	k := tree.NodeKindPage
	return &k
}
