package render

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// assertStaysInRoot embeds out like the SPA shell does (<div id="root">…</div>
// followed by the shell's script) and fails unless the browser's parse keeps
// every rendered node inside #root and the shell element after it right
// behind #root in <body>. Content that escapes #root survives React's
// createRoot and shows next to the app; an unclosed element swallows the shell.
func assertStaysInRoot(t *testing.T, out string) {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(`<!doctype html><html><head></head><body><div id="root">` +
		out + `</div><div id="after"></div></body></html>`))
	if err != nil {
		t.Fatalf("parse shell: %v", err)
	}
	body := findElement(doc, "body")
	if body == nil {
		t.Fatalf("no body in parsed shell for %q", out)
	}
	var kids []*html.Node
	for c := body.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" {
			continue
		}
		kids = append(kids, c)
	}
	if len(kids) != 2 || attr(kids[0], "id") != "root" || attr(kids[1], "id") != "after" {
		var b strings.Builder
		_ = html.Render(&b, body)
		t.Errorf("rendered HTML %q escapes #root or swallows the shell: body parses as %q", out, b.String())
	}
}

func findElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findElement(c, tag); f != nil {
			return f
		}
	}
	return nil
}

func attr(n *html.Node, key string) string {
	if n.Type != html.ElementNode {
		return ""
	}
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func TestRenderHTML_UnbalancedHTML_StaysInsideRoot(t *testing.T) {
	cases := []struct{ name, content string }{
		{name: "balanced markdown", content: "# T\n\n- a\n- b\n\n| a |\n|---|\n| 1 |\n\n<details><summary>s</summary>x</details>"},
		{name: "stray closing div", content: "a</div></div><p>after</p>"},
		{name: "closing body", content: "a</div></body></html><p>after</p>"},
		{name: "unclosed div", content: "<div>unclosed"},
		{name: "unclosed table", content: "<table><tr><td>x"},
		{name: "unclosed link", content: `<a href="https://example.com">unclosed`},
		{name: "unclosed details", content: "<details><summary>s"},
		{name: "plaintext", content: "<plaintext><b>x"},
		{name: "math mxss leftover", content: `<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStaysInRoot(t, renderForTest(t, tc.content, testOptions(nil)))
		})
	}
}

// FuzzRenderHTML checks the two guarantees the crawler HTML relies on for any
// input: nothing executable or clobbering survives (assertSafeHTML) and the
// output stays inside #root. Run: go test ./internal/render -fuzz FuzzRenderHTML
func FuzzRenderHTML(f *testing.F) {
	for _, seed := range []string{
		"# T\n\n[x](child) [[Foo]] ![i](assets/a.png) <kbd>k</kbd>",
		`<a href="java&#x09;script:alert(1)">x</a>`,
		`<img src=x onerror=alert(1)>`,
		"<javascript:alert(1)>",
		"[x][r]\n\n[r]: //evil.example",
		`<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>`,
		`<noscript><p title="</noscript><img src=x onerror=alert(1)>">`,
		"</div><p>x</p>",
		"<details open><summary>s</summary>\n\n*x*\n\n</details>",
	} {
		f.Add(seed)
	}
	opt := testOptions(map[string]string{"Foo": "/foo", "Proto": "//evil.example"})
	f.Fuzz(func(t *testing.T, content string) {
		out, err := RenderHTML(content, opt)
		if errors.Is(err, ErrTooComplex) {
			return // refused on purpose (limits.go); nothing is rendered
		}
		if err != nil {
			t.Fatalf("RenderHTML error: %v", err)
		}
		if strings.Contains(strings.ToLower(out), "<script") {
			t.Fatalf("output contains <script: %q", out)
		}
		assertSafeHTML(t, out)
		assertStaysInRoot(t, out)
	})
}
