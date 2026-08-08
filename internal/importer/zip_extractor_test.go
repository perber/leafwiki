package importer

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/shared"
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
	defer func() {
		if err := ws.Cleanup(); err != nil {
			t.Fatalf("Cleanup failed: %v", err)
		}
	}()

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

// writeRawZip writes a zip file with exactly the given entries (name ->
// content) for tests that need to control the zip's contents precisely,
// same shape as internal/restore/swap_test.go's helper.
func writeRawZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "fixture.zip")

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		zw, err := w.Create(name)
		if err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
		}
		if _, err := zw.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write zip entry %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("failed to close zip file: %v", err)
	}
	return zipPath
}

// Regression tests for the zip-bomb unbounded-decompression DoS
// (CWE-409): extractToDirWithLimits must enforce a per-file cap, a
// cumulative-per-archive cap, and a decompression-ratio cap, instead of
// the raw io.Copy this used to be.

func TestZipExtractor_ExtractToDirWithLimits_RejectsEntryLargerThanPerFileLimit(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"big.txt": strings.Repeat("A", 200),
	})
	limits := shared.ExtractionLimits{
		MaxEntryBytes:   100,
		MaxTotalBytes:   1 << 30,
		MaxRatio:        1 << 30,
		RatioFloorBytes: 1 << 30,
	}

	extractor := NewZipExtractor()
	ws, err := extractor.extractToDirWithLimits(zipPath, t.TempDir(), limits)
	if ws != nil {
		defer func() { _ = ws.Cleanup() }()
	}
	if !errors.Is(err, shared.ErrFileTooLarge) {
		t.Fatalf("expected ErrFileTooLarge, got: %v", err)
	}
}

func TestZipExtractor_ExtractToDirWithLimits_RejectsManyFilesSummingPastCumulativeLimit(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"a.txt": strings.Repeat("A", 60),
		"b.txt": strings.Repeat("B", 60),
		"c.txt": strings.Repeat("C", 60),
	})
	limits := shared.ExtractionLimits{
		MaxEntryBytes:   1 << 30,
		MaxTotalBytes:   100,
		MaxRatio:        1 << 30,
		RatioFloorBytes: 1 << 30,
	}

	extractor := NewZipExtractor()
	ws, err := extractor.extractToDirWithLimits(zipPath, t.TempDir(), limits)
	if ws != nil {
		defer func() { _ = ws.Cleanup() }()
	}
	if !errors.Is(err, shared.ErrCumulativeSizeTooLarge) {
		t.Fatalf("expected ErrCumulativeSizeTooLarge, got: %v", err)
	}
}

func TestZipExtractor_ExtractToDirWithLimits_RejectsHighDecompressionRatio(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"bomb.txt": strings.Repeat("A", 5000),
	})
	limits := shared.ExtractionLimits{
		MaxEntryBytes:   1 << 30,
		MaxTotalBytes:   1 << 30,
		MaxRatio:        3,
		RatioFloorBytes: 50,
	}

	extractor := NewZipExtractor()
	ws, err := extractor.extractToDirWithLimits(zipPath, t.TempDir(), limits)
	if ws != nil {
		defer func() { _ = ws.Cleanup() }()
	}
	if !errors.Is(err, shared.ErrDecompressionRatioTooHigh) {
		t.Fatalf("expected ErrDecompressionRatioTooHigh, got: %v", err)
	}
}

func TestZipExtractor_ExtractToDirWithLimits_AllowsLowRatioContentUnderFloor(t *testing.T) {
	zipPath := writeRawZip(t, map[string]string{
		"small.txt": strings.Repeat("A", 40),
	})
	limits := shared.ExtractionLimits{
		MaxEntryBytes:   1 << 30,
		MaxTotalBytes:   1 << 30,
		MaxRatio:        3,
		RatioFloorBytes: 50,
	}

	extractor := NewZipExtractor()
	ws, err := extractor.extractToDirWithLimits(zipPath, t.TempDir(), limits)
	if err != nil {
		t.Fatalf("expected success for small under-floor content, got: %v", err)
	}
	defer func() { _ = ws.Cleanup() }()
}
