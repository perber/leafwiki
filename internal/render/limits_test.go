package render

import (
	"errors"
	"strings"
	"testing"
)

// tableBomb is a GFM table whose rows goldmark pads to the header's width, so
// n header cells and n one-cell rows render n² cells.
func tableBomb(n int) string {
	return "|" + strings.Repeat("a|", n) + "\n|" + strings.Repeat("-|", n) + "\n" + strings.Repeat("|x\n", n)
}

// quotedTableBomb is tableBomb inside a block quote, so every line starts with "> ".
func quotedTableBomb(n int) string {
	return "> |" + strings.Repeat("a|", n) + "\n> |" + strings.Repeat("-|", n) + "\n" + strings.Repeat("> |x\n", n)
}

func TestRenderHTML_PathologicalInput_IsRefused(t *testing.T) {
	cases := []struct{ name, content string }{
		{name: "too large", content: strings.Repeat("word ", maxContentBytes/5+1)},
		{name: "table padded to n² cells", content: tableBomb(2000)},
		{name: "emphasis delimiters", content: strings.Repeat("*a", maxEmphasisDelimiters+1)},
		{name: "nested list markers on one line", content: strings.Repeat("- ", 50000) + "x"},
		{name: "nested block quotes", content: strings.Repeat(">", 50000)},
		{name: "table inside a block quote", content: quotedTableBomb(2000)},
		{name: "table inside a block quote without space", content: strings.ReplaceAll(quotedTableBomb(2000), "> ", ">")},
		{name: "unclosed links in one paragraph", content: strings.Repeat("[a](", 16<<10)},
		{name: "unclosed angle links in one paragraph", content: strings.Repeat("[a](<", 13<<10)},
		{name: "strikethrough runs in one paragraph", content: strings.Repeat("~~a", 20<<10)},
		{name: "nested link brackets", content: strings.Repeat("[", 8<<10) + strings.Repeat("](/a)", 8<<10)},
		{name: "many deeply nested lines", content: strings.Repeat(strings.Repeat("- ", 60)+"x\n", 2100)},
		{name: "table after an intro line", content: "intro\n" + tableBomb(2000)},
		{name: "table after five lines", content: strings.Repeat("line\n", 5) + tableBomb(1500)},
		{name: "quoted table after an intro line", content: "> intro\n" + quotedTableBomb(2000)},
		{name: "table in a list item after an intro", content: "- intro\n" + strings.ReplaceAll("  "+tableBomb(2000), "\n|", "\n  |")},
		{name: "backtick runs in one paragraph", content: strings.Repeat("`a", 16<<10)},
		{name: "long paragraph", content: strings.Repeat("word ", (maxInlineBytes/5)+2)},
		{name: "text foster-parented out of a raw HTML table", content: "<div>\n<table>" + strings.Repeat("x</td>", 20<<10)},
		{name: "link reference definitions without blank lines", content: strings.Repeat("[r]: /x\n", maxConsecutiveDefinitions+1) + "[r]"},
		{name: "end tags on the closing line of an HTML block", content: "<!X\n" + strings.Repeat("x</td>", 43000)},
		{name: "end tags in a pre block", content: "<pre>\n" + strings.Repeat("x</li>", 43000) + "</pre>"},
		{name: "end tags spread over table rows", content: "|a|\n|-|\n" + strings.Repeat("|"+strings.Repeat("x</td>", 66)+"|\n", 600)},
		{name: "list nested by indentation", content: func() string {
			var b strings.Builder
			for i := 0; i < 600; i++ {
				b.WriteString(strings.Repeat("  ", i) + "- x\n")
			}
			return b.String()
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The up-front check must refuse it: the output cap alone would stop
			// it only after goldmark built the expensive AST.
			if err := checkComplexity(tc.content); !errors.Is(err, ErrTooComplex) {
				t.Fatalf("checkComplexity = %v, want ErrTooComplex", err)
			}
			out, err := RenderHTML(tc.content, testOptions(nil))
			if !errors.Is(err, ErrTooComplex) || out != "" {
				t.Fatalf("RenderHTML = %d bytes, %v; want ErrTooComplex", len(out), err)
			}
		})
	}
}

func TestRenderHTML_LargeOrdinaryPages_StillRender(t *testing.T) {
	var table strings.Builder
	table.WriteString("| a | b | c | d |\n|---|---|---|---|\n")
	for i := 0; i < 2000; i++ {
		table.WriteString("| 1 | 2 | 3 | 4 |\n")
	}
	cases := []struct{ name, content string }{
		{name: "dense markdown", content: strings.Repeat("# H\n\nSome *text* with [a link](x) and `code`.\n\n- item\n  - nested\n\n> quote\n\n", 1500)},
		{name: "long table", content: table.String()},
		{name: "long list of links", content: strings.Repeat("- [a link](/page) and [another](https://example.com/x)\n", 3000)},
		{name: "table with links in every row", content: "| a | b |\n|---|---|\n" + strings.Repeat("| [x](/x) **b** | [[Foo]] ~~c~~ |\n", 1500)},
		{name: "raw HTML table", content: "<table>\n" + strings.Repeat("<tr><td>a</td><td>b</td></tr>\n", 500) + "</table>"},
		{name: "many paragraphs with code spans", content: strings.Repeat("Use `go test` and `make ci` here.\n\n", 3000)},
		{name: "link reference definitions", content: strings.Repeat("[r]: /x\n", 200) + "\n" + strings.Repeat("[r]: /y\n\n", 2000) + "[r]"},
		{name: "large raw HTML table", content: "<table>\n" + strings.Repeat("<tr><td>a</td><td>b</td><td>c</td></tr>\n", 1200) + "</table>"},
		{name: "long pasted paragraph", content: strings.Repeat("A sentence of pasted prose, with a [link](/x) now and then. ", 330)},
		{name: "generated link list", content: strings.Repeat("[link]: https://example.com/page\n", 600) + "[link]"},
		{name: "paragraph with many links", content: strings.Repeat("see [a](/a) and <https://example.com> ", 150)},
		{name: "deep but sane nesting", content: strings.Repeat("> ", 20) + strings.Repeat("- ", 20) + "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if out := renderForTest(t, tc.content, testOptions(nil)); out == "" {
				t.Fatal("empty output")
			}
		})
	}
}

func TestCappedWriter_StopsAtTheLimit(t *testing.T) {
	w := &cappedWriter{limit: 10}
	if _, err := w.Write([]byte("12345")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := w.Write([]byte("123456")); !errors.Is(err, ErrTooComplex) {
		t.Fatalf("write past the limit: %v, want ErrTooComplex", err)
	}
	if w.buf.Len() > 10 || !w.exceeded {
		t.Fatalf("buffered %d bytes (limit 10), exceeded=%v", w.buf.Len(), w.exceeded)
	}
}

// balanceHTML is the last step before the shell; it must refuse anything the
// policy would never let through, so a future policy mistake is an error and
// not XSS.
func TestBalanceHTML_RefusesWhatThePolicyDisallows(t *testing.T) {
	cases := []struct{ name, in string }{
		{name: "unknown element", in: "<p><iframe></iframe></p>"},
		{name: "script", in: "<script>x</script>"},
		{name: "foreign content", in: "<svg><g></g></svg>"},
		{name: "math", in: "<math><mi>x</mi></math>"},
		{name: "comment", in: "<p>a<!-- c --></p>"},
		{name: "event handler", in: `<a href="x" onclick="y">a</a>`},
		{name: "style attribute", in: `<p style="color:red">a</p>`},
		{name: "attribute on the wrong element", in: `<p href="x">a</p>`},
		{name: "id", in: `<div id="root"></div>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if out, err := balanceHTML([]byte(tc.in)); err == nil {
				t.Fatalf("balanceHTML(%q) = %q, want an error", tc.in, out)
			}
		})
	}
	if _, err := balanceHTML([]byte(`<p><a href="/x" title="t">a</a><img src="/i.png" alt="i" width="1"><input type="checkbox" checked disabled></p><table><tr><td align="left" colspan="2">x</td></tr></table>`)); err != nil {
		t.Fatalf("allowed markup refused: %v", err)
	}
}
