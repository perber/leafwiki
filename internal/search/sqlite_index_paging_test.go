package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/perber/wiki/internal/core/excerpt"
	"github.com/perber/wiki/internal/core/tree"
)

// searchOracle is the single-statement Search from before rank-then-render:
// highlight() and snippet() for every match, one ORDER BY, LIMIT/OFFSET.
// rowid is appended to the ORDER BY, the tie-break the two-statement Search
// promises, so both must return the same items in the same order.
func searchOracle(t *testing.T, index *SQLiteIndex, query string, pageIDs []string, offset, limit int) *SearchResult {
	t.Helper()
	parsed := parseSearchQuery(strings.TrimSpace(query))
	ftsQuery := parsed.fts
	if (len(pageIDs) == 0 && pageIDs != nil) || (ftsQuery == "" && len(pageIDs) == 0) {
		return &SearchResult{Items: []SearchResultItem{}, Offset: offset, Limit: limit, TagFacets: []SearchTagFacet{}}
	}
	sr := &SearchResult{TagFacets: []SearchTagFacet{}, Offset: offset, Limit: limit, Items: []SearchResultItem{}}
	err := index.withDBRead(func(db *sql.DB) error {
		whereClause, whereArgs := buildSearchWhereClause(ftsQuery, pageIDs)
		if err := db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM pages WHERE %s;`, whereClause), whereArgs...).Scan(&sr.Count); err != nil {
			return err
		}
		orderBy, orderArgs := searchOrderBy(parsed)
		if ftsQuery != "" {
			orderBy += ", rowid"
		}
		q := fmt.Sprintf(`SELECT pageID, path, kind, %s, %s, content, %s AS bm25_score
			FROM pages WHERE %s ORDER BY %s LIMIT ? OFFSET ?;`,
			searchTitleExpr(ftsQuery != ""), searchExcerptExpr(ftsQuery != ""), searchRankExpr(ftsQuery != ""),
			whereClause, orderBy)
		args := append(append(append([]interface{}{}, whereArgs...), orderArgs...), limit, offset)
		rows, err := db.Query(q, args...)
		if err != nil {
			return err
		}
		defer closeRows(rows)
		for rows.Next() {
			var r SearchResultItem
			var score float64
			var content string
			if err := rows.Scan(&r.PageID, &r.Path, &r.Kind, &r.Title, &r.Excerpt, &content, &score); err != nil {
				return err
			}
			r.Title = sanitizeSearchTitle(r.Title)
			if strings.TrimSpace(r.Excerpt) == "" {
				r.Excerpt = excerpt.FromBody(content)
			}
			if ftsQuery == "" {
				r.Rank = 1
			} else {
				r.Rank = rankFromBM25(score)
			}
			sr.Items = append(sr.Items, r)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("oracle(%q): %v", query, err)
	}
	return sr
}

// newPagingCorpus indexes pages built to stress paging: identical twins with
// equal bm25 (ties across page boundaries), title-only and body-only hits,
// pages with an empty body (snippet falls back to excerpt.FromBody), Unicode,
// and prefix-only matches for the tier ORDER BY.
func newPagingCorpus(t *testing.T) (*SQLiteIndex, []string) {
	t.Helper()
	index := newTestIndex(t)
	var ids []string
	add := func(id, title, body string) {
		t.Helper()
		if err := index.IndexPage("docs/"+id, "docs/"+id+".md", id, title, tree.NodeKindPage, body); err != nil {
			t.Fatalf("IndexPage(%s): %v", id, err)
		}
		ids = append(ids, id)
	}
	for i := 0; i < 12; i++ {
		add(fmt.Sprintf("twin%02d", i), "Twin", "alpha beta gamma shared words")
	}
	for i := 0; i < 5; i++ {
		add(fmt.Sprintf("alpha%d", i), fmt.Sprintf("Alpha %d", i), strings.Repeat("filler ", i*10)+"alpha near beta here")
	}
	add("titleonly", "Alpha Beta", "")
	add("emptybody", "Gamma", "")
	add("prefix", "Alphabet", "alphanumeric soup")
	add("unicode", "Über Café 日本語", "naïve café über alpha 日本語")
	add("long", "Long", strings.Repeat("lorem ipsum dolor ", 3000)+"alpha")
	return index, ids
}

func TestSQLiteIndex_Search_MatchesSingleStatementOracle(t *testing.T) {
	index, ids := newPagingCorpus(t)
	queries := []string{
		"alpha", "alpha beta", "alph", "twin", "gamma", "beta OR gamma", "alpha NOT twin",
		"title:alpha", "NEAR(alpha beta, 3)", "\"shared words\"", "café", "über", "日本語",
		"alpha*", ",", "nomatch",
	}
	filters := map[string][]string{
		"nil":    nil,
		"subset": {"twin03", "twin07", "alpha2", "titleonly", "emptybody", "unicode", "missing"},
		"one":    {"twin11"},
		"all":    ids,
	}
	pagings := []struct{ offset, limit int }{
		{0, 1}, {0, 3}, {3, 3}, {5, 4}, {10, 5}, {11, 2}, {0, 100},
		{22, 10}, {23, 10}, {1000, 10}, // at and beyond the end
		{0, 0}, {4, 0}, // limit 0
		{0, -1}, {7, -1}, // negative limit: SQLite treats it as no limit
	}
	for _, q := range queries {
		t.Run(q, func(t *testing.T) {
			for fname, filter := range filters {
				for _, p := range pagings {
					name := fmt.Sprintf("%s/%d+%d", fname, p.offset, p.limit)
					got, err := index.Search(t.Context(), q, filter, p.offset, p.limit)
					if err != nil {
						t.Errorf("%s: Search: %v", name, err)
						continue
					}
					want := searchOracle(t, index, q, filter, p.offset, p.limit)
					if got.Count != want.Count {
						t.Errorf("%s: count %d, want %d", name, got.Count, want.Count)
					}
					if got.Items == nil {
						t.Errorf("%s: Items is nil, want [] for JSON", name)
					}
					if !reflect.DeepEqual(got.Items, want.Items) {
						t.Errorf("%s: items differ\n got  %s\n want %s", name, itemIDs(got.Items), itemIDs(want.Items))
					}
				}
			}
		})
	}
}

// Walking all pages of a result with ties on bm25 must list every match
// exactly once, for every page size.
func TestSQLiteIndex_Search_PagingOverTiesListsEveryMatchOnce(t *testing.T) {
	index, _ := newPagingCorpus(t)
	for _, q := range []string{"twin", "alpha", "beta OR gamma"} {
		all, err := index.Search(t.Context(), q, nil, 0, -1)
		if err != nil {
			t.Fatalf("Search(%q, all): %v", q, err)
		}
		if len(all.Items) != all.Count {
			t.Fatalf("Search(%q, limit -1): %d items, count %d", q, len(all.Items), all.Count)
		}
		for size := 1; size <= 5; size++ {
			var walked []SearchResultItem
			for offset := 0; offset < all.Count; offset += size {
				page, err := index.Search(t.Context(), q, nil, offset, size)
				if err != nil {
					t.Fatalf("Search(%q, %d+%d): %v", q, offset, size, err)
				}
				walked = append(walked, page.Items...)
			}
			if !reflect.DeepEqual(walked, all.Items) {
				t.Errorf("%q page size %d: walked %s, want %s", q, size, itemIDs(walked), itemIDs(all.Items))
			}
		}
	}
}

// A page whose snippet is empty (the hit is in the title, the body is empty
// or the snippet is only whitespace) gets excerpt.FromBody of its content,
// on later result pages too.
func TestSQLiteIndex_Search_EmptySnippetFallsBackToBodyExcerpt(t *testing.T) {
	index := newTestIndex(t)
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("p%d", i)
		if err := index.IndexPage(id, id+".md", id, "Zebra "+id, tree.NodeKindPage, ""); err != nil {
			t.Fatalf("IndexPage: %v", err)
		}
	}
	for offset := 0; offset < 4; offset++ {
		got, err := index.Search(t.Context(), "zebra", nil, offset, 1)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(got.Items) != 1 {
			t.Fatalf("offset %d: %d items, want 1", offset, len(got.Items))
		}
		if got.Items[0].Excerpt != excerpt.FromBody("") {
			t.Errorf("offset %d: excerpt %q, want the FromBody fallback %q", offset, got.Items[0].Excerpt, excerpt.FromBody(""))
		}
		if !strings.HasPrefix(got.Items[0].Title, "<b>Zebra</b>") {
			t.Errorf("offset %d: title %q not highlighted", offset, got.Items[0].Title)
		}
	}
}

// countingCtx ends itself at the n-th Done call, so successive values of n
// cancel Search at every point where database/sql looks at the context,
// including between the rank and the render statement.
type countingCtx struct {
	context.Context
	cancel context.CancelFunc
	n      int64
	calls  atomic.Int64
}

func (c *countingCtx) Done() <-chan struct{} {
	if c.calls.Add(1) >= c.n {
		c.cancel()
	}
	return c.Context.Done()
}

func TestSQLiteIndex_Search_CancelAtAnyPointReturnsAllOrNothing(t *testing.T) {
	index, _ := newPagingCorpus(t)
	want, err := index.Search(t.Context(), "alpha", nil, 2, 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	completed := false
	for n := int64(1); n <= 200 && !completed; n++ {
		parent, cancel := context.WithCancel(t.Context())
		ctx := &countingCtx{Context: parent, cancel: cancel, n: n}
		got, err := index.Search(ctx, "alpha", nil, 2, 5)
		cancel()
		switch {
		case err == nil:
			if !reflect.DeepEqual(got, want) {
				t.Errorf("cancel at Done call %d: incomplete result without error: %s, want %s", n, itemIDs(got.Items), itemIDs(want.Items))
			}
			completed = ctx.calls.Load() < n
		case errors.Is(err, context.Canceled):
			if got != nil {
				t.Errorf("cancel at Done call %d: got a result %+v with the error", n, got)
			}
		default:
			t.Errorf("cancel at Done call %d: err = %v, want context.Canceled or nil", n, err)
		}
	}
	if !completed {
		t.Errorf("Search never completed without reaching the cancelling Done call")
	}
	assertIndexStillSearchable(t, index, "alpha")
}

// Writes interleaved with searches never make a page shorter than Count
// promises: Count, rank and render all see the same snapshot.
func TestSQLiteIndex_Search_ConcurrentWritesKeepPagesConsistent(t *testing.T) {
	index := newTestIndex(t)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("base%d", i)
		if err := index.IndexPage(id, id+".md", id, "Base "+id, tree.NodeKindPage, "churn content"); err != nil {
			t.Fatalf("IndexPage: %v", err)
		}
	}
	stop := make(chan struct{})
	var writer sync.WaitGroup
	writer.Add(1)
	go func() {
		defer writer.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			id := fmt.Sprintf("base%d", i%30)
			if i%2 == 0 {
				_ = index.RemovePage(id)
			} else {
				_ = index.IndexPage(id, id+".md", id, "Base "+id, tree.NodeKindPage, "churn content")
			}
		}
	}()
	var readers sync.WaitGroup
	errs := make(chan error, 1000)
	for r := 0; r < 6; r++ {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for i := 0; i < 40; i++ {
				offset, limit := (i*3)%30, 7
				got, err := index.Search(context.Background(), "churn", nil, offset, limit)
				if err != nil {
					errs <- err
					continue
				}
				want := min(limit, max(got.Count-offset, 0))
				if len(got.Items) != want {
					errs <- fmt.Errorf("offset %d: %d items with count %d, want %d", offset, len(got.Items), got.Count, want)
				}
				for _, it := range got.Items {
					if !strings.Contains(it.Excerpt, "<b>churn</b>") {
						errs <- fmt.Errorf("item %s not rendered: %q", it.PageID, it.Excerpt)
					}
				}
			}
		}()
	}
	readers.Wait()
	close(stop)
	writer.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// A negative limit returns every match, which renders every rowid in one
// IN list. More matches than SQLite's bound-variable limit must still work.
func TestSQLiteIndex_Search_NegativeLimitWithManyMatches_ReturnsEveryMatch(t *testing.T) {
	index := newTestIndex(t)
	const pages = 33000
	err := index.withDB(func(db *sql.DB) error {
		_, err := db.Exec(fmt.Sprintf(`
			WITH RECURSIVE p(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM p WHERE i < %d)
			INSERT INTO pages (path, filepath, pageID, kind, title, headings, content)
			SELECT 'p' || i, 'p' || i || '.md', 'p' || i, 'page', 'many ' || i, '', 'many words'
			FROM p`, pages))
		return err
	})
	if err != nil {
		t.Fatalf("fill: %v", err)
	}
	got, err := index.Search(t.Context(), "many", nil, 0, -1)
	if err != nil {
		t.Fatalf("Search(limit -1) over %d matches: %v", pages, err)
	}
	if got.Count != pages || len(got.Items) != pages {
		t.Errorf("count %d, items %d, want %d", got.Count, len(got.Items), pages)
	}
}

func itemIDs(items []SearchResultItem) string {
	ids := make([]string, len(items))
	for i, it := range items {
		ids[i] = it.PageID
	}
	return "[" + strings.Join(ids, " ") + "]"
}
