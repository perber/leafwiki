package search

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/tree"
)

// searchCalls runs Search and SearchPageIDs the same way, so each context
// test covers both statements on the search path.
var searchCalls = []struct {
	name string
	run  func(ctx context.Context, index *SQLiteIndex, query string) (hits int, err error)
}{
	{name: "Search", run: func(ctx context.Context, index *SQLiteIndex, query string) (int, error) {
		result, err := index.Search(ctx, query, nil, 0, 10)
		if result == nil {
			return 0, err
		}
		return len(result.Items), err
	}},
	{name: "SearchPageIDs", run: func(ctx context.Context, index *SQLiteIndex, query string) (int, error) {
		ids, err := index.SearchPageIDs(ctx, query, nil)
		return len(ids), err
	}},
}

// heavyQueryPages and heavyQueryTerms size a corpus and a query whose
// uninterrupted Search and SearchPageIDs each take seconds (SearchPageIDs,
// the cheaper one, about 5s on a developer laptop), while building the corpus
// takes a fraction of a second. The query is far over the use case's term
// cap; the index itself has no cap, which is what lets one statement run
// long enough to be interrupted.
const (
	heavyQueryPages = 50
	heavyQueryTerms = 16000
)

// newHeavyQueryIndex fills the index with heavyQueryPages pages of 1000
// words each and returns a query that matches every page through
// heavyQueryTerms OR'ed terms. The rows are written with one INSERT ...
// SELECT, because indexing them through IndexPages (Markdown parsing per
// page) would take longer than the queries the test interrupts.
func newHeavyQueryIndex(t *testing.T) (*SQLiteIndex, string) {
	t.Helper()
	index := newTestIndex(t)
	err := index.withDB(func(db *sql.DB) error {
		_, err := db.Exec(fmt.Sprintf(`
			WITH RECURSIVE
				p(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM p WHERE i < %d),
				w(j) AS (SELECT 1 UNION ALL SELECT j + 1 FROM w WHERE j < 1000)
			INSERT INTO pages (path, filepath, pageID, kind, title, headings, content)
			SELECT 'p' || i, 'p' || i || '.md', 'p' || i, 'page', 'title ' || i, '',
				(SELECT group_concat('w' || ((i * 31 + j * 17) %% 5000), ' ') FROM w)
			FROM p`, heavyQueryPages))
		return err
	})
	if err != nil {
		t.Fatalf("fill heavy corpus: %v", err)
	}
	terms := make([]string, heavyQueryTerms)
	for i := range terms {
		terms[i] = fmt.Sprintf("w%d", i)
	}
	return index, strings.Join(terms, " OR ")
}

// assertIndexStillSearchable fails unless the index has no connection left
// in use and a plain search on it still finds something.
func assertIndexStillSearchable(t *testing.T, index *SQLiteIndex, query string) {
	t.Helper()
	var inUse int
	_ = index.withDBRead(func(db *sql.DB) error {
		inUse = db.Stats().InUse
		return nil
	})
	if inUse != 0 {
		t.Errorf("connections in use after the canceled query = %d, want 0", inUse)
	}
	for _, call := range searchCalls {
		hits, err := call.run(t.Context(), index, query)
		if err != nil {
			t.Fatalf("%s after the canceled query: %v", call.name, err)
		}
		if hits == 0 {
			t.Errorf("%s after the canceled query found nothing for %q", call.name, query)
		}
	}
}

func TestSQLiteIndex_Search_CanceledContext_ReturnsCanceledWithoutResults(t *testing.T) {
	index := newTestIndex(t)
	if err := index.IndexPage("docs/a", "docs/a.md", "a", "Alpha", tree.NodeKindPage, "searchable content"); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	for _, call := range searchCalls {
		t.Run(call.name, func(t *testing.T) {
			hits, err := call.run(ctx, index, "searchable")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if hits != 0 {
				t.Errorf("hits = %d, want none", hits)
			}
		})
	}
	assertIndexStillSearchable(t, index, "searchable")
}

// TestSQLiteIndex_Search_DeadlineMidQuery_InterruptsAndKeepsIndexUsable pins
// that a deadline stops a running statement (the driver calls
// sqlite3_interrupt) instead of only being noticed once the statement is
// done. The deadline error comes back either way; only the elapsed time tells
// an interrupted statement from one that ran to the end, so the bound is far
// below the uninterrupted run and far above an interrupted one. A machine
// fast enough to finish the query within the bound makes the test pass
// without proving the interrupt, never fail.
func TestSQLiteIndex_Search_DeadlineMidQuery_InterruptsAndKeepsIndexUsable(t *testing.T) {
	index, heavyQuery := newHeavyQueryIndex(t)
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	const deadline = 20 * time.Millisecond
	const bound = 2 * time.Second
	for _, call := range searchCalls {
		t.Run(call.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), deadline)
			defer cancel()

			start := time.Now()
			hits, err := call.run(ctx, index, heavyQuery)
			elapsed := time.Since(start)

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want context.DeadlineExceeded", err)
			}
			if hits != 0 {
				t.Errorf("hits = %d, want none", hits)
			}
			if elapsed > bound {
				t.Errorf("returned after %v, want within %v of a %v deadline (statement not interrupted)", elapsed, bound, deadline)
			}
		})
	}
	assertIndexStillSearchable(t, index, "title")
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("an interrupted search logged an error:\n%s", logs.String())
	}
}
