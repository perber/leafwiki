package pages

import (
	"context"

	"github.com/perber/wiki/internal/core/tree"
)

// PageQueries bundles the read-only page lookups that have no policy of
// their own beyond the tree lookup they delegate to.
type PageQueries struct {
	tree *tree.TreeService
}

// NewPageQueries constructs a PageQueries.
func NewPageQueries(t *tree.TreeService) *PageQueries {
	return &PageQueries{tree: t}
}

// GetPage fetches the page by ID.
func (q *PageQueries) GetPage(id string) (*tree.Page, error) {
	return q.tree.GetPage(id)
}

// FindByPath finds the page matching a URL route path (e.g. "docs/api/intro").
func (q *PageQueries) FindByPath(routePath string) (*tree.Page, error) {
	return q.tree.FindPageByRoutePath(routePath)
}

// LookupPath resolves a path to its tree segments, including non-existing ones.
func (q *PageQueries) LookupPath(path string) (*tree.PathLookup, error) {
	return q.tree.LookupPagePath(path)
}

// ResolvePermalink resolves a stable page ID to its current route path.
func (q *PageQueries) ResolvePermalink(id string) (*tree.PermalinkTarget, error) {
	return q.tree.ResolvePermalinkTarget(id)
}

// ─── FindByTitle ─────────────────────────────────────────────────────────────

// WikiLinkMatch is a lightweight page descriptor returned by the by-title API.
type WikiLinkMatch struct {
	ID    string        `json:"id"`
	Title string        `json:"title"`
	Path  string        `json:"path"`
	Kind  tree.NodeKind `json:"kind"`
}

// FindByTitleOutput is the output of FindByTitleUseCase.
type FindByTitleOutput struct {
	Matches []WikiLinkMatch `json:"matches"`
	Count   int             `json:"count"`
}

// FindByTitle finds all pages whose title matches a given string
// (case-insensitive).
func (q *PageQueries) FindByTitle(title string) *FindByTitleOutput {
	nodes := q.tree.FindPagesByTitle(title)
	matches := make([]WikiLinkMatch, 0, len(nodes))
	for _, n := range nodes {
		matches = append(matches, WikiLinkMatch{
			ID:    n.ID,
			Title: n.Title,
			Path:  n.CalculatePath(),
			Kind:  n.Kind,
		})
	}
	return &FindByTitleOutput{Matches: matches, Count: len(matches)}
}

// ─── SortPages ──────────────────────────────────────────────────────────────

// SortPagesInput is the input for SortPagesUseCase.
type SortPagesInput struct {
	ParentID   string
	OrderedIDs []string
}

// SortPagesUseCase reorders the children of a parent node.
type SortPagesUseCase struct {
	tree *tree.TreeService
}

// NewSortPagesUseCase constructs a SortPagesUseCase.
func NewSortPagesUseCase(t *tree.TreeService) *SortPagesUseCase {
	return &SortPagesUseCase{tree: t}
}

// Execute applies the new child order to the parent node.
func (uc *SortPagesUseCase) Execute(_ context.Context, in SortPagesInput) error {
	return uc.tree.SortPages(in.ParentID, in.OrderedIDs)
}

// ─── SuggestSlug ────────────────────────────────────────────────────────────

// SuggestSlugInput is the input for SuggestSlugUseCase.
type SuggestSlugInput struct {
	ParentID  string
	CurrentID string
	Title     string
}

// SuggestSlugOutput is the output of SuggestSlugUseCase.
type SuggestSlugOutput struct {
	Slug string
}

// SuggestSlugUseCase generates a unique slug suggestion for the given title in a parent.
type SuggestSlugUseCase struct {
	tree *tree.TreeService
	slug *tree.SlugService
}

// NewSuggestSlugUseCase constructs a SuggestSlugUseCase.
func NewSuggestSlugUseCase(t *tree.TreeService, s *tree.SlugService) *SuggestSlugUseCase {
	return &SuggestSlugUseCase{tree: t, slug: s}
}

// Execute generates and returns a unique slug suggestion.
func (uc *SuggestSlugUseCase) Execute(_ context.Context, in SuggestSlugInput) (*SuggestSlugOutput, error) {
	if in.ParentID == "" || in.ParentID == "root" {
		return &SuggestSlugOutput{Slug: uc.slug.GenerateUniqueChildSlug(uc.tree.GetTree(), in.CurrentID, in.Title)}, nil
	}
	parent, err := uc.tree.FindPageByID(in.ParentID)
	if err != nil {
		return nil, err
	}
	return &SuggestSlugOutput{Slug: uc.slug.GenerateUniqueChildSlug(parent, in.CurrentID, in.Title)}, nil
}
