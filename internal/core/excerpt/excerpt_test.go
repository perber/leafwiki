package excerpt

import (
	"strings"
	"testing"
)

func TestFromContent_ReturnsBodyText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "frontmatter is skipped", raw: "---\ntitle: Hello\n---\n\nThis is the page body.", want: "This is the page body."},
		{name: "content without frontmatter", raw: "Just plain content here.", want: "Just plain content here."},
		{name: "empty body", raw: "---\ntitle: Hello\n---\n\n", want: ""},
		{name: "empty content", raw: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FromContent(tc.raw); got != tc.want {
				t.Errorf("FromContent(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestFromContent_StripsMarkup(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contains    []string
		notContains []string
	}{
		{name: "fenced code", body: "Before.\n\n```go\nfunc main() {}\n```\n\nAfter.", contains: []string{"Before.", "After."}, notContains: []string{"func main"}},
		{name: "heading markers", body: "# Heading One\n\nSome body text.", contains: []string{"Heading One"}, notContains: []string{"#"}},
		{name: "image syntax keeps alt text", body: "![alt text](image.png) Some text.", contains: []string{"alt text"}, notContains: []string{"![", "image.png"}},
		{name: "link syntax keeps link text", body: "[Click here](https://example.com) for more.", contains: []string{"Click here"}, notContains: []string{"https://example.com", "]("}},
		{name: "HTML tags", body: "<strong>Bold</strong> text.", notContains: []string{"<strong>", "</strong>"}},
		{name: "emphasis markers", body: "LeafWiki **fett** und _kursiv_.", contains: []string{"LeafWiki fett und kursiv."}, notContains: []string{"**", "_"}},
		{name: "whitespace is collapsed", body: "Line one.\n\nLine two.\n\nLine three.", notContains: []string{"\n", "  "}},
		{name: "short content is not truncated", body: "Short body.", notContains: []string{"..."}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := FromContent("---\ntitle: T\n---\n\n" + tc.body)
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("excerpt %q should contain %q", got, want)
				}
			}
			for _, unwanted := range tc.notContains {
				if strings.Contains(got, unwanted) {
					t.Errorf("excerpt %q should not contain %q", got, unwanted)
				}
			}
		})
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
