package search

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/shared/htmlutil"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/http/dto"
	coresearch "github.com/perber/wiki/internal/search"
	coretags "github.com/perber/wiki/internal/tags"
)

// Caps on the text query of one search. Each search term is one more MATCH
// over the whole index (plus the rank tiers), so a long query of short words
// costs far more than a long word; the term cap is the one that matters, the
// rune cap bounds the parsing work before it. The cost grows linearly with
// the terms, so the cap bounds the worst case; it does not make it cheap.
const (
	maxQueryRunes = 256
	maxQueryTerms = 12
)

var ErrSearchUnavailable = sharederrors.NewLocalizedError(
	ErrCodeSearchUnavailable,
	"Search is currently unavailable",
	"search is currently unavailable",
	nil,
)

// ErrSearchQueryTooLong rejects a query over maxQueryRunes runes or
// maxQueryTerms terms. It is rejected rather than truncated, so the hits
// always match what the user typed.
var ErrSearchQueryTooLong = sharederrors.NewLocalizedError(
	ErrCodeSearchQueryTooLong,
	"Search query is too long: at most "+strconv.Itoa(maxQueryRunes)+" characters and "+strconv.Itoa(maxQueryTerms)+" terms",
	"search query is too long: at most %s characters and %s terms",
	nil,
	strconv.Itoa(maxQueryRunes), strconv.Itoa(maxQueryTerms),
)

// ─── SearchUseCase ───────────────────────────────────────────────────────────

type SearchInput struct {
	Query  string
	Tags   []string
	Offset int
	Limit  int
}

type SearchOutput struct {
	Result *coresearch.SearchResult
}

type SearchUseCase struct {
	index *coresearch.SQLiteIndex
	tags  *coretags.TagsService
	tree  *tree.TreeService
}

func NewSearchUseCase(idx *coresearch.SQLiteIndex, tags *coretags.TagsService, tree *tree.TreeService) *SearchUseCase {
	return &SearchUseCase{index: idx, tags: tags, tree: tree}
}

func (uc *SearchUseCase) Execute(_ context.Context, in SearchInput) (*SearchOutput, error) {
	query := strings.TrimSpace(in.Query)
	if utf8.RuneCountInString(query) > maxQueryRunes || coresearch.QueryTermCount(query) > maxQueryTerms {
		return nil, ErrSearchQueryTooLong
	}

	if uc.index == nil {
		return nil, ErrSearchUnavailable
	}

	var pageIDs []string
	if len(in.Tags) > 0 {
		pageIDs = []string{}
		if uc.tags != nil {
			var err error
			pageIDs, err = uc.tags.GetPageIDsByTags(normalizeTags(in.Tags))
			if err != nil {
				return nil, err
			}
		}
	}

	if query == "" && len(pageIDs) > 0 {
		return uc.searchByTags(pageIDs, in.Offset, in.Limit)
	}

	result, err := uc.index.Search(in.Query, pageIDs, in.Offset, in.Limit)
	if err != nil {
		return nil, err
	}

	fullMatchPageIDs, err := uc.index.SearchPageIDs(in.Query, pageIDs)
	if err != nil {
		return nil, err
	}

	uc.attachTags(result.Items)
	result.TagFacets = uc.buildTagFacets(fullMatchPageIDs)
	return &SearchOutput{Result: result}, nil
}

func (uc *SearchUseCase) searchByTags(pageIDs []string, offset, limit int) (*SearchOutput, error) {
	if uc.tags == nil || uc.tree == nil {
		return &SearchOutput{
			Result: &coresearch.SearchResult{
				Count:     0,
				Items:     []coresearch.SearchResultItem{},
				Offset:    offset,
				Limit:     limit,
				TagFacets: []coresearch.SearchTagFacet{},
			},
		}, nil
	}

	items := make([]coresearch.SearchResultItem, 0, len(pageIDs))
	for _, pageID := range pageIDs {
		node, err := uc.tree.FindPageByID(pageID)
		if err != nil || node == nil {
			continue
		}

		items = append(items, coresearch.SearchResultItem{
			PageID: node.ID,
			Title:  htmlutil.EscapeText(node.Title),
			Path:   dto.BuildPathFromNode(node),
			Kind:   string(node.Kind),
			Rank:   1,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Title == items[j].Title {
			return items[i].Path < items[j].Path
		}
		return items[i].Title < items[j].Title
	})

	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}

	count := len(items)
	if offset > count {
		offset = count
	}
	end := offset + limit
	if end > count {
		end = count
	}
	pagedItems := items[offset:end]
	displayedIDs := make([]string, len(pagedItems))
	for i, item := range pagedItems {
		displayedIDs[i] = item.PageID
	}
	excerpts, err := uc.tags.GetExcerptsForPages(displayedIDs)
	if err != nil {
		return nil, err
	}
	for i := range pagedItems {
		pagedItems[i].Excerpt = excerpts[pagedItems[i].PageID]
	}
	uc.attachTags(pagedItems)

	return &SearchOutput{
		Result: &coresearch.SearchResult{
			Count:     count,
			Items:     pagedItems,
			Offset:    offset,
			Limit:     limit,
			TagFacets: uc.buildTagFacets(pageIDs),
		},
	}, nil
}

func (uc *SearchUseCase) attachTags(items []coresearch.SearchResultItem) {
	if uc.tags == nil || len(items) == 0 {
		return
	}

	pageIDs := make([]string, 0, len(items))
	for _, item := range items {
		if item.PageID != "" {
			pageIDs = append(pageIDs, item.PageID)
		}
	}
	if len(pageIDs) == 0 {
		return
	}

	tagsByPage, err := uc.tags.GetTagsForPages(pageIDs)
	if err != nil {
		return
	}

	for i := range items {
		if tags, ok := tagsByPage[items[i].PageID]; ok {
			items[i].Tags = tags
		} else {
			items[i].Tags = []string{}
		}
	}
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func (uc *SearchUseCase) buildTagFacets(pageIDs []string) []coresearch.SearchTagFacet {
	if uc.tags == nil || len(pageIDs) == 0 {
		return []coresearch.SearchTagFacet{}
	}

	tagsByPage, err := uc.tags.GetTagsForPages(pageIDs)
	if err != nil {
		return []coresearch.SearchTagFacet{}
	}

	counts := make(map[string]int)
	for _, pageID := range pageIDs {
		for _, tag := range tagsByPage[pageID] {
			counts[tag]++
		}
	}

	facets := make([]coresearch.SearchTagFacet, 0, len(counts))
	for tag, count := range counts {
		facets = append(facets, coresearch.SearchTagFacet{
			Tag:   tag,
			Count: count,
		})
	}

	sort.Slice(facets, func(i, j int) bool {
		if facets[i].Count == facets[j].Count {
			return facets[i].Tag < facets[j].Tag
		}
		return facets[i].Count > facets[j].Count
	})

	return facets
}

// ─── GetIndexingStatusUseCase ────────────────────────────────────────────────

type GetIndexingStatusOutput struct {
	Status *coresearch.IndexingStatus
}

type GetIndexingStatusUseCase struct {
	status *coresearch.IndexingStatus
}

func NewGetIndexingStatusUseCase(s *coresearch.IndexingStatus) *GetIndexingStatusUseCase {
	return &GetIndexingStatusUseCase{status: s}
}

func (uc *GetIndexingStatusUseCase) Execute(_ context.Context) *GetIndexingStatusOutput {
	if uc.status == nil {
		return &GetIndexingStatusOutput{Status: nil}
	}
	return &GetIndexingStatusOutput{Status: uc.status.Snapshot()}
}
