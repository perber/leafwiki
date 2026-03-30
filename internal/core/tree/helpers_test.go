package tree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePageIsFolder_ConvertsFlatFileToFolder(t *testing.T) {
	tmp := t.TempDir()
	pagePath := "docs/guide"
	flatFile := filepath.Join(tmp, "docs", "guide.md")

	if err := os.MkdirAll(filepath.Dir(flatFile), 0o755); err != nil {
		t.Fatalf("MkdirAll err: %v", err)
	}
	if err := os.WriteFile(flatFile, []byte("# Guide"), 0o644); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}

	if err := EnsurePageIsFolder(tmp, pagePath); err != nil {
		t.Fatalf("EnsurePageIsFolder err: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, "docs", "guide", "index.md")); err != nil {
		t.Fatalf("expected index.md after conversion, got err: %v", err)
	}
	if _, err := os.Stat(flatFile); !os.IsNotExist(err) {
		t.Fatalf("expected flat file to be removed, got err: %v", err)
	}
}

func TestFoldPageFolderIfEmpty_FoldsIndexBackToFlatFile(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "docs", "guide")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll err: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatalf("WriteFile err: %v", err)
	}

	if err := FoldPageFolderIfEmpty(tmp, "docs/guide"); err != nil {
		t.Fatalf("FoldPageFolderIfEmpty err: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tmp, "docs", "guide.md")); err != nil {
		t.Fatalf("expected folded flat file, got err: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected folder to be removed, got err: %v", err)
	}
}

func TestPageDiskPaths_WindowsPath(t *testing.T) {
	storageDir := `C:\wiki\data\root`
	pagePath := "docs/guide"

	if got, want := strings.ReplaceAll(pageDirectoryDiskPath(storageDir, pagePath), `\`, `/`), `C:/wiki/data/root/docs/guide`; got != want {
		t.Fatalf("dir = %q, want %q", got, want)
	}
	if got, want := strings.ReplaceAll(pageMarkdownDiskPath(storageDir, pagePath), `\`, `/`), `C:/wiki/data/root/docs/guide.md`; got != want {
		t.Fatalf("md = %q, want %q", got, want)
	}
	if got, want := strings.ReplaceAll(pageIndexDiskPath(storageDir, pagePath), `\`, `/`), `C:/wiki/data/root/docs/guide/index.md`; got != want {
		t.Fatalf("index = %q, want %q", got, want)
	}
}
