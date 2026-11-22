package search

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func TestSQLiteIndex_IndexPage(t *testing.T) {
	index := newTestIndex(t)

	// Testdata
	path := "docs/test.md"
	pageID := "test123"
	title := "Test Page"
	content := "This is a **test** page."
	expectedContent := "This is a test page."

	err := index.IndexPage(path, path, pageID, title, tree.NodeKindPage, content)
	if err != nil {
		t.Fatalf("IndexPage failed: %v", err)
	}

	var row *sql.Row

	if err := index.withDB(func(db *sql.DB) error {
		row = db.QueryRow(`SELECT path, title, content FROM pages WHERE pageID = ?`, pageID)
		if row == nil {
			t.Fatalf("no data found for pageID %s", pageID)
		}
		return nil
	}); err != nil {
		t.Fatalf("failed to read indexed data: %v", err)
	}

	var gotPath, gotTitle, gotContent string
	err = row.Scan(&gotPath, &gotTitle, &gotContent)
	if err != nil {
		t.Fatalf("failed to read indexed data: %v", err)
	}

	// Assertions
	if gotPath != path {
		t.Errorf("expected path %s, got %s", path, gotPath)
	}
	if gotTitle != title {
		t.Errorf("expected title %s, got %s", title, gotTitle)
	}
	if !strings.HasPrefix(gotContent, expectedContent) {
		t.Errorf("expected content '%s', got '%s'", expectedContent, gotContent)
	}
}

func TestSQLiteIndex_UsesWALJournalMode(t *testing.T) {
	index := newTestIndex(t)

	var mode string
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`PRAGMA journal_mode`).Scan(&mode)
	}); err != nil {
		t.Fatalf("failed to read journal_mode: %v", err)
	}

	if !strings.EqualFold(mode, "wal") {
		t.Fatalf("journal_mode = %q, want %q", mode, "wal")
	}
}

func TestSQLiteIndex_IndexPage_ReindexingExistingPageProducesExactlyOneRow(t *testing.T) {
	index := newTestIndex(t)

	path := "docs/test.md"
	pageID := "test123"

	if err := index.IndexPage(path, path, pageID, "Test Page", tree.NodeKindPage, "first version"); err != nil {
		t.Fatalf("first IndexPage failed: %v", err)
	}
	if err := index.IndexPage(path, path, pageID, "Test Page", tree.NodeKindPage, "second version"); err != nil {
		t.Fatalf("second IndexPage failed: %v", err)
	}

	var count int
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT COUNT(*) FROM pages WHERE pageID = ?`, pageID).Scan(&count)
	}); err != nil {
		t.Fatalf("failed to count rows: %v", err)
	}

	if count != 1 {
		t.Fatalf("row count for pageID %s = %d, want 1 (re-indexing must replace, not duplicate, within one transaction)", pageID, count)
	}
}

func TestSQLiteIndex_RemovePages_RemovesAllGivenIDs(t *testing.T) {
	index := newTestIndex(t)

	ids := []string{"page-a", "page-b", "page-c"}
	for _, id := range ids {
		if err := index.IndexPage(id, id, id, "Title "+id, tree.NodeKindPage, "content"); err != nil {
			t.Fatalf("IndexPage(%s) failed: %v", id, err)
		}
	}

	if err := index.RemovePages([]string{"page-a", "page-c"}); err != nil {
		t.Fatalf("RemovePages failed: %v", err)
	}

	var count int
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT COUNT(*) FROM pages WHERE pageID IN ('page-a', 'page-c')`).Scan(&count)
	}); err != nil {
		t.Fatalf("failed to count removed rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected page-a and page-c to be removed, %d rows remain", count)
	}

	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT COUNT(*) FROM pages WHERE pageID = 'page-b'`).Scan(&count)
	}); err != nil {
		t.Fatalf("failed to count remaining rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected page-b to remain, got count=%d", count)
	}
}

func TestSQLiteIndex_IndexPages_IndexesAllGivenInputs(t *testing.T) {
	index := newTestIndex(t)

	inputs := make([]IndexPageInput, 0, 5)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("page-%d", i)
		inputs = append(inputs, IndexPageInput{
			Path: id, FilePath: id + ".md", PageID: id, Title: "Title " + id, Kind: tree.NodeKindPage, Raw: "body " + id,
		})
	}

	failures, err := index.IndexPages(inputs)
	if err != nil {
		t.Fatalf("IndexPages failed: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}

	for _, in := range inputs {
		var gotTitle string
		if err := index.withDB(func(db *sql.DB) error {
			return db.QueryRow(`SELECT title FROM pages WHERE pageID = ?`, in.PageID).Scan(&gotTitle)
		}); err != nil {
			t.Fatalf("failed to read back %s: %v", in.PageID, err)
		}
		if gotTitle != in.Title {
			t.Errorf("pageID %s: title = %q, want %q", in.PageID, gotTitle, in.Title)
		}
	}
}

func TestSQLiteIndex_IndexPages_SkipsUnparsableInputOthersStillCommit(t *testing.T) {
	index := newTestIndex(t)

	// Properly fenced (so it's recognized as frontmatter at all) but
	// invalid YAML inside (unclosed flow sequence) that markdown's
	// sanitize-and-retry fallback doesn't know how to fix — confirmed by
	// reading internal/core/markdown/frontmatter.go's
	// sanitizeFrontmatterYAML, which only special-cases template
	// placeholders and unquoted colons, neither of which apply here.
	const badFrontmatter = "---\nleafwiki_title: [unclosed\n---\nbody\n"
	inputs := []IndexPageInput{
		{Path: "good-1", FilePath: "good-1.md", PageID: "good-1", Title: "Good 1", Kind: tree.NodeKindPage, Raw: "fine content"},
		{Path: "bad", FilePath: "bad.md", PageID: "bad", Title: "Bad", Kind: tree.NodeKindPage, Raw: badFrontmatter},
		{Path: "good-2", FilePath: "good-2.md", PageID: "good-2", Title: "Good 2", Kind: tree.NodeKindPage, Raw: "fine content too"},
	}

	failures, err := index.IndexPages(inputs)
	if err != nil {
		t.Fatalf("IndexPages returned a transaction-level error, want the batch to still commit for the valid inputs: %v", err)
	}
	if len(failures) != 1 || failures[0].PageID != "bad" {
		t.Fatalf("failures = %v, want exactly one failure for pageID \"bad\"", failures)
	}

	for _, id := range []string{"good-1", "good-2"} {
		var count int
		if err := index.withDB(func(db *sql.DB) error {
			return db.QueryRow(`SELECT COUNT(*) FROM pages WHERE pageID = ?`, id).Scan(&count)
		}); err != nil {
			t.Fatalf("failed to count %s: %v", id, err)
		}
		if count != 1 {
			t.Errorf("expected %s to be indexed despite the other input failing, count=%d", id, count)
		}
	}

	var badCount int
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT COUNT(*) FROM pages WHERE pageID = 'bad'`).Scan(&badCount)
	}); err != nil {
		t.Fatalf("failed to count bad: %v", err)
	}
	if badCount != 0 {
		t.Fatalf("expected the unparsable input to be skipped, not indexed")
	}
}

// Pins the RWMutex change (mu sync.Mutex -> sync.RWMutex, Search/
// SearchPageIDs/Ping moved to withDBRead): concurrent Search calls must
// run without data races or errors. Run with `go test -race` to actually
// catch a regression (mirrors
// internal/links/links_store_test.go's TestLinksStore_ConcurrentReadsDoNotRace).
func TestSQLiteIndex_ConcurrentSearchesDoNotRace(t *testing.T) {
	index := newTestIndex(t)

	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("page-%d", i)
		if err := index.IndexPage(id, id, id, "Title "+id, tree.NodeKindPage, "searchable content"); err != nil {
			t.Fatalf("IndexPage(%s) failed: %v", id, err)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 40)
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := index.Search("searchable", nil, 0, 10); err != nil {
				errCh <- err
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := index.SearchPageIDs("searchable", nil); err != nil {
				errCh <- err
			}
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent search failed: %v", err)
	}
}

func TestSearchIndexDatabasePath_WindowsPath(t *testing.T) {
	got := strings.ReplaceAll(searchIndexDatabasePath(`C:\wiki\data`, "search.db"), `\`, `/`)
	want := `C:/wiki/data/search.db`
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestSQLiteIndex_CreatesDatabaseInStorageDir(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	if _, err := os.Stat(filepath.Join(tmpDir, "search.db")); err != nil {
		t.Fatalf("expected search.db in storage dir, got err: %v", err)
	}
}

func TestSQLiteIndex_Search(t *testing.T) {
	index := newTestIndex(t)

	// Index two pages
	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha Search Test", tree.NodeKindPage, "This content is about SQLite search.")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	err = index.IndexPage("notes/beta", "notes/beta.md", "beta2", "Unrelated Page", tree.NodeKindSection, "This content is not about the search term.")
	if err != nil {
		t.Fatalf("failed to index beta page: %v", err)
	}

	// Perform search
	result, err := index.Search("content:search*", nil, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Assertions
	if result.Count != 2 {
		t.Errorf("expected 2 result, got %d", result.Count)
	}

	if len(result.Items) != 2 {
		t.Fatalf("expected 2 result item, got %d", len(result.Items))
	}

	if result.Items[0].PageID != "alpha1" {
		t.Errorf("expected alpha1 to be ranked first, got %s", result.Items[0].PageID)
	}

	if result.Items[0].Kind != string(tree.NodeKindPage) {
		t.Errorf("expected first result kind %q, got %q", tree.NodeKindPage, result.Items[0].Kind)
	}

	if result.Items[1].Kind != string(tree.NodeKindSection) {
		t.Errorf("expected second result kind %q, got %q", tree.NodeKindSection, result.Items[1].Kind)
	}

	if !strings.Contains(result.Items[0].Excerpt, "<b>") {
		t.Errorf("expected highlighted search snippet, got %q", result.Items[0].Excerpt)
	}
}

// A text query without a match returns an empty, non-nil Items so the API
// serializes "items": [] and not "items": null.
func TestSQLiteIndex_Search_NoMatch_ReturnsEmptyItems(t *testing.T) {
	index, err := NewSQLiteIndex(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	if err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "Some text about databases."); err != nil {
		t.Fatalf("failed to index page: %v", err)
	}

	result, err := index.Search("zebra", nil, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Count != 0 {
		t.Errorf("expected count 0, got %d", result.Count)
	}
	if result.Items == nil || len(result.Items) != 0 {
		t.Errorf("expected non-nil empty Items, got %#v", result.Items)
	}
}

// TestSQLiteIndex_Search_RanksFieldMatchAboveContentOnlyMatch checks the
// bm25 column weights: a hit in the title or a heading outranks a hit only in
// the body text. "search" becomes "search"*, which matches both pages.
func TestSQLiteIndex_Search_RanksFieldMatchAboveContentOnlyMatch(t *testing.T) {
	tests := []struct {
		name         string
		fieldTitle   string
		fieldContent string
		contentTitle string
	}{
		{name: "title", fieldTitle: "Search term in title", fieldContent: "Lorem ipsum dolor sit amet.", contentTitle: "Content only match"},
		{name: "heading", fieldTitle: "No search in title", fieldContent: "## Search term in heading\n\nSome additional body text.", contentTitle: "No search in title"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			index := newTestIndex(t)
			if err := index.IndexPage("docs/fieldMatch", "docs/fieldMatch.md", "fieldMatch", tc.fieldTitle, tree.NodeKindPage, tc.fieldContent); err != nil {
				t.Fatalf("failed to index fieldMatch page: %v", err)
			}
			if err := index.IndexPage("docs/contentMatch", "docs/contentMatch.md", "contentMatch", tc.contentTitle, tree.NodeKindPage,
				"This page has the search term only in the content."); err != nil {
				t.Fatalf("failed to index contentMatch page: %v", err)
			}

			result, err := index.Search("search", nil, 0, 10)
			if err != nil {
				t.Fatalf("search failed: %v", err)
			}
			if result.Count != 2 {
				t.Fatalf("expected 2 results, got %d", result.Count)
			}
			if len(result.Items) != 2 {
				t.Fatalf("expected 2 result items, got %d", len(result.Items))
			}
			if result.Items[0].PageID != "fieldMatch" {
				t.Errorf("expected the %s match to be ranked first, got %s", tc.name, result.Items[0].PageID)
			}
			// Rank is 1/(1+score), so the better match must not have the lower rank.
			if result.Items[0].Rank < result.Items[1].Rank {
				t.Errorf("expected higher rank for the %s match (got %f, %f)", tc.name, result.Items[0].Rank, result.Items[1].Rank)
			}
			for i, item := range result.Items {
				if item.Rank <= 0 || item.Rank > 1 {
					t.Errorf("expected rank for item %d to be in (0,1], got %f", i, item.Rank)
				}
			}
		})
	}
}

func TestSQLiteIndex_Search_EscapesTitleForContentOnlyMatch(t *testing.T) {
	index := newTestIndex(t)

	maliciousTitle := `<img src=x onerror="alert(1)">`
	err := index.IndexPage(
		"docs/malicious",
		"docs/malicious.md",
		"malicious1",
		maliciousTitle,
		tree.NodeKindPage,
		"This page contains the search keyword in content only.",
	)
	if err != nil {
		t.Fatalf("failed to index malicious page: %v", err)
	}

	result, err := index.Search("keyword", nil, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	if got := result.Items[0].Title; strings.Contains(got, "<img") {
		t.Fatalf("expected title to escape raw HTML, got %q", got)
	}

	if got := result.Items[0].Title; !strings.Contains(got, "&lt;img src=x onerror=&#34;alert(1)&#34;&gt;") {
		t.Fatalf("expected escaped title, got %q", got)
	}
}

func TestSQLiteIndex_Search_PreservesBoldHighlightsWhileEscapingTitle(t *testing.T) {
	index := newTestIndex(t)

	maliciousTitle := `<script>alert(1)</script> Search`
	err := index.IndexPage(
		"docs/highlighted",
		"docs/highlighted.md",
		"highlighted1",
		maliciousTitle,
		tree.NodeKindPage,
		"Body content without additional matches.",
	)
	if err != nil {
		t.Fatalf("failed to index highlighted page: %v", err)
	}

	result, err := index.Search("search", nil, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	got := result.Items[0].Title
	if strings.Contains(got, "<script>") {
		t.Fatalf("expected title to escape script tag, got %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("expected escaped script tag in title, got %q", got)
	}
	if !strings.Contains(got, "<b>Search</b>") {
		t.Fatalf("expected preserved bold highlight, got %q", got)
	}
}

func TestSQLiteIndex_Search_EscapesTitleForFilteredResultsWithoutQuery(t *testing.T) {
	index := newTestIndex(t)

	maliciousTitle := `<svg onload=alert(1)>`
	err := index.IndexPage(
		"docs/tagged",
		"docs/tagged.md",
		"tagged1",
		maliciousTitle,
		tree.NodeKindPage,
		"Plain content.",
	)
	if err != nil {
		t.Fatalf("failed to index tagged page: %v", err)
	}

	result, err := index.Search("", []string{"tagged1"}, 0, 10)
	if err != nil {
		t.Fatalf("filtered search failed: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	got := result.Items[0].Title
	if strings.Contains(got, "<svg") {
		t.Fatalf("expected filtered result title to escape raw HTML, got %q", got)
	}
	if got != "&lt;svg onload=alert(1)&gt;" {
		t.Fatalf("expected escaped filtered title, got %q", got)
	}
}

// The excerpt is HTML that the UI splits on <b>/</b> and entity-decodes once.
// The indexed text is already escaped by excerpt.PlainTextForSearch, so the
// excerpt must come back escaped exactly once with only the hit marked.
func TestSQLiteIndex_Search_ExcerptIsEscapedOnceAndMarksOnlyTheHit(t *testing.T) {
	index := newTestIndex(t)

	// Markdown drops raw HTML outside code, so the markup sits in a code span.
	content := "keyword `<b>fake</b>` & \"q\""
	if err := index.IndexPage("docs/a", "docs/a.md", "a", "Plain", tree.NodeKindPage, content); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}

	result, err := index.Search("keyword", nil, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	want := "<b>keyword</b> &lt;b&gt;fake&lt;/b&gt; &amp; &#34;q&#34;"
	if got := result.Items[0].Excerpt; got != want {
		t.Errorf("excerpt = %q, want %q", got, want)
	}
}

// char(2)/char(3) mark the hit in title and excerpt, so the same characters
// in a page must not reach the index, or they would turn into fake <b>s.
func TestSQLiteIndex_Search_HighlightMarkersInPageTextDoNotFakeAHighlight(t *testing.T) {
	index := newTestIndex(t)

	title := "Alpha \u0002fake\u0003 keyword"
	content := "# Heading \u0002fake\u0003\n\nbody \u0002fake\u0003 keyword"
	if err := index.IndexPage("docs/a", "docs/a.md", "a", title, tree.NodeKindPage, content); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}

	result, err := index.Search("keyword", nil, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	item := result.Items[0]
	for _, field := range []struct{ name, value string }{
		{"title", item.Title},
		{"excerpt", item.Excerpt},
	} {
		t.Run(field.name, func(t *testing.T) {
			if n := strings.Count(field.value, "<b>"); n != 1 {
				t.Errorf("%s %q has %d highlight markers, want 1", field.name, field.value, n)
			}
			if !strings.Contains(field.value, "<b>keyword</b>") {
				t.Errorf("%s %q does not mark the hit", field.name, field.value)
			}
			if strings.ContainsAny(field.value, "\u0002\u0003") {
				t.Errorf("%s %q still contains a marker character", field.name, field.value)
			}
		})
	}

	var withMarkers int
	if err := index.withDBRead(func(db *sql.DB) error {
		return db.QueryRow(`SELECT count(*) FROM pages
			WHERE instr(title || headings || content, char(2)) > 0
			   OR instr(title || headings || content, char(3)) > 0`).Scan(&withMarkers)
	}); err != nil {
		t.Fatalf("count rows with markers: %v", err)
	}
	if withMarkers != 0 {
		t.Errorf("%d indexed rows still contain char(2) or char(3)", withMarkers)
	}
}

// Marker characters in odd places: only in a heading, glued to words, as the
// whole title, and other C0 controls next to them. None may fake a highlight,
// and the words around a replaced marker must stay findable on their own.
func TestSQLiteIndex_Search_HighlightMarkerEdgeCases_NeverFakeAHighlight(t *testing.T) {
	tests := []struct {
		name           string
		title          string
		content        string
		query          string
		wantTitleHits  int
		wantExcerptHit string
	}{
		{
			name:           "markers only in a heading",
			title:          "Plain",
			content:        "# Head\u0002fake\u0003ing\n\nbody keyword",
			query:          "fake",
			wantTitleHits:  0,
			wantExcerptHit: "<b>fake</b>",
		},
		{
			name:           "word before a marker in the title",
			title:          "foo\u0002bar",
			content:        "foo\u0003bar",
			query:          "foo",
			wantTitleHits:  1,
			wantExcerptHit: "<b>foo</b> bar",
		},
		{
			name:           "word after a marker in the title",
			title:          "foo\u0002bar",
			content:        "foo\u0003bar",
			query:          "bar",
			wantTitleHits:  1,
			wantExcerptHit: "foo <b>bar</b>",
		},
		{
			name:           "title made only of markers",
			title:          "\u0002\u0003\u0002",
			content:        "keyword",
			query:          "keyword",
			wantTitleHits:  0,
			wantExcerptHit: "<b>keyword</b>",
		},
		{
			name:           "other C0 and DEL controls around the hit",
			title:          "a\u0001b\u001fc\u007f keyword",
			content:        "x\u0001\u001fkeyword\u007fy \u001b[31m",
			query:          "keyword",
			wantTitleHits:  1,
			wantExcerptHit: "<b>keyword</b>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index := newTestIndex(t)
			if err := index.IndexPage("docs/a", "docs/a.md", "a", tt.title, tree.NodeKindPage, tt.content); err != nil {
				t.Fatalf("IndexPage: %v", err)
			}

			result, err := index.Search(tt.query, nil, 0, 10)
			if err != nil {
				t.Fatalf("Search(%q): %v", tt.query, err)
			}
			if len(result.Items) != 1 {
				t.Fatalf("Search(%q): got %d items, want 1", tt.query, len(result.Items))
			}
			item := result.Items[0]

			if n := strings.Count(item.Title, "<b>"); n != tt.wantTitleHits {
				t.Errorf("title %q has %d highlights, want %d", item.Title, n, tt.wantTitleHits)
			}
			if n := strings.Count(item.Excerpt, "<b>"); n != 1 {
				t.Errorf("excerpt %q has %d highlights, want 1", item.Excerpt, n)
			}
			if !strings.Contains(item.Excerpt, tt.wantExcerptHit) {
				t.Errorf("excerpt %q does not contain %q", item.Excerpt, tt.wantExcerptHit)
			}
			for name, value := range map[string]string{"title": item.Title, "excerpt": item.Excerpt} {
				if strings.ContainsAny(value, "\u0002\u0003") {
					t.Errorf("%s %q still contains a marker character", name, value)
				}
			}

			// Headings are never returned, so check the stored column.
			var headings string
			if err := index.withDBRead(func(db *sql.DB) error {
				return db.QueryRow(`SELECT headings FROM pages WHERE pageID = 'a'`).Scan(&headings)
			}); err != nil {
				t.Fatalf("read stored headings: %v", err)
			}
			if strings.ContainsAny(headings, "\u0002\u0003") {
				t.Errorf("stored headings %q still contain a marker character", headings)
			}
		})
	}
}

// newPiIndex indexes pages where "pi" is a whole word (raspberry, circles)
// next to pages where it is only a prefix (pipeline, pipelineSetup). The
// prefix-only pages have the shorter titles, so bm25 alone ranks them first.
// The same pattern repeats for an umlaut word (mueller, mullerStreet) and a
// CJK word (japanese, japanesePage); none of those contain "pi".
func newPiIndex(t *testing.T) *SQLiteIndex {
	t.Helper()
	index := newTestIndex(t)
	pages := []struct{ id, title, body string }{
		{id: "pipeline", title: "Pipeline Notes", body: "Build pipeline steps."},
		{id: "pipelineSetup", title: "Pipeline Setup", body: "Configure the pipeline first."},
		{id: "raspberry", title: "Raspberry Pi Setup", body: "Install it on a Raspberry Pi 4 board."},
		{id: "circles", title: "Pi and Circles", body: "The number pi relates a circle to its diameter, pi is irrational."},
		{id: "mullerStreet", title: "Mullerstrasse", body: "Office address."},
		{id: "mueller", title: "Notes from the call with Frau Müller", body: "Müller asked for the quarterly report."},
		{id: "japanesePage", title: "日本語ページ", body: "日本語ページ です"},
		{id: "japanese", title: "Sprachen im Team", body: "Wir sprechen 日本語 und Deutsch."},
	}
	for _, p := range pages {
		if err := index.IndexPage("docs/"+p.id, "docs/"+p.id+".md", p.id, p.title, tree.NodeKindPage, p.body); err != nil {
			t.Fatalf("failed to index %s: %v", p.id, err)
		}
	}
	return index
}

func resultPageIDs(result *SearchResult) []string {
	ids := make([]string, len(result.Items))
	for i, item := range result.Items {
		ids[i] = item.PageID
	}
	return ids
}

func TestSQLiteIndex_Search_RanksWholeWordMatchesAbovePrefixOnlyMatches(t *testing.T) {
	index := newPiIndex(t)

	// The groups are the tiers in order: every word whole, every word but
	// the last whole and the last one only a prefix, everything else.
	tests := []struct {
		name           string
		query          string
		pageIDs        []string
		wholeWord      []string
		lastWordPrefix []string
		prefixOnly     []string
	}{
		{name: "single word", query: "pi", wholeWord: []string{"raspberry", "circles"}, prefixOnly: []string{"pipeline", "pipelineSetup"}},
		{name: "every word must be whole", query: "pi setup", wholeWord: []string{"raspberry"}, prefixOnly: []string{"pipelineSetup"}},
		{name: "case does not matter", query: "PI", wholeWord: []string{"raspberry", "circles"}, prefixOnly: []string{"pipeline", "pipelineSetup"}},
		{name: "page filter binds before the tier", query: "pi", pageIDs: []string{"pipeline", "circles"}, wholeWord: []string{"circles"}, prefixOnly: []string{"pipeline"}},
		{name: "page filter with only prefix matches", query: "pi", pageIDs: []string{"pipeline", "pipelineSetup", "mueller"}, prefixOnly: []string{"pipeline", "pipelineSetup"}},
		// Search as you type: the word being typed is the last one.
		{name: "the last word may be typed partly", query: "pi set", lastWordPrefix: []string{"raspberry"}, prefixOnly: []string{"pipelineSetup"}},
		{name: "only the last word may be partial", query: "set pi", prefixOnly: []string{"raspberry", "pipelineSetup"}},
		{name: "diacritics fold for the tier", query: "muller", wholeWord: []string{"mueller"}, prefixOnly: []string{"mullerStreet"}},
		{name: "umlaut and upper case in the query", query: "MÜLLER", wholeWord: []string{"mueller"}, prefixOnly: []string{"mullerStreet"}},
		{name: "CJK word", query: "日本語", wholeWord: []string{"japanese"}, prefixOnly: []string{"japanesePage"}},
		{name: "emoji next to a word is ignored", query: "pi 🚀", wholeWord: []string{"raspberry", "circles"}, prefixOnly: []string{"pipeline", "pipelineSetup"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := index.Search(tc.query, tc.pageIDs, 0, 10)
			if err != nil {
				t.Fatalf("Search(%q) failed: %v", tc.query, err)
			}
			got := resultPageIDs(result)
			tiers := [][]string{tc.wholeWord, tc.lastWordPrefix, tc.prefixOnly}
			want := len(tc.wholeWord) + len(tc.lastWordPrefix) + len(tc.prefixOnly)
			if result.Count != want || len(got) != want {
				t.Fatalf("Search(%q): count %d, items %q, want %d results", tc.query, result.Count, got, want)
			}
			start := 0
			for tier, ids := range tiers {
				for _, id := range ids {
					if rank := slices.Index(got, id); rank < start || rank >= start+len(ids) {
						t.Errorf("Search(%q) = %q: %q must rank in tier %d, places %d to %d", tc.query, got, id, tier, start+1, start+len(ids))
					}
				}
				start += len(ids)
			}
		})
	}
}

// TestSQLiteIndex_Search_PaginatesAcrossTheWholeWordTier pages through a
// result whose tier boundary falls inside a page and whose pages tie on bm25
// within each tier: every page reports the full count, and the pages together
// list every match exactly once, whole-word matches first.
func TestSQLiteIndex_Search_PaginatesAcrossTheWholeWordTier(t *testing.T) {
	index := newTestIndex(t)
	const perTier = 23
	var wholeWord, all []string
	for i := 0; i < perTier; i++ {
		whole, prefix := fmt.Sprintf("whole%02d", i), fmt.Sprintf("prefix%02d", i)
		if err := index.IndexPage("docs/"+whole, "docs/"+whole+".md", whole, "Board", tree.NodeKindPage, "Runs on a pi board."); err != nil {
			t.Fatalf("failed to index %s: %v", whole, err)
		}
		if err := index.IndexPage("docs/"+prefix, "docs/"+prefix+".md", prefix, "Board", tree.NodeKindPage, "Runs a pipeline board."); err != nil {
			t.Fatalf("failed to index %s: %v", prefix, err)
		}
		wholeWord = append(wholeWord, whole)
		all = append(all, whole, prefix)
	}
	subset := append(append([]string{}, all[:10]...), "missing")

	// For "pi boa" the whole-word pages sit in the middle tier and the
	// prefix pages in the last one, so the boundary is the same.
	tests := []struct {
		name      string
		query     string
		pageIDs   []string
		limit     int
		want      []string
		wholeWord int
	}{
		{name: "limit 1", limit: 1, want: all, wholeWord: perTier},
		{name: "middle tier, limit 7", query: "pi boa", limit: 7, want: all, wholeWord: perTier},
		{name: "limit 7", limit: 7, want: all, wholeWord: perTier},
		{name: "limit 10", limit: 10, want: all, wholeWord: perTier},
		{name: "page filter, limit 3", pageIDs: subset, limit: 3, want: all[:10], wholeWord: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := tc.query
			if query == "" {
				query = "pi"
			}
			var got []string
			for offset := 0; offset < len(tc.want)+tc.limit; offset += tc.limit {
				result, err := index.Search(query, tc.pageIDs, offset, tc.limit)
				if err != nil {
					t.Fatalf("Search(%q, offset %d) failed: %v", query, offset, err)
				}
				if result.Count != len(tc.want) {
					t.Fatalf("Search(%q, offset %d): count %d, want %d", query, offset, result.Count, len(tc.want))
				}
				got = append(got, resultPageIDs(result)...)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("pages listed %d items, want %d: %q", len(got), len(tc.want), got)
			}
			seen := map[string]bool{}
			for i, id := range got {
				if seen[id] {
					t.Errorf("%q listed twice", id)
				}
				seen[id] = true
				if isWhole := slices.Contains(wholeWord, id); isWhole != (i < tc.wholeWord) {
					t.Errorf("item %d is %q: whole-word matches must fill exactly the first %d places", i, id, tc.wholeWord)
				}
			}
			for _, id := range tc.want {
				if !seen[id] {
					t.Errorf("%q is missing from the pages", id)
				}
			}
		})
	}
}

// bm25Order is the order Search had before the whole-word tier: the same
// MATCH and bm25 weights, nothing else.
func bm25Order(t *testing.T, index *SQLiteIndex, query string) []string {
	t.Helper()
	var ids []string
	err := index.withDBRead(func(db *sql.DB) error {
		rows, err := db.Query(fmt.Sprintf(`SELECT pageID FROM pages WHERE pages MATCH ? ORDER BY %s ASC`, searchRankExpr(true)), buildFuzzyQuery(query))
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatalf("bm25 reference query for %q failed: %v", query, err)
	}
	return ids
}

func TestSQLiteIndex_Search_OperatorQueriesKeepBM25Order(t *testing.T) {
	index := newPiIndex(t)

	for _, query := range []string{"title:pi", "pi*", `"pi"`, "pi OR x", "pi* setup", "pi AND setup"} {
		t.Run(query, func(t *testing.T) {
			result, err := index.Search(query, nil, 0, 10)
			if err != nil {
				t.Fatalf("Search(%q) failed: %v", query, err)
			}
			got, want := resultPageIDs(result), bm25Order(t, index, query)
			if len(want) == 0 {
				t.Fatalf("bm25 reference for %q is empty, the case checks nothing", query)
			}
			if !slices.Equal(got, want) {
				t.Errorf("Search(%q) = %q, want the bm25 order %q", query, got, want)
			}
		})
	}
}

// TestSQLiteIndex_Search_LowercaseKeywordIsAnOrdinaryWord pins that only
// upper-case AND/OR/NOT are operators, as the search help says. A lowercase
// "and" is one more literal word: ANDed as the prefix "and"*, so the result
// is the pages that match "pi setup" and also contain a word starting with
// "and", ranked by the whole-word tiers. Before, FTS5 read the unquoted "and"
// as the exact token and the query lost its tiers.
func TestSQLiteIndex_Search_LowercaseKeywordIsAnOrdinaryWord(t *testing.T) {
	index := newTestIndex(t)
	pages := []struct{ id, title, body string }{
		{id: "noAnd", title: "Raspberry Pi Setup", body: "Install it on a board."},
		{id: "wholeAnd", title: "Pi and Setup Guide", body: "Notes."},
		{id: "prefixAnd", title: "Pipeline Setup", body: "Builds the Android app."},
		{id: "andOnly", title: "Salt and Pepper", body: "Kitchen."},
	}
	for _, p := range pages {
		if err := index.IndexPage("docs/"+p.id, "docs/"+p.id+".md", p.id, p.title, tree.NodeKindPage, p.body); err != nil {
			t.Fatalf("failed to index %s: %v", p.id, err)
		}
	}

	search := func(query string) []string {
		t.Helper()
		result, err := index.Search(query, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q) failed: %v", query, err)
		}
		if ids := resultPageIDs(result); result.Count != len(ids) {
			t.Fatalf("Search(%q): count %d, but %d items", query, result.Count, len(ids))
		}
		return resultPageIDs(result)
	}

	withoutAnd := search("pi setup")
	slices.Sort(withoutAnd)
	if want := []string{"noAnd", "prefixAnd", "wholeAnd"}; !slices.Equal(withoutAnd, want) {
		t.Fatalf("Search(pi setup) = %q, want the pages %q", withoutAnd, want)
	}
	// AND narrows: the pi setup pages that also have a word starting with
	// "and", whole words first. "Salt and Pepper" has no pi or setup.
	if got, want := search("pi and setup"), []string{"wholeAnd", "prefixAnd"}; !slices.Equal(got, want) {
		t.Errorf("Search(pi and setup) = %q, want %q in this order", got, want)
	}
}

// TestSQLiteIndex_Search_WholeWordTierIsInjectionSafe sends input that would
// break the exact-word expression if it were spliced into the SQL or kept
// its quotes: it must neither fail nor touch the table.
func TestSQLiteIndex_Search_WholeWordTierIsInjectionSafe(t *testing.T) {
	index := newPiIndex(t)

	for _, query := range []string{`pi'); DROP TABLE pages;--`, `pi" OR "x`, `pi"*`, `"pi`, `pi''`, `pi?`} {
		t.Run(query, func(t *testing.T) {
			if _, err := index.Search(query, nil, 0, 10); err != nil {
				t.Fatalf("Search(%q) failed: %v", query, err)
			}
		})
	}
	result, err := index.Search("pi", nil, 0, 10)
	if err != nil || result.Count != 4 {
		t.Fatalf("after the hostile queries Search(pi) = %v, %v; want all 4 pages", result, err)
	}
}

func TestSQLiteIndex_SearchPageIDs_RespectsQueryAndPageFilters(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("docs/alpha", "docs/alpha.md", "alpha", "Alpha Page", tree.NodeKindPage, "Shared token in alpha.")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	err = index.IndexPage("docs/beta", "docs/beta.md", "beta", "Beta Page", tree.NodeKindPage, "Shared token in beta.")
	if err != nil {
		t.Fatalf("failed to index beta page: %v", err)
	}

	err = index.IndexPage("docs/gamma", "docs/gamma.md", "gamma", "Gamma Page", tree.NodeKindPage, "Gamma only content.")
	if err != nil {
		t.Fatalf("failed to index gamma page: %v", err)
	}

	pageIDs, err := index.SearchPageIDs("shared token", []string{"alpha"})
	if err != nil {
		t.Fatalf("SearchPageIDs failed: %v", err)
	}

	if len(pageIDs) != 1 || pageIDs[0] != "alpha" {
		t.Fatalf("expected only alpha page, got %#v", pageIDs)
	}

	noMatches, err := index.SearchPageIDs("shared token", []string{})
	if err != nil {
		t.Fatalf("SearchPageIDs with empty page filter failed: %v", err)
	}
	if len(noMatches) != 0 {
		t.Fatalf("expected no matches for empty page filter, got %#v", noMatches)
	}
}

func TestSQLiteIndex_Search_FiltersByPageIDs(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage(
		"docs/react-guide",
		"docs/react-guide.md",
		"react-guide",
		"React guide",
		tree.NodeKindPage,
		"Search term appears here.",
	)
	if err != nil {
		t.Fatalf("failed to index react-guide page: %v", err)
	}

	err = index.IndexPage(
		"docs/plain-guide",
		"docs/plain-guide.md",
		"plain-guide",
		"Plain guide",
		tree.NodeKindPage,
		"Search term appears here as well.",
	)
	if err != nil {
		t.Fatalf("failed to index plain-guide page: %v", err)
	}

	result, err := index.Search("search", []string{"react-guide"}, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Count != 1 {
		t.Fatalf("expected 1 filtered result, got %d", result.Count)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 filtered result item, got %d", len(result.Items))
	}
	if result.Items[0].PageID != "react-guide" {
		t.Fatalf("expected filtered page react-guide, got %s", result.Items[0].PageID)
	}
}

func TestSQLiteIndex_Search_ReturnsNoResultsWhenPageIDFilterIsEmpty(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage(
		"docs/react-guide",
		"docs/react-guide.md",
		"react-guide",
		"React guide",
		tree.NodeKindPage,
		"Search term appears here.",
	)
	if err != nil {
		t.Fatalf("failed to index react-guide page: %v", err)
	}

	result, err := index.Search("search", []string{}, 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Count != 0 {
		t.Fatalf("expected 0 filtered results, got %d", result.Count)
	}
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 filtered result items, got %d", len(result.Items))
	}
}

func TestSQLiteIndex_IndexPage_StripsShoutoutFenceSyntaxButKeepsLabel(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage(
		"docs/shoutout",
		"docs/shoutout.md",
		"shoutout1",
		"Shoutout Page",
		tree.NodeKindPage,
		strings.Join([]string{
			"::: blue",
			"Shoutout body text.",
			":::",
		}, "\n"),
	)
	if err != nil {
		t.Fatalf("IndexPage failed: %v", err)
	}

	var gotContent string
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT content FROM pages WHERE pageID = ?`, "shoutout1").Scan(&gotContent)
	}); err != nil {
		t.Fatalf("failed to read indexed content: %v", err)
	}

	if strings.Contains(gotContent, ":::") {
		t.Fatalf("expected indexed content to exclude shoutout fences, got %q", gotContent)
	}
	if !strings.Contains(gotContent, "blue") {
		t.Fatalf("expected indexed content to keep shoutout label, got %q", gotContent)
	}
	if !strings.Contains(gotContent, "Shoutout body text.") {
		t.Fatalf("expected indexed content to keep shoutout body, got %q", gotContent)
	}
}

func TestSQLiteIndex_IndexPage_StripsMarkdownFormattingFromIndexedContent(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage(
		"docs/markdown",
		"docs/markdown.md",
		"markdown1",
		"Markdown Page",
		tree.NodeKindPage,
		"LeafWiki **fett** und _kursiv_.",
	)
	if err != nil {
		t.Fatalf("IndexPage failed: %v", err)
	}

	var gotContent string
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT content FROM pages WHERE pageID = ?`, "markdown1").Scan(&gotContent)
	}); err != nil {
		t.Fatalf("failed to read indexed content: %v", err)
	}

	if strings.Contains(gotContent, "**") || strings.Contains(gotContent, "_") {
		t.Fatalf("expected indexed content to exclude markdown emphasis markers, got %q", gotContent)
	}
}

func TestSQLiteIndex_Search_FindsTermsInsideCodeBlocks(t *testing.T) {
	index := newTestIndex(t)

	content := strings.Join([]string{
		"# Example Test Page with a code block.",
		"",
		"```bash",
		"# Fetch IP and store it in a variable",
		"MY_IP=$(curl -sL api.ipify.org)",
		"",
		"# Get extra details in json format",
		`curl -sL "ipapi.co/${MY_IP}/json/"`,
		"```",
	}, "\n")

	err := index.IndexPage("docs/code-example", "docs/code-example.md", "code1", "Example Test Page with a code block.", tree.NodeKindPage, content)
	if err != nil {
		t.Fatalf("IndexPage failed: %v", err)
	}

	for _, term := range []string{"curl", "Fetch"} {
		result, err := index.Search(term, nil, 0, 10)
		if err != nil {
			t.Fatalf("search for %q failed: %v", term, err)
		}
		if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "code1" {
			t.Errorf("expected to find page via code block term %q, got count=%d", term, result.Count)
		}
	}
}

// TestSQLiteIndex_Search_TolerateFTS5SpecialCharacters is the regression test
// for https://github.com/perber/leafwiki/issues/1577: a comma, apostrophe, or
// stray quote in the query used to reach SQLite's FTS5 query parser
// unescaped and fail with "fts5: syntax error" / "unterminated string",
// surfacing to callers as a generic error instead of a normal (possibly
// empty) result.
func TestSQLiteIndex_Search_TolerateFTS5SpecialCharacters(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Hello World", tree.NodeKindPage, "hello world content")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	queries := []string{
		"hello,world",
		"hello, world",
		"it's",
		`hello "world`,
		`hello"world`,
	}

	for _, q := range queries {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
		if _, err := index.SearchPageIDs(q, nil); err != nil {
			t.Errorf("SearchPageIDs(%q) returned an error instead of a result: %v", q, err)
		}
	}
}

// TestSQLiteIndex_Search_CommaSeparatedTermsStillFindMatches guards against a
// fix that merely swallows the error (e.g. by returning empty results
// whenever punctuation is present) instead of actually searching: splitting
// "hello,world" on the comma should still find a page containing both words.
func TestSQLiteIndex_Search_CommaSeparatedTermsStillFindMatches(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Greeting", tree.NodeKindPage, "hello world, this page greets everyone")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("hello,world", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected comma-joined query to still find the page, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_SingleWordPrefixMatchUnaffectedByQuoting pins the
// existing prefix-search behavior for ordinary single-word queries: the fix
// for #1577 must not change ranking/matching for the common case.
func TestSQLiteIndex_Search_SingleWordPrefixMatchUnaffectedByQuoting(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "this page is about searching and searches")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("search", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected prefix match on 'search' to still find the page, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_ExplicitFTS5SyntaxStillPassesThrough guards the
// existing "trust the user's FTS5 syntax" branch (column filters, boolean
// operators): the #1577 fix only changes the plain-word fallback path.
func TestSQLiteIndex_Search_ExplicitFTS5SyntaxStillPassesThrough(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha Search Test", tree.NodeKindPage, "This content is about SQLite search.")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("content:search*", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("expected explicit FTS5 column-filter syntax to still work, got count=%d", result.Count)
	}

	result, err = index.Search("search AND content", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("expected explicit FTS5 AND syntax to still work, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_HyphenatedQueryStillFindsPunctuationToken guards the
// old branch-2 behavior (querying a hyphen/dot/hash-bearing token) after its
// whole-query quoting is replaced by per-term quoting: a query matching an
// indexed punctuation-bearing token should still be found.
func TestSQLiteIndex_Search_HyphenatedQueryStillFindsPunctuationToken(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Report", tree.NodeKindPage, "see report-final.v2 for details")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("report-final.v2", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected hyphenated/dotted query to still find the page, got count=%d", result.Count)
	}
}

// TestBuildFuzzyQuery_NeverProducesSyntaxErrorForMixedPunctuationAndOperators
// is the regression test for the follow-up finding on #1577: the original
// fix only escaped a comma/apostrophe when the *whole query* contained no
// FTS5 operator character anywhere, so a query mixing an operator in one
// field with a bare comma/apostrophe in another (e.g. "foo* bar,baz") still
// produced "fts5: syntax error". Each case here is executed against a real
// FTS5 table, not just asserted against buildFuzzyQuery's string output, so
// a change that "looks" safe but is still invalid FTS5 syntax would fail.
func TestBuildFuzzyQuery_NeverProducesSyntaxErrorForMixedPunctuationAndOperators(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Foo Bar", tree.NodeKindPage, "foo bar baz content")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	queries := []string{
		"foo* bar,baz",
		"foo AND bar,baz",
		`"foo" bar,baz`,
		"it's* thing",
		`"foo*`, // unbalanced quote directly adjacent to an operator character
		// NB: "(foo) bar,baz" is deliberately not covered here — it hits a
		// separate, pre-existing FTS5 grammar quirk unrelated to #1577: a
		// parenthesized group followed by any other juxtaposed term (with no
		// explicit AND/OR) is already a syntax error before this fix too,
		// since the pre-#1577 code trusted anything containing "(" and
		// passed it through unescaped. It needs its own fix in the query
		// sanitizer, so it is left out of this regression test.
	}

	for _, q := range queries {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
	}
}

// TestSQLiteIndex_Search_ApostropheWordStillMatchesIndexedContent guards
// against a fix that merely avoids the crash without still finding the
// intended content: "it's" should still find a page containing "it's".
func TestSQLiteIndex_Search_ApostropheWordStillMatchesIndexedContent(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Note", tree.NodeKindPage, "it's a great day today")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("it's", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected apostrophe word to still find the page, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_BareColonQueriesDoNotCrash is the regression test
// for a pre-existing bug found (and fixed) as a side effect of #1577: any
// query containing ":" was trusted as an FTS5 column filter regardless of
// whether the text before the colon was a real column, so a pasted URL,
// Windows path, or time crashed with "no such column: ...".
func TestSQLiteIndex_Search_BareColonQueriesDoNotCrash(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Notes", tree.NodeKindPage, "see http example com for the schedule at 10 30")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	queries := []string{
		"http://example.com",
		"see http://example.com for details",
		`c:\Users\foo`,
		"time: 10:30",
	}

	for _, q := range queries {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
	}
}

// TestSQLiteIndex_Search_RealColumnFilterStillWorks guards the existing
// "content:search*" style power-user syntax after adding column-name
// validation: a genuine column filter naming a real column must still be
// trusted and behave as before.
func TestSQLiteIndex_Search_RealColumnFilterStillWorks(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha Search Test", tree.NodeKindPage, "This content is about SQLite search.")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	for _, q := range []string{"content:search*", "title:Alpha*", "PAGEID:alpha1"} {
		result, err := index.Search(q, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q) failed: %v", q, err)
		}
		if result.Count != 1 {
			t.Errorf("Search(%q): expected column-filter syntax to still find the page, got count=%d", q, result.Count)
		}
	}
}

// TestSQLiteIndex_Search_DanglingBooleanKeywordDoesNotCrash is the
// regression test for the other pre-existing bug found (and fixed) as a
// side effect of #1577: a bare AND/OR/NOT with no operand on one side (a
// user literally searching for that word, or an as-you-type request
// captured mid-typing) was trusted as a boolean operator regardless of
// context and crashed with an FTS5 syntax error.
func TestSQLiteIndex_Search_DanglingBooleanKeywordDoesNotCrash(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Notes", tree.NodeKindPage, "foo and bar or not baz")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	queries := []string{"AND", "OR", "NOT", "NOT foo", "foo AND", "AND foo"}

	for _, q := range queries {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
	}
}

// TestSQLiteIndex_Search_BooleanOperatorBetweenRealOperandsStillWorks
// guards the existing "search AND content" style explicit boolean syntax
// after adding the neighbor-context check: AND/OR sitting between two real
// operands must still be trusted and behave as before.
func TestSQLiteIndex_Search_BooleanOperatorBetweenRealOperandsStillWorks(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha Search Test", tree.NodeKindPage, "This content is about SQLite search.")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("search AND content", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("expected explicit FTS5 AND syntax between real operands to still work, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_PunctuationOnlyQueryReturnsEmptyResultNotError is
// the regression test for a bug found (and fixed) as a side effect of
// #1577: a query built entirely from characters outside the tokenizer's
// word alphabet (a lone comma, apostrophe, unbalanced quote, colon(s), or
// emoji) sanitizes to an empty FTS5 query string, and `pages MATCH ”` is
// itself an FTS5 syntax error — the exact crash class #1577 reported, for
// the simplest possible input. It should behave like any other query with
// nothing to search on: an empty result, not an error.
func TestSQLiteIndex_Search_PunctuationOnlyQueryReturnsEmptyResultNotError(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "some content")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	queries := []string{",", "'", `"`, "::", "!!!", "😀"}

	for _, q := range queries {
		result, err := index.Search(q, nil, 0, 10)
		if err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
			continue
		}
		if result.Count != 0 {
			t.Errorf("Search(%q): expected 0 results for a punctuation-only query, got %d", q, result.Count)
		}
		if _, err := index.SearchPageIDs(q, nil); err != nil {
			t.Errorf("SearchPageIDs(%q) returned an error instead of a result: %v", q, err)
		}
	}
}

// TestSQLiteIndex_Search_PunctuationOnlyQueryWithPageIDFilterStillFilters
// guards the combined case: a punctuation-only query text alongside a
// pageIDs filter should behave like an empty query with that filter (return
// the filtered pages, no MATCH applied) rather than erroring or ignoring
// the filter.
func TestSQLiteIndex_Search_PunctuationOnlyQueryWithPageIDFilterStillFilters(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "some content")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}
	err = index.IndexPage("notes/beta", "notes/beta.md", "beta1", "Beta", tree.NodeKindPage, "other content")
	if err != nil {
		t.Fatalf("failed to index beta page: %v", err)
	}

	result, err := index.Search(",", []string{"alpha1"}, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected punctuation-only query + pageID filter to return the filtered page, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_BareOperatorCharacterDoesNotCrash is the
// regression test for a bug found (and fixed) as a side effect of #1577:
// a lone "*", "(", or ")" with no operand/matching partner was trusted as
// deliberate FTS5 syntax purely because it contained an operator
// character, even though none of these are valid standalone FTS5 syntax.
func TestSQLiteIndex_Search_BareOperatorCharacterDoesNotCrash(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "some content")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	for _, q := range []string{"*", "(", ")", "foo * bar"} {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
	}
}

// TestSQLiteIndex_Search_DoubledBooleanKeywordDoesNotCrash is the
// regression test for a bug found (and fixed) as a side effect of #1577:
// hasRealOperandNeighbors only checked slice position, not whether a
// keyword's neighbor was itself another keyword, so a doubled "AND AND"
// (a plausible typo or fast-typing artifact) was trusted twice over and
// reached SQLite as two adjacent, unconnected operators.
func TestSQLiteIndex_Search_DoubledBooleanKeywordDoesNotCrash(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "foo bar content")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	if _, err := index.Search("foo AND AND bar", nil, 0, 10); err != nil {
		t.Errorf("Search(%q) returned an error instead of a result: %v", "foo AND AND bar", err)
	}
}

// TestSQLiteIndex_Search_ColumnFilterWithPunctuationValueStaysScoped is the
// regression test for two bugs found (and fixed) as a side effect of
// #1577: a column filter's value was either trusted verbatim (breaking on
// a pasted URL like "content:http://example.com", whose second ":" is
// invalid FTS5 syntax) or, once that was gated behind an "unsafe chars"
// check, silently lost its column scope entirely for a value containing
// an apostrophe or comma (falling back to an unscoped, all-columns
// search). Rewriting into a scoped, per-word quoted-prefix group fixes
// both: no crash, and the column scope from the user's `col:` filter is
// preserved rather than dropped.
func TestSQLiteIndex_Search_ColumnFilterWithPunctuationValueStaysScoped(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "don't forget the http example com link")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}
	// "wobble" appears only in beta's title, never in any page's content -
	// used below to confirm a punctuation-value column filter still
	// actually scopes to the named column instead of silently falling back
	// to an unscoped, all-columns search.
	err = index.IndexPage("notes/beta", "notes/beta.md", "beta1", "Wobble's Page", tree.NodeKindPage, "unrelated body text")
	if err != nil {
		t.Fatalf("failed to index beta page: %v", err)
	}

	for _, q := range []string{"content:http://example.com", "content:don't"} {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
	}

	result, err := index.Search("content:wobble", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 0 {
		t.Errorf("content:wobble matched %d pages, want 0 (\"wobble\" only appears in beta's title) — a punctuation-value column filter must still be column-scoped, not silently searching every column", result.Count)
	}

	result, err = index.Search("content:don't", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected content:don't to still find alpha1 via its content, got count=%d", result.Count)
	}
}

// TestSQLiteIndex_Search_NearFunctionCallStillWorks is the regression test
// for an actual regression found (and fixed) as a side effect of #1577,
// not just an unfixed pre-existing gap: NEAR(term1 term2, distance) syntax
// worked before this fix's per-field classification existed (the whole
// query was trusted verbatim), but per-field splitting on whitespace broke
// it apart into "term2," and "distance)" as two unrelated fields and
// silently stripped the comma the NEAR grammar requires — no error, just
// wrong (empty) results.
func TestSQLiteIndex_Search_NearFunctionCallStillWorks(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "foo one two three bar")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	result, err := index.Search("NEAR(foo bar, 5)", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Count != 1 || len(result.Items) == 0 || result.Items[0].PageID != "alpha1" {
		t.Errorf("expected NEAR(...) proximity syntax to still find the page, got count=%d", result.Count)
	}
}

func TestExtractHeadings_SingleH1(t *testing.T) {
	got := extractHeadings("# Hello World")
	if !strings.Contains(got, "Hello World") {
		t.Errorf("expected heading text, got %q", got)
	}
}

func TestExtractHeadings_MultipleHeadings(t *testing.T) {
	input := "# First\n## Second\n### Third"
	got := extractHeadings(input)
	for _, want := range []string{"First", "Second", "Third"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in result, got %q", want, got)
		}
	}
}

func TestExtractHeadings_InlineFormatting(t *testing.T) {
	got := extractHeadings("## **Bold** and _italic_ heading")
	if strings.Contains(got, "**") || strings.Contains(got, "_") {
		t.Errorf("expected formatting markers stripped, got %q", got)
	}
	if !strings.Contains(got, "Bold") || !strings.Contains(got, "italic") || !strings.Contains(got, "heading") {
		t.Errorf("expected heading words preserved, got %q", got)
	}
}

func TestExtractHeadings_IgnoresBodyText(t *testing.T) {
	got := extractHeadings("# Title\n\nSome body paragraph that should not appear.")
	if strings.Contains(got, "body paragraph") {
		t.Errorf("body text should not appear in headings, got %q", got)
	}
	if !strings.Contains(got, "Title") {
		t.Errorf("expected heading text, got %q", got)
	}
}

func TestExtractHeadings_NoHeadings(t *testing.T) {
	got := extractHeadings("Just plain text without any heading.")
	if got != "" {
		t.Errorf("expected empty result for no headings, got %q", got)
	}
}

func TestExtractHeadings_EmptyInput(t *testing.T) {
	got := extractHeadings("")
	if got != "" {
		t.Errorf("expected empty result for empty input, got %q", got)
	}
}

func TestExtractHeadings_HeadingWithCodeSpan(t *testing.T) {
	got := extractHeadings("## Heading `code` here")
	if strings.Contains(got, "`") {
		t.Errorf("expected backticks stripped, got %q", got)
	}
	if !strings.Contains(got, "Heading") || !strings.Contains(got, "here") {
		t.Errorf("expected heading words preserved, got %q", got)
	}
}

// TestSQLiteIndex_Search_OperatorFieldWithTokenPunctuationDoesNotError guards
// fields that contain an FTS5 operator character (*, (, )) together with
// punctuation FTS5 barewords don't allow (- . / #) or with unbalanced
// parentheses. They used to be passed to MATCH verbatim and fail with a
// syntax error.
func TestSQLiteIndex_Search_OperatorFieldWithTokenPunctuationDoesNotError(t *testing.T) {
	index := newTestIndex(t)

	err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "foo-bar-baz report.v2-final c#sharp a/b/c")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	queries := []string{
		"foo-bar*", "report.v2*", "c#*", "a/b*", "foo**", "*foo", "f(x)",
		"(foo", "foo)", "foo AND (bar", "NEAR(a b", "()", "(*)", "foo*bar",
	}
	for _, q := range queries {
		if _, err := index.Search(q, nil, 0, 10); err != nil {
			t.Errorf("Search(%q) returned an error instead of a result: %v", q, err)
		}
		if _, err := index.SearchPageIDs(q, nil); err != nil {
			t.Errorf("SearchPageIDs(%q) returned an error instead of a result: %v", q, err)
		}
	}

	for _, q := range []string{"foo-bar*", "report.v2*"} {
		result, err := index.Search(q, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		if result.Count != 1 {
			t.Errorf("Search(%q): expected the alpha page as prefix match, got %d results", q, result.Count)
		}
	}
}

// TestSQLiteIndex_Search_BalancedParenGroupAcrossFieldsStillWorks makes sure
// deliberate grouping spread over several fields is still trusted.
func TestSQLiteIndex_Search_BalancedParenGroupAcrossFieldsStillWorks(t *testing.T) {
	index := newTestIndex(t)

	if err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha", tree.NodeKindPage, "apple banana"); err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}
	if err := index.IndexPage("notes/beta", "notes/beta.md", "beta1", "Beta", tree.NodeKindPage, "cherry banana"); err != nil {
		t.Fatalf("failed to index beta page: %v", err)
	}

	result, err := index.Search("(apple OR cherry) AND banana", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search returned an error: %v", err)
	}
	if result.Count != 2 {
		t.Errorf("expected both pages for a grouped OR query, got %d", result.Count)
	}
}

// search.db is a derived index (ADR-0022): opening it keeps its rows only when
// the stored schema version and DDL match this binary; anything else, and a
// corrupt file (ADR-0013), leaves empty tables at the current version.
func TestNewSQLiteIndex_OpenKeepsOrRebuildsTheSchema(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		wantRows int
	}{
		{
			name:     "fresh database gets the schema version",
			setup:    func(t *testing.T, dir string) {},
			wantRows: 0,
		},
		{
			name:     "reopening keeps the indexed rows",
			setup:    indexOnePageAndClose,
			wantRows: 1,
		},
		{
			name: "other schema version rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir, fmt.Sprintf(`PRAGMA user_version = %d`, searchSchemaVersion+1))
			},
			wantRows: 0,
		},
		{
			// Version 1 kept char(2)/char(3) from page text, which the
			// highlight markers now rely on being absent.
			name: "version 1 index from before marker stripping rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir, `PRAGMA user_version = 1`)
			},
			wantRows: 0,
		},
		{
			// An older binary drops and recreates pages in its own shape but
			// leaves user_version alone, so the version alone can't tell.
			name: "older pages DDL with the current version rebuilds",
			setup: func(t *testing.T, dir string) {
				execRawSearchDB(t, dir,
					`CREATE VIRTUAL TABLE pages USING fts5(path UNINDEXED, pageID, title, content)`,
					`INSERT INTO pages (path, pageID, title, content) VALUES ('old', 'old', 'Old', 'old content')`,
					fmt.Sprintf(`PRAGMA user_version = %d`, searchSchemaVersion),
				)
			},
			wantRows: 0,
		},
		{
			name: "unexpected extra table rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir, `CREATE TABLE leftover (id INTEGER)`)
			},
			wantRows: 0,
		},
		{
			// The upgrade path: a binary from before ADR-0022 wrote the
			// current DDL but never set user_version.
			name: "current DDL without a schema version rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir, `PRAGMA user_version = 0`)
			},
			wantRows: 0,
		},
		{
			name: "negative schema version rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir, `PRAGMA user_version = -1`)
			},
			wantRows: 0,
		},
		{
			name: "unexpected view rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir, `CREATE VIEW leftover AS SELECT pageID FROM pages`)
			},
			wantRows: 0,
		},
		{
			name: "unexpected table with quotes and Unicode in its name rebuilds",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir,
					`CREATE TABLE "left""over ä 🌿" (id INTEGER)`,
					`CREATE INDEX "idx ""x""" ON "left""over ä 🌿"(id)`,
					`CREATE TRIGGER trg AFTER INSERT ON "left""over ä 🌿" BEGIN SELECT 1; END`,
				)
			},
			wantRows: 0,
		},
		{
			name: "unexpected second FTS5 table and its shadow tables rebuild",
			setup: func(t *testing.T, dir string) {
				indexOnePageAndClose(t, dir)
				execRawSearchDB(t, dir,
					`CREATE VIRTUAL TABLE old_trigram USING fts5(content, tokenize = 'trigram')`,
					`INSERT INTO old_trigram (content) VALUES ('leftover')`,
				)
			},
			wantRows: 0,
		},
		{
			// The schema in page 1 is intact, so the read-only probe and the
			// sqlite_master comparison pass; the FTS5 table itself is
			// unreadable. Before ADR-0022 the DROP hit this and
			// RebuildOnCorruption recovered.
			name: "corrupt FTS5 pages behind an intact schema rebuild",
			setup: func(t *testing.T, dir string) {
				index, err := NewSQLiteIndex(dir)
				if err != nil {
					t.Fatalf("NewSQLiteIndex: %v", err)
				}
				for i := range 300 {
					if err := index.IndexPage(fmt.Sprintf("docs/p%d", i), "docs/p.md", fmt.Sprintf("p%d", i), "P", tree.NodeKindPage, fmt.Sprintf("word%d lorem ipsum dolor sit amet %d", i, i*7)); err != nil {
						t.Fatalf("IndexPage: %v", err)
					}
				}
				if err := index.Close(); err != nil {
					t.Fatalf("Close: %v", err)
				}
				path := filepath.Join(dir, "search.db")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				for i := 3 * 4096; i < len(data); i++ {
					data[i] = 0xA5
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			wantRows: 0,
		},
		{
			name: "corrupt file rebuilds",
			setup: func(t *testing.T, dir string) {
				test_utils.WriteCorruptSQLiteDB(t, filepath.Join(dir, "search.db"))
			},
			wantRows: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			index, err := NewSQLiteIndex(dir)
			if err != nil {
				t.Fatalf("NewSQLiteIndex: %v", err)
			}
			defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

			var version, rows, extra int
			if err := index.withDB(func(db *sql.DB) error {
				if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
					return err
				}
				if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'leftover'`).Scan(&extra); err != nil {
					return err
				}
				// Selecting every current column fails on an older shape.
				return db.QueryRow(`SELECT count(*) FROM (SELECT path, filepath, pageID, kind, title, headings, content FROM pages)`).Scan(&rows)
			}); err != nil {
				t.Fatalf("read reopened index: %v", err)
			}
			if version != searchSchemaVersion {
				t.Errorf("user_version = %d, want %d", version, searchSchemaVersion)
			}
			if rows != tt.wantRows {
				t.Errorf("rows = %d, want %d", rows, tt.wantRows)
			}
			if extra != 0 {
				t.Errorf("unexpected table survived the rebuild")
			}
			want, err := expectedSearchSchema()
			if err != nil {
				t.Fatalf("expectedSearchSchema: %v", err)
			}
			var got string
			if err := index.withDB(func(db *sql.DB) error {
				var err error
				got, err = readSchema(db)
				return err
			}); err != nil {
				t.Fatalf("readSchema: %v", err)
			}
			if got != want {
				t.Errorf("schema after open differs from searchSchema:\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// An index from before the searchSchemaVersion bump is rebuilt on the first
// open only: what is indexed after that rebuild survives the next open.
func TestNewSQLiteIndex_RebuildsAnOlderVersionOnceAndThenKeepsIt(t *testing.T) {
	dir := t.TempDir()
	indexOnePageAndClose(t, dir)
	execRawSearchDB(t, dir, fmt.Sprintf(`PRAGMA user_version = %d`, searchSchemaVersion-1))

	countRows := func(index *SQLiteIndex) int {
		t.Helper()
		var rows int
		if err := index.withDBRead(func(db *sql.DB) error {
			return db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows)
		}); err != nil {
			t.Fatalf("count rows: %v", err)
		}
		return rows
	}

	index, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("first NewSQLiteIndex: %v", err)
	}
	if rows := countRows(index); rows != 0 {
		t.Errorf("first open kept %d rows of the older index, want 0", rows)
	}
	if err := index.IndexPage("docs/b", "docs/b.md", "b", "B", tree.NodeKindPage, "reindexed content"); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}
	if err := index.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	index, err = NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("second NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
	if rows := countRows(index); rows != 1 {
		t.Errorf("second open has %d rows, want the 1 reindexed row", rows)
	}
	result, err := index.Search("reindexed", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if result.Count != 1 {
		t.Errorf("Search after second open: got %d results, want 1", result.Count)
	}
}

// newTestIndex opens a fresh index in a temporary directory and closes it
// when the test ends.
func newTestIndex(t *testing.T) *SQLiteIndex {
	t.Helper()
	index, err := NewSQLiteIndex(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(index.Close, t) })
	return index
}

func indexOnePageAndClose(t *testing.T, dir string) {
	t.Helper()
	index, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	if err := index.IndexPage("docs/a", "docs/a.md", "a", "A", tree.NodeKindPage, "persisted content"); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}
	if err := index.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func execRawSearchDB(t *testing.T, dir string, statements ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "search.db"))
	if err != nil {
		t.Fatalf("open search.db: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

// A reopened index must answer queries exactly as before the restart: same
// hits, bm25 order, ranks and highlighted excerpts, and an intact FTS5 index.
func TestNewSQLiteIndex_ReopenKeepsSearchResultsAndRanking(t *testing.T) {
	dir := t.TempDir()
	pages := []IndexPageInput{
		{Path: "docs/title", FilePath: "docs/title.md", PageID: "title", Title: "Leaf term in title", Kind: tree.NodeKindPage, Raw: "Lorem ipsum."},
		{Path: "docs/heading", FilePath: "docs/heading.md", PageID: "heading", Title: "Heading page", Kind: tree.NodeKindPage, Raw: "# Leaf heading\nbody"},
		{Path: "docs/content", FilePath: "docs/content.md", PageID: "content", Title: "Content page", Kind: tree.NodeKindPage, Raw: "Only the body mentions leaf here."},
		{Path: "docs/übersicht", FilePath: "docs/übersicht.md", PageID: "unicode", Title: "Übersicht 🌿", Kind: tree.NodeKindPage, Raw: "Grüße an die Blätter, leaf."},
	}
	queries := []string{"leaf", "übersicht", "Grüße", "title:leaf"}

	first, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	if failures, err := first.IndexPages(pages); err != nil || len(failures) != 0 {
		t.Fatalf("IndexPages: failures=%v err=%v", failures, err)
	}
	before := map[string]*SearchResult{}
	for _, q := range queries {
		res, err := first.Search(q, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q) before reopen: %v", q, err)
		}
		if res.Count == 0 {
			t.Fatalf("Search(%q) before reopen found nothing", q)
		}
		before[q] = res
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(reopened.Close, t)

	for _, q := range queries {
		got, err := reopened.Search(q, nil, 0, 10)
		if err != nil {
			t.Fatalf("Search(%q) after reopen: %v", q, err)
		}
		if !reflect.DeepEqual(got, before[q]) {
			t.Errorf("Search(%q) changed across reopen:\ngot  %+v\nwant %+v", q, got, before[q])
		}
	}
	if got := before["leaf"].Items[0].PageID; got != "title" {
		t.Errorf("bm25 order: first hit for leaf = %q, want title", got)
	}
	if err := reopened.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`INSERT INTO pages(pages) VALUES('integrity-check')`)
		return err
	}); err != nil {
		t.Errorf("FTS5 integrity-check after reopen: %v", err)
	}
}

// Boot runs Clear and a full reindex on the reopened file; doing that twice
// must leave exactly one row per page.
func TestNewSQLiteIndex_ClearAndReindexAfterReopenLeavesNoDuplicates(t *testing.T) {
	dir := t.TempDir()
	for range 3 {
		index, err := NewSQLiteIndex(dir)
		if err != nil {
			t.Fatalf("NewSQLiteIndex: %v", err)
		}
		if err := index.Clear(); err != nil {
			t.Fatalf("Clear: %v", err)
		}
		for _, id := range []string{"a", "b"} {
			if err := index.IndexPage("docs/"+id, "docs/"+id+".md", id, "Page "+id, tree.NodeKindPage, "shared body"); err != nil {
				t.Fatalf("IndexPage: %v", err)
			}
		}
		if err := index.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
	}

	index, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
	res, err := index.Search("shared", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("Search count = %d, want 2", res.Count)
	}
	var rows int
	if err := index.withDB(func(db *sql.DB) error {
		return db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows)
	}); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Errorf("rows = %d, want 2", rows)
	}
}

// copySearchDBFiles copies search.db and its sidecars as they are on disk
// right now, like the state an unclean shutdown leaves behind.
func copySearchDBFiles(t *testing.T, from, to string) {
	t.Helper()
	for _, suffix := range []string{"", "-wal"} {
		data, err := os.ReadFile(filepath.Join(from, "search.db"+suffix))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("read: %v", err)
		}
		if err := os.WriteFile(filepath.Join(to, "search.db"+suffix), data, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
}

// Rows committed only to the -wal (no checkpoint, no Close) survive the next
// open, because the version and schema in the WAL still match.
func TestNewSQLiteIndex_UncheckpointedWALFromUncleanShutdownKeepsRows(t *testing.T) {
	live := t.TempDir()
	index, err := NewSQLiteIndex(live)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
	if err := index.IndexPage("docs/a", "docs/a.md", "a", "A", tree.NodeKindPage, "walonly content"); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}
	if info, err := os.Stat(filepath.Join(live, "search.db-wal")); err != nil || info.Size() == 0 {
		t.Fatalf("expected a non-empty -wal, err=%v", err)
	}

	crashed := t.TempDir()
	copySearchDBFiles(t, live, crashed)

	reopened, err := NewSQLiteIndex(crashed)
	if err != nil {
		t.Fatalf("open after unclean shutdown: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(reopened.Close, t)
	res, err := reopened.Search("walonly", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("Search count = %d, want 1 (rows from the -wal must survive)", res.Count)
	}
}

// A rebuild interrupted before its commit (process killed with the drops
// already spilled into the -wal) leaves the previous committed file; the next
// open sees the old version and rebuilds it cleanly.
func TestNewSQLiteIndex_CrashInTheMiddleOfARebuildRecoversOnNextOpen(t *testing.T) {
	live := t.TempDir()
	indexOnePageAndClose(t, live)
	execRawSearchDB(t, live, `PRAGMA user_version = 99`)

	db, err := sql.Open("sqlite", filepath.Join(live, "search.db")+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(conn.Close, t)
	for _, stmt := range []string{
		`PRAGMA cache_size = 1`, // spill uncommitted pages into the -wal
		`BEGIN`,
		`DROP TABLE pages`,
		`CREATE TABLE filler (b BLOB)`,
		`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 64) INSERT INTO filler SELECT randomblob(4096) FROM n`,
	} {
		if _, err := conn.ExecContext(t.Context(), stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}

	crashed := t.TempDir()
	copySearchDBFiles(t, live, crashed)
	if info, err := os.Stat(filepath.Join(crashed, "search.db-wal")); err != nil || info.Size() == 0 {
		t.Fatalf("expected uncommitted frames in the copied -wal, err=%v", err)
	}

	reopened, err := NewSQLiteIndex(crashed)
	if err != nil {
		t.Fatalf("open after a crash mid-rebuild: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(reopened.Close, t)
	var version, filler int
	var got string
	if err := reopened.withDB(func(db *sql.DB) error {
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
			return err
		}
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'filler'`).Scan(&filler); err != nil {
			return err
		}
		got, err = readSchema(db)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	want, err := expectedSearchSchema()
	if err != nil {
		t.Fatal(err)
	}
	if version != searchSchemaVersion || filler != 0 || got != want {
		t.Errorf("after recovery: version=%d filler=%d schemaMatches=%v", version, filler, got == want)
	}
}

// While another writer holds the lock, opening an outdated file waits
// (busy_timeout) instead of failing, and rebuilds once the lock is released.
//
// This pins BEGIN IMMEDIATE in ensureSchema deterministically: with a
// deferred BEGIN the opener first takes a read snapshot, and SQLite does not
// call the busy handler when such a transaction then needs the write lock, so
// the open fails with SQLITE_BUSY at once instead of waiting.
func TestNewSQLiteIndex_OutdatedFileOpenWaitsForAnotherWriterThenRebuilds(t *testing.T) {
	dir := t.TempDir()
	indexOnePageAndClose(t, dir)
	execRawSearchDB(t, dir, `PRAGMA user_version = 99`)

	locker, err := sql.Open("sqlite", filepath.Join(dir, "search.db")+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(locker.Close, t)
	conn, err := locker.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(conn.Close, t)
	if _, err := conn.ExecContext(t.Context(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatalf("BEGIN IMMEDIATE: %v", err)
	}

	type opened struct {
		index *SQLiteIndex
		err   error
	}
	done := make(chan opened, 1)
	go func() {
		index, err := NewSQLiteIndex(dir)
		done <- opened{index, err}
	}()

	// Well below the 5s busy_timeout: the open must still be waiting.
	select {
	case o := <-done:
		if o.index != nil {
			_ = o.index.Close()
		}
		t.Fatalf("NewSQLiteIndex returned while another writer held the lock: err=%v", o.err)
	case <-time.After(200 * time.Millisecond):
	}

	if _, err := conn.ExecContext(t.Context(), `ROLLBACK`); err != nil {
		t.Fatal(err)
	}

	o := <-done
	if o.err != nil {
		t.Fatalf("NewSQLiteIndex after the lock is released: %v", o.err)
	}
	defer test_utils.WrapCloseWithErrorCheck(o.index.Close, t)
	var version, rows int
	if err := o.index.withDB(func(db *sql.DB) error {
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
			return err
		}
		return db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows)
	}); err != nil {
		t.Fatal(err)
	}
	if version != searchSchemaVersion || rows != 0 {
		t.Errorf("after rebuild: version=%d rows=%d", version, rows)
	}
}

// Running ANALYZE (by hand, or a future PRAGMA optimize) adds sqlite_stat1.
// The file must still be kept on the next open instead of being rebuilt on
// every start.
func TestNewSQLiteIndex_AnalyzeStatsDoNotForceARebuildOnEveryOpen(t *testing.T) {
	dir := t.TempDir()
	indexOnePageAndClose(t, dir)
	execRawSearchDB(t, dir, `ANALYZE`)

	for i := range 2 {
		index, err := NewSQLiteIndex(dir)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		if i == 0 {
			if err := index.IndexPage("docs/b", "docs/b.md", "b", "B", tree.NodeKindPage, "second"); err != nil {
				t.Fatal(err)
			}
		}
		var rows int
		if err := index.withDB(func(db *sql.DB) error {
			return db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows)
		}); err != nil {
			t.Fatal(err)
		}
		if err := index.Close(); err != nil {
			t.Fatal(err)
		}
		if rows != 2 {
			t.Errorf("open %d: rows = %d, want 2 (the index must not be rebuilt although nothing about it changed)", i, rows)
		}
	}
}

// Several SQLiteIndex instances opening the same outdated file at once all
// come up with a usable index at the current schema, and only the first one
// to get the lock rebuilds it; the others see its result and keep it.
func TestNewSQLiteIndex_ConcurrentOpensOfAnOutdatedFile_AllSucceedAndOneRebuilds(t *testing.T) {
	dir := t.TempDir()
	indexOnePageAndClose(t, dir)
	execRawSearchDB(t, dir, `PRAGMA user_version = 0`)

	var logs bytes.Buffer // slog's handler serializes the writes
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)
	indexes := make([]*SQLiteIndex, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			indexes[i], errs[i] = NewSQLiteIndex(dir)
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("open %d: %v", i, errs[i])
		}
		defer test_utils.WrapCloseWithErrorCheck(indexes[i].Close, t)
		if err := indexes[i].IndexPage("docs/x", "docs/x.md", fmt.Sprintf("x%d", i), "X", tree.NodeKindPage, "x"); err != nil {
			t.Errorf("IndexPage on instance %d: %v", i, err)
		}
	}

	if got := strings.Count(logs.String(), "rebuilding it empty"); got != 1 {
		t.Errorf("rebuild logged %d times, want exactly once:\n%s", got, logs.String())
	}
	want, err := expectedSearchSchema()
	if err != nil {
		t.Fatal(err)
	}
	var version, rows int
	var got string
	if err := indexes[0].withDB(func(db *sql.DB) error {
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
			return err
		}
		if err := db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows); err != nil {
			return err
		}
		got, err = readSchema(db)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if version != searchSchemaVersion || got != want {
		t.Errorf("after the opens: version=%d schemaMatches=%v", version, got == want)
	}
	if rows != n {
		t.Errorf("rows = %d, want %d (one page per instance, the old row rebuilt away)", rows, n)
	}
}

// writeLeftoverWithUnknownModule leaves search.db at version 99 with its one
// indexed page and a virtual table whose module no binary has. Its name sorts
// after pages, so a rebuild drops pages first and then fails on it.
func writeLeftoverWithUnknownModule(t *testing.T, dir string) {
	t.Helper()
	indexOnePageAndClose(t, dir)
	// One Exec, so writable_schema applies to the connection that writes.
	execRawSearchDB(t, dir, `
		PRAGMA user_version = 99;
		PRAGMA writable_schema = ON;
		INSERT INTO sqlite_master (type, name, tbl_name, rootpage, sql)
		VALUES ('table', 'zz_leftover', 'zz_leftover', 0, 'CREATE VIRTUAL TABLE zz_leftover USING no_such_module()');
		PRAGMA writable_schema = OFF;`)
}

// A rebuild whose statement fails partway rolls back as a whole: pages is
// dropped first, then the leftover's DROP fails with "no such module". Called
// without NewSQLiteIndex's corruption policy, which would delete the file,
// so the rolled-back file can be inspected.
func TestSQLiteIndex_EnsureSchema_RebuildThatFailsPartwayRollsBackAsAWhole(t *testing.T) {
	dir := t.TempDir()
	writeLeftoverWithUnknownModule(t, dir)

	index := &SQLiteIndex{storageDir: dir, filename: "search.db"}
	err := index.ensureSchema()
	if closeErr := index.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
	if !errors.Is(err, sqliteutil.ErrUnusableDatabase) || !strings.Contains(err.Error(), "no such module") {
		t.Fatalf("ensureSchema err = %v, want ErrUnusableDatabase from the failing DROP", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dir, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)
	var version, rows, leftover int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows); err != nil {
		t.Fatalf("pages after the failed rebuild: %v", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'zz_leftover'`).Scan(&leftover); err != nil {
		t.Fatal(err)
	}
	if version != 99 || rows != 1 || leftover != 1 {
		t.Errorf("after the failed rebuild: version=%d rows=%d leftover=%d, want 99, 1 and 1", version, rows, leftover)
	}
}

// search.db is derived, so a leftover table this binary cannot drop does not
// block startup: RebuildOnCorruption deletes the file with its sidecars and
// the open starts from a fresh, current one.
func TestNewSQLiteIndex_LeftoverTableWithUnknownModuleRebuildsTheFile(t *testing.T) {
	dir := t.TempDir()
	writeLeftoverWithUnknownModule(t, dir)

	index, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	want, err := expectedSearchSchema()
	if err != nil {
		t.Fatal(err)
	}
	var version, rows int
	var got string
	if err := index.withDB(func(db *sql.DB) error {
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
			return err
		}
		if err := db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows); err != nil {
			return err
		}
		got, err = readSchema(db)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if version != searchSchemaVersion || rows != 0 || got != want {
		t.Errorf("after the open: version=%d rows=%d schemaMatches=%v, want a fresh current file", version, rows, got == want)
	}
}

// When another writer keeps the lock past busy_timeout, boot gets a plain
// SQLITE_BUSY error. It is not a corruption error, so RebuildOnCorruption must
// not delete the file, and nothing of the rebuild may be applied.
func TestNewSQLiteIndex_LockHeldPastBusyTimeoutFailsWithoutTouchingTheFile(t *testing.T) {
	dir := t.TempDir()
	indexOnePageAndClose(t, dir)
	execRawSearchDB(t, dir, `PRAGMA user_version = 99`)

	locker, err := sql.Open("sqlite", filepath.Join(dir, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(locker.Close, t)
	conn, err := locker.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(conn.Close, t)
	if _, err := conn.ExecContext(t.Context(), `BEGIN IMMEDIATE`); err != nil {
		t.Fatalf("BEGIN IMMEDIATE: %v", err)
	}

	index, err := NewSQLiteIndex(dir)
	if err == nil {
		_ = index.Close()
		t.Fatal("NewSQLiteIndex succeeded while another writer held the lock past busy_timeout")
	}
	if !strings.Contains(err.Error(), "SQLITE_BUSY") {
		t.Errorf("err = %v, want SQLITE_BUSY", err)
	}
	if sqliteutil.IsSQLiteRecoverableError(err) {
		t.Errorf("SQLITE_BUSY must not count as corruption: %v", err)
	}

	var version, rows int
	if err := conn.QueryRowContext(t.Context(), `PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRowContext(t.Context(), `SELECT count(*) FROM pages`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if version != 99 || rows != 1 {
		t.Errorf("after the timed-out open: version=%d rows=%d, want 99 and 1 (file must stay untouched)", version, rows)
	}
	if _, err := conn.ExecContext(t.Context(), `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
}

// Damage inside the FTS5 shadow tables is not found on open (no
// integrity-check there, ADR-0022); it surfaces on the first query. Clear,
// which boot and resync run before the full reindex, must heal it.
func TestSQLiteIndex_ClearHealsFTS5ShadowTableDamage(t *testing.T) {
	tests := []struct {
		name   string
		damage string
	}{
		{name: "inverted index with missing segments", damage: `DELETE FROM pages_data WHERE id > 10`},
		{name: "missing content rows", damage: `DELETE FROM pages_content`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			indexOnePageAndClose(t, dir)
			execRawSearchDB(t, dir, tt.damage)

			index, err := NewSQLiteIndex(dir)
			if err != nil {
				t.Fatalf("NewSQLiteIndex: %v", err)
			}
			defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
			if _, err := index.Search("persisted", nil, 0, 10); err == nil {
				t.Fatal("Search on the damaged index succeeded; the damage no longer reproduces")
			}

			if err := index.Clear(); err != nil {
				t.Fatalf("Clear: %v", err)
			}
			if err := index.IndexPage("docs/a", "docs/a.md", "a", "A", tree.NodeKindPage, "persisted content"); err != nil {
				t.Fatalf("IndexPage after Clear: %v", err)
			}
			res, err := index.Search("persisted", nil, 0, 10)
			if err != nil {
				t.Fatalf("Search after Clear and reindex: %v", err)
			}
			if res.Count != 1 {
				t.Errorf("Search count = %d, want 1", res.Count)
			}
		})
	}
}

// Clear drops and recreates the tables; searches running at the same time,
// as during boot's reindex, must see the index before or after Clear and
// never fail.
func TestSQLiteIndex_SearchDuringClearDoesNotFail(t *testing.T) {
	index := newTestIndex(t)
	for i := range 20 {
		id := fmt.Sprintf("page-%d", i)
		if err := index.IndexPage(id, id, id, "Title "+id, tree.NodeKindPage, "searchable content"); err != nil {
			t.Fatalf("IndexPage(%s) failed: %v", id, err)
		}
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 60)
	for range 20 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			if err := index.Clear(); err != nil {
				errCh <- fmt.Errorf("Clear: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := index.Search("searchable", nil, 0, 10); err != nil {
				errCh <- fmt.Errorf("Search: %w", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := index.SearchPageIDs("searchable", nil); err != nil {
				errCh <- fmt.Errorf("SearchPageIDs: %w", err)
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent call failed: %v", err)
	}
}

// addUnknownModuleTableLive inserts a virtual table whose module no binary
// has into an index that is open right now, bumping schema_version so the
// open connections see it, the way a write from outside would.
func addUnknownModuleTableLive(t *testing.T, dir string) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer test_utils.WrapCloseWithErrorCheck(db.Close, t)
	db.SetMaxOpenConns(1)
	var cookie int
	if err := db.QueryRow(`PRAGMA schema_version`).Scan(&cookie); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`
		PRAGMA writable_schema = ON;
		INSERT INTO sqlite_master (type, name, tbl_name, rootpage, sql)
		VALUES ('table', 'zz_leftover', 'zz_leftover', 0, 'CREATE VIRTUAL TABLE zz_leftover USING no_such_module()');
		PRAGMA schema_version = %d;
		PRAGMA writable_schema = OFF;`, cookie+1)); err != nil {
		t.Fatal(err)
	}
}

// A Clear that fails partway (pages already dropped, then the leftover's DROP
// fails) rolls back as a whole: the caller gets ErrUnusableDatabase and the
// index keeps answering with its previous rows instead of being half-dropped.
func TestSQLiteIndex_ClearThatFailsPartwayKeepsTheIndexUsable(t *testing.T) {
	dir := t.TempDir()
	index, err := NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
	if err := index.IndexPage("docs/a", "docs/a.md", "a", "A", tree.NodeKindPage, "persisted content"); err != nil {
		t.Fatalf("IndexPage: %v", err)
	}
	addUnknownModuleTableLive(t, dir)

	err = index.Clear()
	if !errors.Is(err, sqliteutil.ErrUnusableDatabase) {
		t.Fatalf("Clear err = %v, want ErrUnusableDatabase", err)
	}
	res, err := index.Search("persisted", nil, 0, 10)
	if err != nil {
		t.Fatalf("Search after the failed Clear: %v", err)
	}
	if res.Count != 1 {
		t.Errorf("Search count after the failed Clear = %d, want 1 (rolled back, not half-dropped)", res.Count)
	}
	if err := index.IndexPage("docs/b", "docs/b.md", "b", "B", tree.NodeKindPage, "persisted too"); err != nil {
		t.Errorf("IndexPage after the failed Clear: %v", err)
	}
}

// writeUnknownModuleTableIntoWAL leaves search.db with its leftover only in
// an uncheckpointed -wal and returns the still-open writer, a second handle.
func writeUnknownModuleTableIntoWAL(t *testing.T, dir string) *sql.DB {
	t.Helper()
	indexOnePageAndClose(t, dir)
	db, err := sql.Open("sqlite", filepath.Join(dir, "search.db")+"?_pragma=journal_mode(WAL)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA wal_autocheckpoint = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
		PRAGMA user_version = 99;
		PRAGMA writable_schema = ON;
		INSERT INTO sqlite_master (type, name, tbl_name, rootpage, sql)
		VALUES ('table', 'zz_leftover', 'zz_leftover', 0, 'CREATE VIRTUAL TABLE zz_leftover USING no_such_module()');
		PRAGMA writable_schema = OFF;`); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "search.db-wal")); err != nil || info.Size() == 0 {
		t.Fatalf("expected the leftover in a non-empty -wal, err=%v", err)
	}
	return db
}

// The unknown-module rebuild also works when the leftover only exists in a
// -wal from an unclean shutdown: the policy deletes the file with that -wal
// and the open starts fresh.
func TestNewSQLiteIndex_UnknownModuleOnlyInAnUncheckpointedWALRebuilds(t *testing.T) {
	live := t.TempDir()
	writer := writeUnknownModuleTableIntoWAL(t, live)
	defer test_utils.WrapCloseWithErrorCheck(writer.Close, t)
	crashed := t.TempDir()
	copySearchDBFiles(t, live, crashed)

	index, err := NewSQLiteIndex(crashed)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
	var version, rows, leftover int
	if err := index.withDB(func(db *sql.DB) error {
		if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
			return err
		}
		if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 'zz_leftover'`).Scan(&leftover); err != nil {
			return err
		}
		return db.QueryRow(`SELECT count(*) FROM pages`).Scan(&rows)
	}); err != nil {
		t.Fatal(err)
	}
	if version != searchSchemaVersion || rows != 0 || leftover != 0 {
		t.Errorf("after the open: version=%d rows=%d leftover=%d, want a fresh current file", version, rows, leftover)
	}
}

// With a second handle still open when the policy deletes the file, Unix
// unlinks it anyway: the new index works on a fresh file and the old handle
// keeps reading the deleted one. Boot holds no other handle, so this only
// pins that the new index is not disturbed by the ghost. On Windows the
// delete fails instead and the open returns that error.
func TestNewSQLiteIndex_UnknownModuleRebuildWithAnotherHandleOpen(t *testing.T) {
	dir := t.TempDir()
	other := writeUnknownModuleTableIntoWAL(t, dir)
	defer test_utils.WrapCloseWithErrorCheck(other.Close, t)

	index, err := NewSQLiteIndex(dir)
	if runtime.GOOS == "windows" {
		if err == nil {
			_ = index.Close()
			t.Fatal("NewSQLiteIndex succeeded although the file could not be deleted")
		}
		return
	}
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)
	if err := index.IndexPage("docs/new", "docs/new.md", "new", "New", tree.NodeKindPage, "freshterm"); err != nil {
		t.Fatalf("IndexPage on the rebuilt file: %v", err)
	}
	res, err := index.Search("freshterm", nil, 0, 10)
	if err != nil || res.Count != 1 {
		t.Fatalf("Search on the rebuilt file: count=%v err=%v", res, err)
	}

	var ghostVersion, ghostNew int
	if err := other.QueryRow(`PRAGMA user_version`).Scan(&ghostVersion); err != nil {
		t.Fatalf("old handle: %v", err)
	}
	if err := other.QueryRow(`SELECT count(*) FROM pages WHERE pageID = 'new'`).Scan(&ghostNew); err != nil {
		t.Fatalf("old handle: %v", err)
	}
	if ghostVersion != 99 || ghostNew != 0 {
		t.Errorf("old handle: version=%d newRows=%d, want 99 and 0 (it reads the deleted file)", ghostVersion, ghostNew)
	}
}

// TestQueryTermCount_MatchesTheTokensSentToFTS5 checks the term count the
// search use case caps against the real tokenizer: the MATCH expression a
// query parses into, minus its syntax (column names, boolean keywords, the
// NEAR keyword and distance), holds exactly as many tokens as QueryTermCount
// reports. An under-count would let a cheap-looking query expand into more
// MATCH terms than the cap allows.
func TestQueryTermCount_MatchesTheTokensSentToFTS5(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE VIRTUAL TABLE probe USING fts5(c, tokenize = "unicode61 tokenchars '` + searchTokenChars + `'")`); err != nil {
		t.Fatalf("create probe table: %v", err)
	}
	if _, err := db.Exec(`CREATE VIRTUAL TABLE probe_vocab USING fts5vocab(probe, instance)`); err != nil {
		t.Fatalf("create vocab table: %v", err)
	}
	columnPrefix := regexp.MustCompile(`(?i)\b(path|filepath|pageid|kind|title|headings|content):`)
	keyword := regexp.MustCompile(`(^|\s)(AND|OR|NOT)(\s|$)`)
	nearCall := regexp.MustCompile(`^NEAR\((.*?)(,\s*\d+)?\)$`)
	tokensSentToFTS5 := func(t *testing.T, fts string) int {
		t.Helper()
		fts = nearCall.ReplaceAllString(fts, "$1")
		fts = columnPrefix.ReplaceAllString(fts, " ")
		// Applied twice: adjacent keywords share the whitespace between them.
		fts = keyword.ReplaceAllString(keyword.ReplaceAllString(fts, " "), " ")
		if _, err := db.Exec(`DELETE FROM probe`); err != nil {
			t.Fatalf("clear probe: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO probe(c) VALUES (?)`, fts); err != nil {
			t.Fatalf("insert probe: %v", err)
		}
		var n int
		if err := db.QueryRow(`SELECT count(*) FROM probe_vocab`).Scan(&n); err != nil {
			t.Fatalf("count tokens: %v", err)
		}
		return n
	}

	tests := []struct {
		name  string
		input string
	}{
		{name: "plain words", input: "foo bar baz"},
		{name: "comma-separated words", input: "a,b,c,d,e,f,g,h,i,j,k,l,m"},
		{name: "token punctuation keeps one word", input: "foo-bar-baz.v2/x#y+z"},
		{name: "apostrophes and quotes inside words", input: `it's abc"def`},
		{name: "column filters", input: "title:a title:b content:c,d"},
		{name: "unknown column", input: "nope:a,b"},
		{name: "nested groups with prefixes", input: "((a OR b) AND (c OR d*))"},
		{name: "quoted phrase", input: `"a b c d e f g h i j k l m"`},
		{name: "quoted phrase with escaped quote", input: `"a""b c"`},
		{name: "mixed operators, filters and phrases", input: `(a OR b) NOT title:c,d "e f" g*`},
		{name: "CJK runs", input: "日本語 中文 한국어"},
		{name: "emoji between words", input: "🙂foo🙂bar🙂"},
		{name: "NEAR call without distance", input: "NEAR(a b c)"},
		{name: "dangling and doubled keywords", input: "AND a AND AND b NOT"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fts := buildFuzzyQuery(tc.input)
			if got, want := QueryTermCount(tc.input), tokensSentToFTS5(t, fts); got != want {
				t.Errorf("QueryTermCount(%q) = %d, but its MATCH expression %q holds %d tokens", tc.input, got, fts, want)
			}
		})
	}
}

// TestSQLiteIndex_Search_QueryAtTheTermCapStillFindsThePage: the use case
// lets through queries of up to 12 terms; such a query, literal or mixed with
// operators, must still match and rank normally rather than hit an FTS5 limit.
func TestSQLiteIndex_Search_QueryAtTheTermCapStillFindsThePage(t *testing.T) {
	index := newTestIndex(t)
	content := "alpha bravo charlie delta echo foxtrot golf hotel india juliet kilo lima"
	if err := index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alphabet", tree.NodeKindPage, content); err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}
	if err := index.IndexPage("notes/other", "notes/other.md", "other", "Other", tree.NodeKindPage, "alpha bravo"); err != nil {
		t.Fatalf("failed to index other page: %v", err)
	}

	tests := []struct {
		name  string
		query string
	}{
		{name: "12 literal words", query: content},
		{name: "12 comma-separated words", query: strings.ReplaceAll(content, " ", ",")},
		{name: "12 terms mixing operators, filters and phrases", query: `(alpha OR zulu) AND content:bravo,charlie AND "delta echo" foxtrot* golf hotel india juliet kilo`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryTermCount(tc.query); got != 12 {
				t.Fatalf("QueryTermCount(%q) = %d, want 12", tc.query, got)
			}
			result, err := index.Search(tc.query, nil, 0, 10)
			if err != nil {
				t.Fatalf("Search(%q) failed: %v", tc.query, err)
			}
			if result.Count != 1 || len(result.Items) != 1 || result.Items[0].PageID != "alpha1" {
				t.Errorf("Search(%q) = count %d, want exactly the alpha page", tc.query, result.Count)
			}
		})
	}
}
