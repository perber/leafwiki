package search

import (
	"os"
	"path/filepath"
	"testing"
)

// Hilfsfunktion: Testdatenstruktur anlegen
func createTestFiles(t *testing.T, root string, files map[string]string) {
	for relPath, content := range files {
		fullPath := filepath.Join(root, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
}

func TestIndexer_BasicIndexing(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"index.md":            "# Home",
		"subdir/about.md":     "About page",
		"subdir/ignore.txt":   "not markdown",
		"deep/nested/info.md": "deep info",
	}

	createTestFiles(t, tmpDir, files)

	var indexed []string
	indexer := NewIndexer(tmpDir, 4, func(path string, content []byte) error {
		indexed = append(indexed, path)
		return nil
	})

	err := indexer.Start()
	if err != nil {
		t.Fatalf("indexer failed: %v", err)
	}

	// Erwartete Dateien
	expected := []string{
		filepath.Join(tmpDir, "index.md"),
		filepath.Join(tmpDir, "subdir/about.md"),
		filepath.Join(tmpDir, "deep/nested/info.md"),
	}

	for _, want := range expected {
		found := false
		for _, got := range indexed {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be indexed", want)
		}
	}
}
