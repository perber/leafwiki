package render

import (
	"strings"
	"testing"
)

// testOptions is the default fixture: a page at /docs/page under base path
// /wiki whose wikilinks resolve through pages. Tests override single fields.
func testOptions(pages map[string]string) Options {
	return Options{
		BasePath:  "/wiki",
		RoutePath: "/docs/page",
		ResolveWikiLink: func(target string) (string, bool) {
			route, ok := pages[target]
			return route, ok
		},
	}
}

func renderForTest(t *testing.T, content string, opt Options) string {
	t.Helper()
	out, err := RenderHTML(content, opt)
	if err != nil {
		t.Fatalf("RenderHTML(%q) error: %v", content, err)
	}
	return out
}

// renderCase is the shape shared by the table tests in this package: the
// output must contain every want and none of the wantNot fragments.
type renderCase struct {
	name    string
	content string
	want    []string
	wantNot []string
}

func runRenderCases(t *testing.T, cases []renderCase, opt Options) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := renderForTest(t, tc.content, opt)
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output %q does not contain %q", out, w)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(out, w) {
					t.Errorf("output %q contains %q", out, w)
				}
			}
			assertSafeHTML(t, out)
		})
	}
}

func TestRenderHTML_Markdown_RendersBlocks(t *testing.T) {
	cases := []renderCase{
		{name: "heading without id", content: "# Title\n\n## Sub", want: []string{"<h1>Title</h1>", "<h2>Sub</h2>"}},
		{name: "paragraph with emphasis", content: "Some *em* and **strong**", want: []string{"<p>Some <em>em</em> and <strong>strong</strong></p>"}},
		{name: "table with alignment", content: "| a | b |\n|:-:|--:|\n| 1 | 2 |", want: []string{"<table>", `<th align="center">a</th>`, `<td align="right">2</td>`}},
		{name: "task list", content: "- [x] done\n- [ ] open", want: []string{`<input checked="" disabled="" type="checkbox"`, `<input disabled="" type="checkbox"`, "done", "open"}},
		{name: "strikethrough", content: "~~gone~~", want: []string{"<del>gone</del>"}},
		{name: "ordered list keeps start", content: "3. three\n4. four", want: []string{`<ol start="3">`}},
		{name: "blockquote", content: "> quoted", want: []string{"<blockquote>", "quoted"}},
		{name: "linkified URL", content: "see https://example.com/x", want: []string{`<a href="https://example.com/x">https://example.com/x</a>`}},
		{name: "mermaid stays code", content: "```mermaid\ngraph TD; A-->B\n```", want: []string{"<pre><code>graph TD; A--&gt;B\n</code></pre>"}, wantNot: []string{"language-mermaid"}},
		{name: "math stays text", content: "$$\nx^2\n$$", want: []string{"$$", "x^2"}},
		{name: "raw html kept when safe", content: "<details open><summary>More</summary>\n\nhidden\n\n</details>", want: []string{"<details open", "<summary>More</summary>", "hidden"}},
		{name: "raw inline html kept when safe", content: "a <kbd>Ctrl</kbd> b <mark>hi</mark>", want: []string{"<kbd>Ctrl</kbd>", "<mark>hi</mark>"}},
		{name: "embedded iframe dropped", content: `<iframe src="https://www.youtube.com/embed/x"></iframe>`, wantNot: []string{"iframe", "youtube"}},
		{name: "audio and video dropped", content: `<video src="/assets/a.mp4" controls></video><audio src="a.mp3"></audio>`, wantNot: []string{"<video", "<audio"}},
		{name: "text is escaped", content: "1 < 2 & 3 > 2", want: []string{"1 &lt; 2 &amp; 3 &gt; 2"}},
	}
	runRenderCases(t, cases, testOptions(nil))
}

func TestRenderHTML_EmptyContent_ReturnsEmpty(t *testing.T) {
	if out := renderForTest(t, "", testOptions(nil)); out != "" {
		t.Fatalf("RenderHTML(\"\") = %q, want empty", out)
	}
}

func TestRenderHTML_ZeroOptions_Renders(t *testing.T) {
	out := renderForTest(t, "[[Foo]] [x](child)", Options{})
	for _, want := range []string{"Foo", `<a href="/child">x</a>`} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q does not contain %q", out, want)
		}
	}
}

// BenchmarkRenderHTML renders a ~100 KB page with links, wikilinks, a table,
// code and raw HTML: the per-crawler-request cost on small hardware.
func BenchmarkRenderHTML(b *testing.B) {
	section := "## Section\n\nSome *text* with a [link](child) and [[Foo]], " +
		"`code [[Bar]]`, an ![img](assets/x.png) and <kbd>raw</kbd>.\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\nfunc main() {}\n```\n\n- [x] task\n\n"
	content := strings.Repeat(section, 100*1024/len(section)+1)
	opt := testOptions(map[string]string{"Foo": "/foo"})
	b.SetBytes(int64(len(content)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderHTML(content, opt); err != nil {
			b.Fatal(err)
		}
	}
}
