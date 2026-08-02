package search

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func TestSQLiteIndex_IndexPage(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	// Testdata
	path := "docs/test.md"
	pageID := "test123"
	title := "Test Page"
	content := "This is a **test** page."
	expectedContent := "This is a test page."

	err = index.IndexPage(path, path, pageID, title, tree.NodeKindPage, content)
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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
	tmpDir := t.TempDir()
	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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
	tmpDir := t.TempDir()
	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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
	tmpDir := t.TempDir()
	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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
	tmpDir := t.TempDir()
	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	// Index two pages
	err = index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha Search Test", tree.NodeKindPage, "This content is about SQLite search.")
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

func TestSQLiteIndex_Search_RanksTitleMatchHigherThanContent(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	// page with match in title
	err = index.IndexPage(
		"docs/titleMatch",
		"docs/titleMatch.md",
		"titleMatch",
		"Search term in title",
		tree.NodeKindPage,
		"Lorem ipsum dolor sit amet.",
	)
	if err != nil {
		t.Fatalf("failed to index titleMatch page: %v", err)
	}

	// page with match only in content
	err = index.IndexPage(
		"docs/contentMatch",
		"docs/contentMatch.md",
		"contentMatch",
		"Content only match",
		tree.NodeKindPage,
		"This page has the search term only in the content.",
	)
	if err != nil {
		t.Fatalf("failed to index contentMatch page: %v", err)
	}

	// "search" is converted by buildFuzzyQuery to "search*", matching both
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

	// Title match should be ranked higher than content match
	if result.Items[0].PageID != "titleMatch" {
		t.Errorf("expected titleMatch to be ranked first, got %s", result.Items[0].PageID)
	}

	// and the rank value should be higher (because 1/(1+score), score smaller)
	if result.Items[0].Rank < result.Items[1].Rank {
		t.Errorf("expected higher rank for titleMatch (got %f, %f)", result.Items[0].Rank, result.Items[1].Rank)
	}

	// sanity check: Ranks should be > 0 and <= 1
	for i, item := range result.Items {
		if item.Rank <= 0 || item.Rank > 1 {
			t.Errorf("expected rank for item %d to be in (0,1], got %f", i, item.Rank)
		}
	}
}

func TestSQLiteIndex_Search_EscapesTitleForContentOnlyMatch(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	maliciousTitle := `<img src=x onerror="alert(1)">`
	err = index.IndexPage(
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	maliciousTitle := `<script>alert(1)</script> Search`
	err = index.IndexPage(
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	maliciousTitle := `<svg onload=alert(1)>`
	err = index.IndexPage(
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

func TestSQLiteIndex_Search_RanksHeadingHigherThanContent(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	// page with match in heading (Markdown heading)
	err = index.IndexPage(
		"docs/headingMatch",
		"docs/headingMatch.md",
		"headingMatch",
		"No search in title",
		tree.NodeKindPage,
		"## Search term in heading\n\nSome additional body text.",
	)
	if err != nil {
		t.Fatalf("failed to index headingMatch page: %v", err)
	}

	// page with match only in content
	err = index.IndexPage(
		"docs/contentOnly",
		"docs/contentOnly.md",
		"contentOnly",
		"No search in title",
		tree.NodeKindPage,
		"This page has the search term only in the content.",
	)
	if err != nil {
		t.Fatalf("failed to index contentOnly page: %v", err)
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

	// Heading match should be ranked higher than content match
	if result.Items[0].PageID != "headingMatch" {
		t.Errorf("expected headingMatch to be ranked first, got %s", result.Items[0].PageID)
	}

	if result.Items[0].Rank < result.Items[1].Rank {
		t.Errorf("expected higher rank for headingMatch (got %f, %f)", result.Items[0].Rank, result.Items[1].Rank)
	}
}

func TestSQLiteIndex_SearchPageIDs_RespectsQueryAndPageFilters(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	err = index.IndexPage("docs/alpha", "docs/alpha.md", "alpha", "Alpha Page", tree.NodeKindPage, "Shared token in alpha.")
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	err = index.IndexPage(
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	err = index.IndexPage(
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	err = index.IndexPage(
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

	err = index.IndexPage(
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
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer test_utils.WrapCloseWithErrorCheck(index.Close, t)

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

	err = index.IndexPage("docs/code-example", "docs/code-example.md", "code1", "Example Test Page with a code block.", tree.NodeKindPage, content)
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
