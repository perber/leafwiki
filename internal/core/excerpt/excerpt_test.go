package excerpt

import (
	"strings"
	"testing"
)

func TestFromContent_PlainText(t *testing.T) {
	raw := "---\ntitle: Hello\n---\n\nThis is the page body."
	got := FromContent(raw)
	if got != "This is the page body." {
		t.Errorf("got %q", got)
	}
}

func TestFromContent_NoFrontmatter(t *testing.T) {
	raw := "Just plain content here."
	got := FromContent(raw)
	if got != "Just plain content here." {
		t.Errorf("got %q", got)
	}
}

func TestFromContent_EmptyBody(t *testing.T) {
	raw := "---\ntitle: Hello\n---\n\n"
	got := FromContent(raw)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFromContent_EmptyContent(t *testing.T) {
	got := FromContent("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestFromContent_StripsFencedCode(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nBefore.\n\n```go\nfunc main() {}\n```\n\nAfter."
	got := FromContent(raw)
	if strings.Contains(got, "func main") {
		t.Errorf("excerpt should not contain fenced code, got %q", got)
	}
	if !strings.Contains(got, "Before.") || !strings.Contains(got, "After.") {
		t.Errorf("excerpt should contain surrounding text, got %q", got)
	}
}

func TestPlainTextForSearch_IncludesCodeBlockContent(t *testing.T) {
	body := "Before.\n\n```go\nfunc main() {}\n```\n\nAfter."
	got := PlainTextForSearch(body)
	if !strings.Contains(got, "func main") {
		t.Errorf("search text should contain fenced code content, got %q", got)
	}
	if !strings.Contains(got, "Before.") || !strings.Contains(got, "After.") {
		t.Errorf("search text should contain surrounding text, got %q", got)
	}
}

func TestFromContent_StripsMarkdownHeadings(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n# Heading One\n\nSome body text."
	got := FromContent(raw)
	if strings.Contains(got, "#") {
		t.Errorf("excerpt should not contain # heading markers, got %q", got)
	}
	if !strings.Contains(got, "Heading One") {
		t.Errorf("heading text should be preserved, got %q", got)
	}
}

func TestFromContent_StripsImageSyntax(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n![alt text](image.png) Some text."
	got := FromContent(raw)
	if strings.Contains(got, "![") || strings.Contains(got, "image.png") {
		t.Errorf("image syntax should be stripped, got %q", got)
	}
	if !strings.Contains(got, "alt text") {
		t.Errorf("alt text should be preserved, got %q", got)
	}
}

func TestFromContent_StripsLinkSyntax(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n[Click here](https://example.com) for more."
	got := FromContent(raw)
	if strings.Contains(got, "https://example.com") || strings.Contains(got, "](") {
		t.Errorf("link URL should be stripped, got %q", got)
	}
	if !strings.Contains(got, "Click here") {
		t.Errorf("link text should be preserved, got %q", got)
	}
}

func TestFromContent_TruncatesLongContent(t *testing.T) {
	body := strings.Repeat("word ", 200)
	raw := "---\ntitle: T\n---\n\n" + body
	got := FromContent(raw)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("long content should be truncated with ellipsis, got %q", got)
	}
	runes := []rune(got)
	if len(runes) > MaxRunes+10 {
		t.Errorf("excerpt too long: %d runes", len(runes))
	}
}

func TestFromContent_ShortContentNotTruncated(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nShort body."
	got := FromContent(raw)
	if strings.HasSuffix(got, "...") {
		t.Errorf("short content should not be truncated, got %q", got)
	}
}

func TestFromContent_CollapsesWhitespace(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nLine one.\n\nLine two.\n\nLine three."
	got := FromContent(raw)
	if strings.Contains(got, "\n") {
		t.Errorf("excerpt should have no newlines, got %q", got)
	}
	if strings.Contains(got, "  ") {
		t.Errorf("excerpt should have no double spaces, got %q", got)
	}
}

func TestFromContent_StripsHTMLTags(t *testing.T) {
	raw := "---\ntitle: T\n---\n\n<strong>Bold</strong> text."
	got := FromContent(raw)
	if strings.Contains(got, "<strong>") || strings.Contains(got, "</strong>") {
		t.Errorf("HTML tags should be stripped, got %q", got)
	}
}

func TestFromContent_StripsMarkdownEmphasisMarkers(t *testing.T) {
	raw := "---\ntitle: T\n---\n\nLeafWiki **fett** und _kursiv_."
	got := FromContent(raw)
	if strings.Contains(got, "**") || strings.Contains(got, "_") {
		t.Errorf("markdown emphasis markers should be stripped, got %q", got)
	}
	if !strings.Contains(got, "LeafWiki fett und kursiv.") {
		t.Errorf("expected plain text excerpt, got %q", got)
	}
}

func TestNormalizeMarkdownBody_KeepsLabelsAndRemovesShoutoutFenceSyntax(t *testing.T) {
	body := strings.Join([]string{
		"::: info",
		"Helpful details.",
		":::",
		"",
		"::: custom-banner",
		"Custom text.",
		":::",
	}, "\n")

	got := NormalizeMarkdownBody(body)

	if strings.Contains(got, ":::") {
		t.Fatalf("expected fence syntax to be removed, got %q", got)
	}
	for _, want := range []string{"info", "custom-banner", "Helpful details.", "Custom text."} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected normalized content to contain %q, got %q", want, got)
		}
	}
}

func TestNormalizeMarkdownBody_IgnoresCodeFences(t *testing.T) {
	body := strings.Join([]string{
		"```md",
		"::: info",
		"literal",
		":::",
		"```",
	}, "\n")

	got := NormalizeMarkdownBody(body)

	if got != body {
		t.Fatalf("expected fenced code block to stay unchanged, got %q", got)
	}
}

func TestPlainTextFromMarkdown_StripsWikiLinks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"plain wikilink", "See [[Some Page]] for details.", "See Some Page for details."},
		{"aliased wikilink", "See [[Some Page|this page]] for details.", "See this page for details."},
		{"image embed", "![[image.png]] Some text.", "Some text."},
		{"path-hinted wikilink shows last segment", "See [[reference/endpoints]] for details.", "See endpoints for details."},
		{"path-hinted aliased wikilink shows alias", "See [[reference/endpoints|API]] for details.", "See API for details."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PlainTextFromMarkdown(tc.body)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPlainTextForSearch_WikiLinkAliasInCodeBlockPreservesPageName(t *testing.T) {
	body := "```\n[[SomePage|Display Text]]\n```"
	got := PlainTextForSearch(body)
	if !strings.Contains(got, "SomePage") {
		t.Errorf("page name inside code block should be searchable, got %q", got)
	}
	if !strings.Contains(got, "Display") {
		t.Errorf("alias text inside code block should be searchable, got %q", got)
	}
}

func TestPlainTextForSearch_WikiImageInCodeBlockPreservesFilename(t *testing.T) {
	body := "```\n![[screenshot.png]]\n```"
	got := PlainTextForSearch(body)
	if !strings.Contains(got, "screenshot.png") {
		t.Errorf("image filename inside code block should be searchable, got %q", got)
	}
}

func TestPlainTextForSearch_InlinePatternsProseMayRunNormally(t *testing.T) {
	body := "See [[ProseLink|prose alias]] and ![[icon.png]] outside code."
	got := PlainTextForSearch(body)
	if !strings.Contains(got, "prose alias") {
		t.Errorf("wiki link alias outside code block should be extracted, got %q", got)
	}
	if strings.Contains(got, "ProseLink") {
		t.Errorf("wiki link page name outside code block should not appear verbatim, got %q", got)
	}
}

func TestPlainTextForSearch_MixedCodeAndProsePatterns(t *testing.T) {
	body := "See [[ProseLink|prose]] for info.\n\n```\n[[CodePage|code alias]]\n```"
	got := PlainTextForSearch(body)
	if !strings.Contains(got, "prose") {
		t.Errorf("prose wiki link alias should be extracted, got %q", got)
	}
	if !strings.Contains(got, "CodePage") {
		t.Errorf("page name inside code block should be searchable, got %q", got)
	}
}

func TestPlainTextForSearch_ExcludesMermaidBlocks(t *testing.T) {
	body := "Some prose.\n\n```mermaid\ngraph TD\n    A[Login] --> B{Auth ok?}\n```\n\nMore prose."
	got := PlainTextForSearch(body)
	if strings.Contains(got, "graph") || strings.Contains(got, "-->") {
		t.Errorf("mermaid diagram syntax should not be in search index, got %q", got)
	}
	if !strings.Contains(got, "Some prose.") || !strings.Contains(got, "More prose.") {
		t.Errorf("surrounding prose should still be indexed, got %q", got)
	}
}

func TestPlainTextFromMarkdown_ExcludesMermaidBlocks(t *testing.T) {
	body := "Some prose.\n\n```mermaid\ngraph TD\n    A[Login] --> B{Auth ok?}\n```\n\nMore prose."
	got := PlainTextFromMarkdown(body)
	if strings.Contains(got, "graph") || strings.Contains(got, "-->") {
		t.Errorf("mermaid diagram syntax should not appear in display excerpt, got %q", got)
	}
}

// Bug 1: fencePattern requires "^ {0,3}" (literal spaces), so "> ```go" (block-quote prefix)
// doesn't match and fences inside block-quotes are invisible to applyInlinePatternsOutsideFences.
func TestPlainTextForSearch_WikiImageInBlockQuotedCodeBlock(t *testing.T) {
	body := "> ```\n> ![[screenshot.png]]\n> ```"
	got := PlainTextForSearch(body)
	if !strings.Contains(got, "screenshot.png") {
		t.Errorf("image filename in block-quoted code should be searchable, got %q", got)
	}
}

// Bug 2: the fence-OPENING line is processed by inline patterns because fence==nil
// at the time of the if-check (nextFence is computed first but not yet assigned).
func TestApplyInlinePatternsOutsideFences_FenceOpenLineNotProcessed(t *testing.T) {
	input := "```![[img.png]]\ncurl\n```"
	got := applyInlinePatternsOutsideFences(input)
	lines := strings.Split(got, "\n")
	if lines[0] != "```![[img.png]]" {
		t.Errorf("fence-open line should not be processed by patterns, got %q", lines[0])
	}
	if lines[1] != "curl" {
		t.Errorf("code content should be preserved verbatim, got %q", lines[1])
	}
}

func TestFromBody_StripsHTMLAndMarkdown(t *testing.T) {
	body := "<p><strong>Bold</strong> [link](https://example.com)</p>"
	got := FromBody(body)
	if got != "Bold link" {
		t.Errorf("got %q", got)
	}
}
