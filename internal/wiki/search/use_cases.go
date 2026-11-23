package search

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"
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

// searchTimeout bounds the index queries of one search. The query caps
// bound a query's cost in terms, not in time; on a large wiki even a capped
// query can run long, and without a deadline it holds a connection and the
// index's read lock until it is done. A var so tests can shorten it. The
// 503 description of handleSearch in routes.go names this value; keep them
// in sync.
var searchTimeout = 10 * time.Second

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
	log   *slog.Logger
}

func NewSearchUseCase(idx *coresearch.SQLiteIndex, tags *coretags.TagsService, tree *tree.TreeService, log *slog.Logger) *SearchUseCase {
	return &SearchUseCase{index: idx, tags: tags, tree: tree, log: log.With("component", "SearchUseCase")}
}

// Paging bounds for a search. The text index treats a negative limit as "all
// rows", so without an upper bound one request could rank, excerpt and
// serialize every page of the wiki. A huge offset is capped as well: SQLite
// can only keep the top offset+limit rows while sorting, so an unbounded
// offset makes it hold every matching row, content included.
const (
	searchDefaultLimit = 20
	searchMaxLimit     = 100
	searchMaxOffset    = 10_000
)

// boundPaging turns a limit below 1 into the default, caps it at
// searchMaxLimit, turns a negative offset into 0 and caps the offset at
// searchMaxOffset, the same substitute-don't-reject shape GET /api/tags uses.
func boundPaging(offset, limit int) (int, int) {
	if offset < 0 {
		offset = 0
	}
	if offset > searchMaxOffset {
		offset = searchMaxOffset
	}
	if limit < 1 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}
	return offset, limit
}

func (uc *SearchUseCase) Execute(ctx context.Context, in SearchInput) (*SearchOutput, error) {
	query := strings.TrimSpace(in.Query)
	// Parsed once here and reused below (SearchWithFacetIDs), instead of
	// QueryTermCount, Search and SearchPageIDs each re-deriving the same
	// FTS5 expression from the raw string independently.
	parsed := coresearch.ParseQuery(query)
	if utf8.RuneCountInString(query) > maxQueryRunes || parsed.TermCount() > maxQueryTerms {
		return nil, ErrSearchQueryTooLong
	}

	if uc.index == nil {
		return nil, ErrSearchUnavailable
	}
	in.Offset, in.Limit = boundPaging(in.Offset, in.Limit)

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

	ctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	// Search and SearchPageIDs are independent reads over the same parsed
	// query; SearchWithFacetIDs runs them concurrently instead of one after
	// the other.
	result, fullMatchPageIDs, err := uc.index.SearchWithFacetIDs(ctx, parsed, pageIDs, in.Offset, in.Limit)
	if err != nil {
		return nil, uc.searchQueryError(ctx, err, query, parsed)
	}

	uc.attachTags(result.Items)
	result.TagFacets = uc.buildTagFacets(fullMatchPageIDs)
	return &SearchOutput{Result: result}, nil
}

// searchQueryError maps an index error once ctx has ended: a timeout becomes
// ErrSearchUnavailable and is logged here, where it stops propagating; the
// log has the query's size, never its text, which may be private. A canceled
// ctx (the client went away) comes back as context.Canceled and is not
// logged; the handler answers it without a body. Checking ctx and not only
// err also catches a driver error that doesn't wrap the context's error.
// parsed is the same ParsedQuery Execute already built — its term count is
// reused for the log line rather than re-parsing the query again here.
func (uc *SearchUseCase) searchQueryError(ctx context.Context, err error, query string, parsed coresearch.ParsedQuery) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded):
		uc.log.Warn("search timed out",
			"queryRunes", utf8.RuneCountInString(query),
			"terms", parsed.TermCount())
		return ErrSearchUnavailable
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		return err
	}
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

	// Execute already bounds the paging; this keeps direct callers safe too,
	// since items[offset:end] below panics on a negative offset.
	offset, limit = boundPaging(offset, limit)
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
