package search

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

func TestBuildAndRunIndexer_BasicIndexing(t *testing.T) {
	tmp := t.TempDir()

	treeSvc := tree.NewTreeService(tmp)
	if err := treeSvc.LoadTree(); err != nil {
		t.Fatalf("failed to load tree: %v", err)
	}

	_, err := treeSvc.CreatePage(nil, "Docs", "docs")
	if err != nil {
		t.Fatalf("CreatePage failed: %v", err)
	}

	mdPath := filepath.Join(tmp, "root", "docs.md")
	content := "# Hello Search\nSome content."
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write .md file: %v", err)
	}

	index, err := NewSQLiteIndex(tmp)
	if err != nil {
		t.Fatalf("Failed to init SQLiteIndex: %v", err)
	}
	defer index.Close()

	corePath := filepath.Join(tmp, "root")
	err = BuildAndRunIndexer(treeSvc, index, corePath, 2)
	if err != nil {
		t.Fatalf("BuildAndRunIndexer failed: %v", err)
	}

	row := index.GetDB().QueryRow(`SELECT title, content FROM pages WHERE path = ?`, "docs.md")
	var title, text string
	if err := row.Scan(&title, &text); err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if title != "Docs" {
		t.Errorf("expected title 'Docs', got %q", title)
	}
	if !strings.Contains(text, "Hello Search") {
		t.Errorf("expected content to contain 'Hello Search', got %q", text)
	}
}
