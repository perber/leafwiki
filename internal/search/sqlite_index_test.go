package search

import (
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite" // Import SQLite driver
)

func TestSQLiteIndex_IndexPage(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// Testdata
	path := "docs/test.md"
	pageID := "test123"
	title := "Test Page"
	content := "This is a **test** page."
	expectedContent := "This is a test page."

	err = index.IndexPage(path, path, pageID, title, content)
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

func TestSQLiteIndex_Search(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// Index two pages
	err = index.IndexPage("notes/alpha", "notes/alpha.md", "alpha1", "Alpha Search Test", "This content is about SQLite search.")
	if err != nil {
		t.Fatalf("failed to index alpha page: %v", err)
	}

	err = index.IndexPage("notes/beta", "notes/beta.md", "beta2", "Unrelated Page", "This content is not about the search term.")
	if err != nil {
		t.Fatalf("failed to index beta page: %v", err)
	}

	// Perform search
	result, err := index.Search("content:search", 0, 10)
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
}

func TestSQLiteIndex_Search_RanksTitleMatchHigherThanContent(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// page with match in title
	err = index.IndexPage(
		"docs/titleMatch",
		"docs/titleMatch.md",
		"titleMatch",
		"Search term in title",
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
		"This page has the search term only in the content.",
	)
	if err != nil {
		t.Fatalf("failed to index contentMatch page: %v", err)
	}

	// "search" is converted by buildFuzzyQuery to "search*", matching both
	result, err := index.Search("search", 0, 10)
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
	defer index.Close()

	// page with match in heading (Markdown heading)
	err = index.IndexPage(
		"docs/headingMatch",
		"docs/headingMatch.md",
		"headingMatch",
		"No search in title",
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
		"This page has the search term only in the content.",
	)
	if err != nil {
		t.Fatalf("failed to index contentOnly page: %v", err)
	}

	result, err := index.Search("search", 0, 10)
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

func TestSQLiteIndex_Search_WithHyphenatedTerms(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// Index pages with hyphenated terms
	err = index.IndexPage(
		"docs/testing",
		"docs/testing.md",
		"testing1",
		"Testing Guide",
		"This page describes test-case scenarios and test-driven development.",
	)
	if err != nil {
		t.Fatalf("failed to index testing page: %v", err)
	}

	err = index.IndexPage(
		"docs/other",
		"docs/other.md",
		"other1",
		"Other Page",
		"This page has no hyphenated testing terms.",
	)
	if err != nil {
		t.Fatalf("failed to index other page: %v", err)
	}

	// Search for hyphenated term
	result, err := index.Search("test-case", 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Should find the page with "test-case"
	if result.Count != 1 {
		t.Errorf("expected 1 result for 'test-case', got %d", result.Count)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	if result.Items[0].PageID != "testing1" {
		t.Errorf("expected testing1 to be found, got %s", result.Items[0].PageID)
	}
}

func TestSQLiteIndex_Search_WithDotsInFilenames(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// Index pages with filenames containing dots
	err = index.IndexPage(
		"docs/files",
		"docs/files.md",
		"files1",
		"File Documentation",
		"You can download the config.yaml or script.sh files from the repository.",
	)
	if err != nil {
		t.Fatalf("failed to index files page: %v", err)
	}

	err = index.IndexPage(
		"docs/setup",
		"docs/setup.md",
		"setup1",
		"Setup Guide",
		"This guide shows how to set up the application.",
	)
	if err != nil {
		t.Fatalf("failed to index setup page: %v", err)
	}

	// Search for filename with dot
	result, err := index.Search("config.yaml", 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Should find the page with "config.yaml"
	if result.Count != 1 {
		t.Errorf("expected 1 result for 'config.yaml', got %d", result.Count)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	if result.Items[0].PageID != "files1" {
		t.Errorf("expected files1 to be found, got %s", result.Items[0].PageID)
	}
}

func TestSQLiteIndex_Search_WithPlusSignsInTerms(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// Index pages with programming language mentions
	err = index.IndexPage(
		"docs/cpp",
		"docs/cpp.md",
		"cpp1",
		"C++ Programming",
		"This guide covers c++ development and best practices.",
	)
	if err != nil {
		t.Fatalf("failed to index cpp page: %v", err)
	}

	err = index.IndexPage(
		"docs/python",
		"docs/python.md",
		"python1",
		"Python Programming",
		"This guide covers Python development.",
	)
	if err != nil {
		t.Fatalf("failed to index python page: %v", err)
	}

	// Search for term with plus signs
	result, err := index.Search("c++", 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Should find the page with "c++"
	if result.Count != 1 {
		t.Errorf("expected 1 result for 'c++', got %d", result.Count)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	if result.Items[0].PageID != "cpp1" {
		t.Errorf("expected cpp1 to be found, got %s", result.Items[0].PageID)
	}
}

func TestSQLiteIndex_Search_WithSlashesInPaths(t *testing.T) {
	tmpDir := t.TempDir()

	index, err := NewSQLiteIndex(tmpDir)
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	defer index.Close()

	// Index pages with path references
	err = index.IndexPage(
		"docs/paths",
		"docs/paths.md",
		"paths1",
		"Path Documentation",
		"The configuration is located at /etc/config/app.conf and data at /var/data/files.",
	)
	if err != nil {
		t.Fatalf("failed to index paths page: %v", err)
	}

	err = index.IndexPage(
		"docs/general",
		"docs/general.md",
		"general1",
		"General Info",
		"This is general information about the application.",
	)
	if err != nil {
		t.Fatalf("failed to index general page: %v", err)
	}

	// Search for path with slashes
	result, err := index.Search("/etc/config/app.conf", 0, 10)
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	// Should find the page with the path
	if result.Count != 1 {
		t.Errorf("expected 1 result for '/etc/config/app.conf', got %d", result.Count)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %d", len(result.Items))
	}

	if result.Items[0].PageID != "paths1" {
		t.Errorf("expected paths1 to be found, got %s", result.Items[0].PageID)
	}
}
