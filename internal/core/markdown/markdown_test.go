package markdown

import (
	"os"
	"strings"
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

func TestMarkdownFile_WriteToFile_PreservesCustomFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	abs := test_utils.WriteFile(t, tmp, "t.md", `---
custom_key: keep-me
aliases:
  - one
leafwiki_id: old-id
leafwiki_title: Old Title
---

# Heading`)

	mdFile, err := LoadMarkdownFile(abs)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	mdFile.setFrontmatterID("new-id")
	if err := mdFile.WriteToFile(); err != nil {
		t.Fatalf("WriteToFile err: %v", err)
	}

	rawBytes, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("ReadFile err: %v", err)
	}
	raw := string(rawBytes)
	if !strings.Contains(raw, "custom_key: keep-me") {
		t.Fatalf("expected custom frontmatter to be preserved, got: %q", raw)
	}
	if !strings.Contains(raw, "- one") {
		t.Fatalf("expected custom list frontmatter to be preserved, got: %q", raw)
	}

	fm, body, has, err := ParseFrontmatter(raw)
	if err != nil {
		t.Fatalf("ParseFrontmatter err: %v", err)
	}
	if !has {
		t.Fatalf("expected frontmatter after write")
	}
	if fm.LeafWikiID != "new-id" {
		t.Fatalf("expected id 'new-id', got %q", fm.LeafWikiID)
	}
	if fm.LeafWikiTitle != "Old Title" {
		t.Fatalf("expected title 'Old Title', got %q", fm.LeafWikiTitle)
	}
	if body != "\n# Heading" {
		t.Fatalf("unexpected body: %q", body)
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

func TestNewMarkdownFileFromRaw_PreservesCustomFrontmatter(t *testing.T) {
	mdFile, err := NewMarkdownFileFromRaw("/tmp/test.md", `---
custom_key: keep-me
leafwiki_id: p1
leafwiki_title: Existing Title
---
# Body
Hello
`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if mdFile.GetFrontmatter().LeafWikiID != "p1" {
		t.Fatalf("expected id p1, got %q", mdFile.GetFrontmatter().LeafWikiID)
	}
	if mdFile.GetFrontmatter().LeafWikiTitle != "Existing Title" {
		t.Fatalf("expected title 'Existing Title', got %q", mdFile.GetFrontmatter().LeafWikiTitle)
	}
	if got := mdFile.GetContent(); got != "# Body\nHello\n" {
		t.Fatalf("unexpected content: %q", got)
	}
	if got := mdFile.GetFrontmatter().ExtraFields["custom_key"]; got != "keep-me" {
		t.Fatalf("expected custom_key to be preserved, got %#v", got)
	}
}

func TestNewMarkdownFileFromRaw_InvalidFrontmatter(t *testing.T) {
	_, err := NewMarkdownFileFromRaw("/tmp/test.md", `---
leafwiki_id: [broken
---
# Body
`)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
