package search

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
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

// TestSQLiteIndex_Search_RepeatedInterrupts_KeepReadersAndWriterHealthy pins
// that interrupting statements does not poison the pool: after many
// interrupted queries in a row, and many contexts that end right after a
// query has returned, a write and concurrent reads on the same index all
// succeed, no SQLITE_INTERRUPT surfaces on an unrelated statement, and no
// connection is left in use.
func TestSQLiteIndex_Search_RepeatedInterrupts_KeepReadersAndWriterHealthy(t *testing.T) {
	index, heavyQuery := newHeavyQueryIndex(t)

	for i := range 10 {
		call := searchCalls[i%len(searchCalls)]
		ctx, cancel := context.WithTimeout(t.Context(), time.Duration(1+i)*time.Millisecond)
		_, err := call.run(ctx, index, heavyQuery)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("interrupt %d (%s): err = %v, want context.DeadlineExceeded", i, call.name, err)
		}
	}

	// A context that ends just after the statement has returned must not
	// interrupt the next statement that gets the same connection.
	for i := range 100 {
		call := searchCalls[i%len(searchCalls)]
		ctx, cancel := context.WithCancel(t.Context())
		_, err := call.run(ctx, index, "title")
		cancel()
		if err != nil {
			t.Fatalf("search %d (%s) with a context canceled after it: %v", i, call.name, err)
		}
	}

	if err := index.IndexPage("fresh", "fresh.md", "fresh", "Fresh", tree.NodeKindPage, "freshword body"); err != nil {
		t.Fatalf("IndexPage right after interrupted searches: %v", err)
	}

	const readers, rounds = 8, 10
	errs := make(chan error, readers*rounds*len(searchCalls)+rounds)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range rounds {
			id := fmt.Sprintf("w%d", i)
			if err := index.IndexPage(id, id+".md", id, "Writer "+id, tree.NodeKindPage, "freshword writer"); err != nil {
				errs <- fmt.Errorf("concurrent IndexPage %d: %w", i, err)
			}
		}
	}()
	var wg sync.WaitGroup
	for r := range readers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range rounds {
				for _, call := range searchCalls {
					hits, err := call.run(t.Context(), index, "freshword")
					if err != nil {
						errs <- fmt.Errorf("reader %d round %d %s: %w", r, i, call.name, err)
					} else if hits == 0 {
						errs <- fmt.Errorf("reader %d round %d %s found nothing", r, i, call.name)
					}
				}
			}
		}()
	}
	wg.Wait()
	<-done
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	assertIndexStillSearchable(t, index, "freshword")
}
