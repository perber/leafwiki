package search

import (
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

	err = index.IndexPage(path, pageID, title, content)
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
	if gotContent != content {
		t.Errorf("expected content %s, got %s", content, gotContent)
	}
}
