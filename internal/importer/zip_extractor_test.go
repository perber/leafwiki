package importer

import (
	"archive/zip"
	"errors"
	"fmt"
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

func TestZipExtractor_ExtractToDirWithLimits_RejectsTooManyEntriesBeforeExtracting(t *testing.T) {
	entries := map[string]string{}
	for i := 0; i < 11; i++ {
		entries[fmt.Sprintf("dir%d/page.md", i)] = ""
	}
	zipPath := writeRawZip(t, entries)
	limits := shared.DefaultZipExtractionLimits
	limits.MaxEntries = 10
	baseDir := t.TempDir()

	ws, err := NewZipExtractor().extractToDirWithLimits(zipPath, baseDir, limits)
	if ws != nil {
		defer func() { _ = ws.Cleanup() }()
	}
	if !errors.Is(err, shared.ErrTooManyEntries) {
		t.Fatalf("expected ErrTooManyEntries, got: %v", err)
	}
	left, err := os.ReadDir(baseDir)
	if err != nil {
		t.Fatalf("ReadDir(baseDir) error: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("rejected archive left %d entries in the base dir, want none", len(left))
	}
}

// writeEmptyEntriesZip writes a zip whose entries are the given names, each
// with no content, in order. Names ending in "/" become directory entries.
func writeEmptyEntriesZip(t *testing.T, names []string) string {
	t.Helper()
	zipPath := filepath.Join(t.TempDir(), "entries.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("failed to create zip: %v", err)
	}
	w := zip.NewWriter(f)
	for _, name := range names {
		if _, err := w.Create(name); err != nil {
			t.Fatalf("failed to create zip entry %s: %v", name, err)
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

func numberedNames(format string, n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf(format, i)
	}
	return names
}

// TestZipExtractor_EntryCapBoundaries pins which entries count toward
// MaxEntries (every central-directory entry, including directories and
// every distinct parent directory an entry path implies, and
// blank names the extractor skips) and that ExtractToDir applies the
// production default of DefaultZipExtractionLimits.MaxEntries.
func TestZipExtractor_EntryCapBoundaries(t *testing.T) {
	defaultCap := shared.DefaultZipExtractionLimits.MaxEntries
	tests := []struct {
		name       string
		names      []string
		maxEntries int // 0 means: call ExtractToDir with the production defaults
		wantReject bool
	}{
		{"exactly at the cap", numberedNames("page%d.md", 10), 10, false},
		{"directory entries count toward the cap", append(numberedNames("dir%d/", 6), numberedNames("page%d.md", 5)...), 10, true},
		{"only directory entries above the cap", numberedNames("dir%d/", 11), 10, true},
		{"blank-named entries count toward the cap", append(numberedNames("page%d.md", 10), " "), 10, true},
		{"parent directories of one deep entry count", []string{"a/b/c/d/e/f/g/h/i/j/page.md"}, 10, true},
		{"a shared parent directory counts once", numberedNames("dir/page%d.md", 9), 10, false},
		{"distinct deep parents count separately", []string{"x/d/d/d/page.md", "y/d/d/d/page.md", "z/d/d/d/page.md"}, 10, true},
		{"default cap: exactly at the cap", numberedNames("dir%d/", defaultCap), 0, false},
		{"default cap: one above the cap", numberedNames("dir%d/", defaultCap+1), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipPath := writeEmptyEntriesZip(t, tt.names)
			baseDir := t.TempDir()

			var ws *ZipWorkspace
			var err error
			if tt.maxEntries == 0 {
				ws, err = NewZipExtractor().ExtractToDir(zipPath, baseDir)
			} else {
				limits := shared.DefaultZipExtractionLimits
				limits.MaxEntries = tt.maxEntries
				ws, err = NewZipExtractor().extractToDirWithLimits(zipPath, baseDir, limits)
			}
			if ws != nil {
				defer func() { _ = ws.Cleanup() }()
			}
			if tt.wantReject {
				if !errors.Is(err, shared.ErrTooManyEntries) {
					t.Fatalf("expected ErrTooManyEntries, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the archive to be accepted, got: %v", err)
			}
		})
	}
}
