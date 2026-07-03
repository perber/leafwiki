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

// --- Task 1.1: CompileLines ---

func TestCompileLines(t *testing.T) {
	ig := CompileLines([]string{"*.md", "!important.md"})
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("readme.md", false) {
		t.Fatal("expected *.md to match readme.md")
	}
	if ig.Matches("important.md", false) {
		t.Fatal("expected !important.md to un-ignore important.md")
	}
	if ig.Matches("notes.txt", false) {
		t.Fatal("expected no pattern to match notes.txt")
	}
	if ig.PatternCount() != 2 {
		t.Fatalf("expected 2 patterns, got %d", ig.PatternCount())
	}
}

// --- Task 1.2: Cache — empty root returns nil ---

func TestCache_NoIgnoreFiles_ReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	subdir := filepath.Join(root, "docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	c := NewCache(root)
	ig := c.Get(subdir)
	if ig != nil {
		t.Fatalf("expected nil, got %v", ig)
	}

	// Root itself also returns nil
	ig = c.Get(root)
	if ig != nil {
		t.Fatalf("expected nil for root, got %v", ig)
	}
}

// --- Task 1.3: Cache — root-only ignore file ---

func TestCache_RootOnly(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	subdir := filepath.Join(root, "docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Write .leafwikiignore at root only
	if err := os.WriteFile(filepath.Join(root, IgnoreFilename), []byte("*.log"), 0o644); err != nil {
		t.Fatalf("write root .leafwikiignore: %v", err)
	}

	c := NewCache(root)

	// Get for subdir should return matcher with root patterns
	ig := c.Get(subdir)
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	if !ig.Matches("debug.log", false) {
		t.Fatal("expected *.log to match debug.log")
	}
	if ig.Matches("notes.md", false) {
		t.Fatal("expected *.log to NOT match notes.md")
	}
	if ig.PatternCount() != 1 {
		t.Fatalf("expected 1 pattern, got %d", ig.PatternCount())
	}

	// Root also gets its own patterns
	ig = c.Get(root)
	if ig == nil {
		t.Fatal("expected non-nil for root")
	}
}

// --- Task 1.4: Cache — multi-level accumulation ---

func TestCache_MultiLevel(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	subdir := filepath.Join(root, "docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Root: ignore all .md files. Subdir: un-ignore important.md
	if err := os.WriteFile(filepath.Join(root, IgnoreFilename), []byte("*.md"), 0o644); err != nil {
		t.Fatalf("write root .leafwikiignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir, IgnoreFilename), []byte("!important.md"), 0o644); err != nil {
		t.Fatalf("write docs .leafwikiignore: %v", err)
	}

	c := NewCache(root)

	// Get for subdir should have both patterns: root *.md + docs !important.md
	ig := c.Get(subdir)
	if ig == nil {
		t.Fatal("expected non-nil IgnoreFile")
	}
	// Root *.md still matches non-important md files in subdir
	if !ig.Matches("docs/readme.md", false) {
		t.Fatal("expected *.md to match docs/readme.md")
	}
	// Negation in subdir un-ignores important.md
	if ig.Matches("docs/important.md", false) {
		t.Fatal("expected !important.md to un-ignore docs/important.md")
	}
	// Root-only patterns still apply for non-md files? No, *.md is the only pattern
	if ig.Matches("docs/notes.txt", false) {
		t.Fatal("expected no pattern to match docs/notes.txt")
	}
}

// --- Task 1.5: Cache — child-only ignore ---

func TestCache_ChildOnly(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	subdir := filepath.Join(root, "docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Only write .leafwikiignore in child, not in root
	if err := os.WriteFile(filepath.Join(subdir, IgnoreFilename), []byte("*.log"), 0o644); err != nil {
		t.Fatalf("write docs .leafwikiignore: %v", err)
	}

	c := NewCache(root)

	// Root has no .leafwikiignore → nil
	ig := c.Get(root)
	if ig != nil {
		t.Fatalf("expected nil for root, got %v", ig)
	}

	// Subdir has its own patterns
	ig = c.Get(subdir)
	if ig == nil {
		t.Fatal("expected non-nil for subdir")
	}
	if !ig.Matches("docs/debug.log", false) {
		t.Fatal("expected *.log to match docs/debug.log")
	}
	if ig.Matches("docs/notes.md", false) {
		t.Fatal("expected *.log to NOT match docs/notes.md")
	}
}

// --- Task 1.6: Cache — caching works ---

func TestCache_Caching(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir root: %v", err)
	}
	subdir := filepath.Join(root, "docs")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, IgnoreFilename), []byte("*.log"), 0o644); err != nil {
		t.Fatalf("write root .leafwikiignore: %v", err)
	}

	c := NewCache(root)

	// First call computes and caches
	ig1 := c.Get(subdir)
	if ig1 == nil {
		t.Fatal("expected non-nil on first call")
	}

	// Second call returns cached result (pointer equality)
	ig2 := c.Get(subdir)
	if ig2 == nil {
		t.Fatal("expected non-nil on second call")
	}
	if ig1 != ig2 {
		t.Fatal("expected pointer equality, got different objects")
	}
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
