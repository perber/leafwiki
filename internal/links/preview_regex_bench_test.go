package links

import (
	"reflect"
	"strings"
	"testing"
)

func BenchmarkFindWikiLinksPreview(b *testing.B) {
	e := NewMarkdownRefactorEngine()
	finder := CompileWikiLinkFinder("/docs/project-plan", "Project Plan")
	content := strings.Repeat("Text [[Project Plan|Alias]] and [[docs/project-plan]]. `[[Project Plan]]`\n\n", 20)
	b.ReportAllocs()
	for b.Loop() {
		e.FindWikiLinksPrecompiled(content, finder)
	}
}

func TestCompiledWikiLinkFinder_ReusedAcrossPages_PreservesMatching(t *testing.T) {
	e := NewMarkdownRefactorEngine()
	finder := CompileWikiLinkFinder("/docs/project-plan", "Project Plan")
	cases := []struct {
		content string
		want    []string
	}{
		{"[[docs/project-plan]] [[PROJECT PLAN|Alias]]", []string{"[[docs/project-plan]]", "[[Project Plan]]"}},
		{"`[[Project Plan]]`\n\n```\n[[docs/project-plan]]\n```", nil},
		{"[[ Project Plan ]]", []string{"[[Project Plan]]"}},
		{"ordinary text", nil}, {"", nil},
	}
	for _, tc := range cases {
		got := e.FindWikiLinksPrecompiled(tc.content, finder)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%q: got %v want %v", tc.content, got, tc.want)
		}
	}
}
func TestCompiledWikiLinkFinder_RegexMetacharacters_AreLiteral(t *testing.T) {
	e := NewMarkdownRefactorEngine()
	got := e.FindWikiLinksPrecompiled("[[C++ (Guide)]] [[C (Guide)]]", CompileWikiLinkFinder("", "C++ (Guide)"))
	if !reflect.DeepEqual(got, []string{"[[C++ (Guide)]]"}) {
		t.Fatalf("got %v", got)
	}
}

func BenchmarkFindWikiLinksPreviewBatch(b *testing.B) {
	content := strings.Repeat("Text [[Project Plan|Alias]] and [[docs/project-plan]]. `[[Project Plan]]`\n\n", 20)
	for _, compiled := range []bool{false, true} {
		name := "CompilePerPage"
		if compiled {
			name = "CompilePerPreview"
		}
		b.Run(name, func(b *testing.B) {
			e := NewMarkdownRefactorEngine()
			b.ReportAllocs()
			for b.Loop() {
				if compiled {
					finder := CompileWikiLinkFinder("/docs/project-plan", "Project Plan")
					for i := 0; i < 100; i++ {
						e.FindWikiLinksPrecompiled(content, finder)
					}
				} else {
					for i := 0; i < 100; i++ {
						e.FindWikiLinksForPath(content, "/docs/project-plan", "Project Plan")
					}
				}
			}
		})
	}
}
