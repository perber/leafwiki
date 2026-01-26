package markdown

import (
	"testing"

	"github.com/perber/wiki/internal/test_utils"
)

func TestPlanner_extractTitleFromMDFile_FrontmatterTitleWins(t *testing.T) {
	tmp := t.TempDir()
	abs := test_utils.WriteFile(t, tmp, "t.md", "---\ntitle: FM Title\n---\n\n# Heading")

	mdFile, err := LoadMarkdownFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	title, err := mdFile.GetTitle()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "FM Title" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_extractTitleFromMDFile_LeafwikiTitle(t *testing.T) {
	tmp := t.TempDir()
	abs := test_utils.WriteFile(t, tmp, "t.md", "---\nleafwiki_title: Leaf\n---\n\n# Heading")

	mdFile, err := LoadMarkdownFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	title, err := mdFile.GetTitle()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "Leaf" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_extractTitleFromMDFile_FirstHeadingFallback(t *testing.T) {
	tmp := t.TempDir()
	abs := test_utils.WriteFile(t, tmp, "t.md", "no fm\n\n# Heading Only\nx")

	mdFile, err := LoadMarkdownFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	title, err := mdFile.GetTitle()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "Heading Only" {
		t.Fatalf("title = %q", title)
	}
}

func TestPlanner_extractTitleFromMDFile_FilenameFallback(t *testing.T) {
	tmp := t.TempDir()
	abs := test_utils.WriteFile(t, tmp, "some-file.md", "no title")

	mdFile, err := LoadMarkdownFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	title, err := mdFile.GetTitle()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "some-file" {
		t.Fatalf("title = %q", title)
	}
}

func TestLoadMarkdownFile_UppercaseExtension(t *testing.T) {
	tmp := t.TempDir()
	abs := test_utils.WriteFile(t, tmp, "README.MD", "# Uppercase Extension\n\nThis file has .MD extension")

	mdFile, err := LoadMarkdownFile(abs)
	if err != nil {
		t.Fatalf("expected no error for .MD extension, got: %v", err)
	}
	title, err := mdFile.GetTitle()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if title != "Uppercase Extension" {
		t.Fatalf("title = %q, want %q", title, "Uppercase Extension")
	}
}
