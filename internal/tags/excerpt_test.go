package tags

import (
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/excerpt"
)

func TestExtractExcerptFromContent_PlainText(t *testing.T) {
	raw := "---\ntitle: Hello\n---\n\nThis is the page body."
	got := ExtractExcerptFromContent(raw)
	if got != "This is the page body." {
		t.Errorf("got %q", got)
	}
}

func TestExtractExcerptFromContent_NoFrontmatter(t *testing.T) {
	raw := "Just plain content here."
	got := ExtractExcerptFromContent(raw)
	if got != "Just plain content here." {
		t.Errorf("got %q", got)
	}
}

func TestExtractExcerptFromContent_EmptyBody(t *testing.T) {
	raw := "---\ntitle: Hello\n---\n\n"
	got := ExtractExcerptFromContent(raw)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractExcerptFromContent_EmptyContent(t *testing.T) {
	got := ExtractExcerptFromContent("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestExtractExcerptFromContent_StripsFencedCode(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nBefore.\n\n```go\nfunc main() {}\n```\n\nAfter."
	got := ExtractExcerptFromContent(raw)
	if strings.Contains(got, "func main") {
		t.Errorf("excerpt should not contain fenced code, got %q", got)
	}
	if !strings.Contains(got, "Before.") || !strings.Contains(got, "After.") {
		t.Errorf("excerpt should contain surrounding text, got %q", got)
	}
}

func TestExtractExcerptFromContent_StripsMarkdownHeadings(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n# Heading One\n\nSome body text."
	got := ExtractExcerptFromContent(raw)
	if strings.Contains(got, "#") {
		t.Errorf("excerpt should not contain # heading markers, got %q", got)
	}
	if !strings.Contains(got, "Heading One") {
		t.Errorf("heading text should be preserved, got %q", got)
	}
}

func TestExtractExcerptFromContent_StripsImageSyntax(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n![alt text](image.png) Some text."
	got := ExtractExcerptFromContent(raw)
	if strings.Contains(got, "![") || strings.Contains(got, "image.png") {
		t.Errorf("image syntax should be stripped, got %q", got)
	}
	if !strings.Contains(got, "alt text") {
		t.Errorf("alt text should be preserved, got %q", got)
	}
}

func TestExtractExcerptFromContent_StripsLinkSyntax(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n[Click here](https://example.com) for more."
	got := ExtractExcerptFromContent(raw)
	if strings.Contains(got, "https://example.com") || strings.Contains(got, "](") {
		t.Errorf("link URL should be stripped, got %q", got)
	}
	if !strings.Contains(got, "Click here") {
		t.Errorf("link text should be preserved, got %q", got)
	}
}

func TestExtractExcerptFromContent_TruncatesLongContent(t *testing.T) {
	body := strings.Repeat("word ", 200)
	raw := "---\ntitle: T\n---\n\n" + body
	got := ExtractExcerptFromContent(raw)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("long content should be truncated with ellipsis, got %q", got)
	}
	runes := []rune(got)
	if len(runes) > excerpt.MaxRunes+10 {
		t.Errorf("excerpt too long: %d runes", len(runes))
	}
}

func TestExtractExcerptFromContent_ShortContentNotTruncated(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nShort body."
	got := ExtractExcerptFromContent(raw)
	if strings.HasSuffix(got, "...") {
		t.Errorf("short content should not be truncated, got %q", got)
	}
}

func TestExtractExcerptFromContent_CollapsesWhitespace(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nLine one.\n\nLine two.\n\nLine three."
	got := ExtractExcerptFromContent(raw)
	if strings.Contains(got, "\n") {
		t.Errorf("excerpt should have no newlines, got %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Errorf("excerpt should have no double spaces, got %q", got)
	}
}

func TestExtractExcerptFromContent_StripsHTMLTags(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n<strong>Bold</strong> text."
	got := ExtractExcerptFromContent(raw)
	if strings.Contains(got, "<strong>") || strings.Contains(got, "</strong>") {
		t.Errorf("HTML tags should be stripped, got %q", got)
	}
}
