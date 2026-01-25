package importer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZipExtractor_ValidateExtractedFiles(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	zipPath := "fixtures/fixture-1.zip"

	extractor := NewZipExtractor()
	ws, err := extractor.ExtractToTemp(filepath.Join(currentDir, zipPath))
	if err != nil {
		t.Fatalf("ExtractToTemp failed: %v", err)
	}
	defer ws.Cleanup()

	// Check if expected files exist
	expectedFiles := []string{
		"home.md",
		"features/index.md",
		"features/mermaind.md",
	}

	for _, relPath := range expectedFiles {
		fullPath := filepath.Join(ws.Root, relPath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", relPath)
		}
	}
}

func TestZipExtractor_Cleanup(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	zipPath := "fixtures/fixture-1.zip"

	extractor := NewZipExtractor()
	ws, err := extractor.ExtractToTemp(filepath.Join(currentDir, zipPath))
	if err != nil {
		t.Fatalf("ExtractToTemp failed: %v", err)
	}

	workspaceRoot := ws.Root

	// Cleanup
	if err := ws.Cleanup(); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify cleanup
	if _, err := os.Stat(workspaceRoot); !os.IsNotExist(err) {
		t.Errorf("Workspace root %s still exists after cleanup", workspaceRoot)
	}
}
