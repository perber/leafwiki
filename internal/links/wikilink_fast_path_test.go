package links

import (
	"reflect"
	"strings"
	"testing"
)

func TestExtractWikiLinksFromMarkdown_NonWikilinkDocuments_ReturnNoTargets(t *testing.T) {
	for _, content := range []string{"", "Plain text [link](/page)", "`code`\n\n```go\nvar x = 1\n```", "[[unfinished", "[[ -n $value ]]"} {
		if got := extractWikiLinksFromMarkdown(content); len(got) != 0 {
			t.Fatalf("%q: got %v", content, got)
		}
	}
}
func TestExtractWikiLinksFromMarkdown_MixedCodeAndUnicode_PreservesTargets(t *testing.T) {
	content := "[[Überblick]] [[Notes|Alias]] `[[Inline]]`\n\n```\n[[Fenced]]\n```\n\n    [[Indented]]\n\n[[Überblick]]"
	if got := extractWikiLinksFromMarkdown(content); !reflect.DeepEqual(got, []string{"Überblick", "Notes"}) {
		t.Fatalf("got %v", got)
	}
}
func BenchmarkExtractWikiLinksFastPath(b *testing.B) {
	for _, tc := range []struct{ name, content string }{
		{"NoLinks", strings.Repeat("Paragraph with **formatting**, `code` and [ordinary links](/page).\n\n", 200)},
		{"WithLinks", strings.Repeat("Paragraph [[Overview]] and `[[Excluded]]`.\n\n", 200)},
		{"CodeOnly", strings.Repeat("```go\n// [[Excluded]]\n```\n\n", 200)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				extractWikiLinksFromMarkdown(tc.content)
			}
		})
	}
}
