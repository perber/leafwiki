package search

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
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
