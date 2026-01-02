package search

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	_ "modernc.org/sqlite" // Import SQLite driver
)

func TestBuildAndRunIndexer_BasicIndexing(t *testing.T) {
	tmp := t.TempDir()

	treeSvc := tree.NewTreeService(tmp)
	if err := treeSvc.LoadTree(); err != nil {
		t.Fatalf("failed to load tree: %v", err)
	}

	_, err := treeSvc.CreatePage("system", nil, "Docs", "docs")
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

	status := NewIndexingStatus()

	corePath := filepath.Join(tmp, "root")
	err = BuildAndRunIndexer(treeSvc, index, corePath, 2, status)
	if err != nil {
		t.Fatalf("BuildAndRunIndexer failed: %v", err)
	}

	var title, text string

	if err := index.withDB(func(db *sql.DB) error {

		row := db.QueryRow(`SELECT title, content FROM pages WHERE filePath = ?`, "docs.md")
		if err := row.Scan(&title, &text); err != nil {
			return err
		}
		return nil

	}); err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if title != "Docs" {
		t.Errorf("expected title 'Docs', got %q", title)
	}
	if !strings.Contains(text, "Hello Search") {
		t.Errorf("expected content to contain 'Hello Search', got %q", text)
	}

	snap := status.Snapshot()
	if snap.Active {
		t.Errorf("expected indexing to be inactive, got active")
	}

	if snap.Indexed < 1 {
		t.Errorf("expected at least 1 indexed page, got %d", snap.Indexed)
	}

}
