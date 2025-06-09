package search

import (
	"strings"
	"testing"
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

	row := index.GetDB().QueryRow(`SELECT path, title, content FROM pages WHERE pageID = ?`, pageID)

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

	item := result.Items[0]
	if item.PageID != "alpha1" {
		t.Errorf("expected PageID alpha1, got %s", item.PageID)
	}
}
