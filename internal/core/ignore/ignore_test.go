package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// loadIgnore is a test helper that writes content to a .leafwikiignore file
// in tmp and returns the parsed IgnoreFile.
func loadIgnore(t *testing.T, tmpDir string, content string) *IgnoreFile {
	t.Helper()
	if err := os.WriteFile(filepath.Join(tmpDir, IgnoreFilename), []byte(content), 0o644); err != nil {
		t.Fatalf("write .leafwikiignore: %v", err)
	}
	ig, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	return ig
}

// --- Task 1.1: Missing file returns nil ---

func TestLoadFromDir_MissingFile_ReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	ig, err := LoadFromDir(tmp)
	if err != nil {
		t.Fatalf("LoadFromDir with no .leafwikiignore: %v", err)
	}
	if ig != nil {
		t.Fatalf("expected nil, got %v", ig)
	}
}

// --- Task 1.2: Empty file matches nothing ---

func TestMatches_EmptyFile_MatchesNothing(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile for empty file")
	}
	if ig.Matches("any.md", false) {
		t.Fatal("expected empty file to match nothing")
	}
	if ig.Matches("some/dir", true) {
		t.Fatal("expected empty file to match nothing for dir")
	}
}

// --- Task 1.3: Simple file pattern ---

func TestMatches_SimpleFilePattern(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "*.log")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("debug.log", false) {
		t.Fatal("expected *.log to match debug.log")
	}
	if ig.Matches("notes.md", false) {
		t.Fatal("expected *.log to NOT match notes.md")
	}
}

// --- Task 1.4: Directory-only pattern ---

func TestMatches_DirectoryOnlyPattern(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "drafts/")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("drafts", true) {
		t.Fatal("expected drafts/ to match drafts directory")
	}
	if ig.Matches("drafts", false) {
		t.Fatal("expected drafts/ to NOT match drafts file")
	}
}

// --- Task 1.5: Negation ---

func TestMatches_Negation(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "*.md\n!important.md")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("readme.md", false) {
		t.Fatal("expected *.md to match readme.md")
	}
	if ig.Matches("important.md", false) {
		t.Fatal("expected !important.md to un-ignore important.md")
	}
}

// --- Task 1.6: Root-anchored ---

func TestMatches_RootAnchored(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "/build/")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("build", true) {
		t.Fatal("expected /build/ to match root-level build/")
	}
	if ig.Matches("docs/build", true) {
		t.Fatal("expected /build/ to NOT match nested build/")
	}
}

// --- Task 1.7: Globstar ---

func TestMatches_Globstar(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "temp/**/notes.md")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("temp/a/b/notes.md", false) {
		t.Fatal("expected temp/**/notes.md to match nested path")
	}
	if ig.Matches("notes.md", false) {
		t.Fatal("expected temp/**/notes.md to NOT match root notes.md")
	}
}

// --- Task 1.8: Comments and blank lines ---

func TestMatches_CommentsAndBlanks(t *testing.T) {
	tmp := t.TempDir()
	ig := loadIgnore(t, tmp, "# comment\n\n*.md\n")
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("test.md", false) {
		t.Fatal("expected *.md to match test.md")
	}
	if ig.Matches("test.log", false) {
		t.Fatal("expected *.md to NOT match test.log")
	}
}

// --- PatternCount ---

func TestPatternCount(t *testing.T) {
	t.Run("nil returns 0", func(t *testing.T) {
		var ig *IgnoreFile
		if got := ig.PatternCount(); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})

	t.Run("counts non-comment non-blank lines", func(t *testing.T) {
		tmp := t.TempDir()
		ig := loadIgnore(t, tmp, "# comment\n\n*.log\n*.tmp\n")
		if ig == nil {
			t.Fatal("expected non-nil IgnoreFile")
		}
		if got := ig.PatternCount(); got != 2 {
			t.Fatalf("expected 2 patterns, got %d", got)
		}
	})
}

// --- Error cases ---

func TestLoadFromDir_ErrorOnDirectory(t *testing.T) {
	tmp := t.TempDir()
	dirPath := filepath.Join(tmp, IgnoreFilename)
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_, err := LoadFromDir(tmp)
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("expected error about directory, got %v", err)
	}
}

func TestLoadFromDir_ErrorOnInvalidPattern(t *testing.T) {
	tmp := t.TempDir()
	// Write a file with a regex-invalid character that might cause issues
	content := "[invalid\n"
	if err := os.WriteFile(filepath.Join(tmp, IgnoreFilename), []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// The library is lenient with patterns — it just won't produce a match.
	// LoadFromDir should still succeed.
	ig, err := LoadFromDir(tmp)
	if err != nil {
		t.Fatalf("LoadFromDir with potentially invalid pattern: %v", err)
	}
	if ig == nil {
		t.Fatal("expected non-nil even with questionable patterns")
	}
}
