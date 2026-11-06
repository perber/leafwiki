package render

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// safeElements is every element the crawler HTML may contain. assertSafeHTML
// checks the output against it independently of the policy under test.
var safeElements = map[string]bool{
	"a": true, "b": true, "blockquote": true, "br": true, "code": true, "dd": true,
	"del": true, "details": true, "div": true, "dl": true, "dt": true, "em": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true,
	"i": true, "img": true, "input": true, "ins": true, "kbd": true, "li": true,
	"mark": true, "ol": true, "p": true, "pre": true, "q": true, "s": true, "samp": true,
	"span": true, "strike": true, "strong": true, "sub": true, "summary": true,
	"sup": true, "table": true, "tbody": true, "td": true, "tfoot": true, "th": true,
	"thead": true, "tr": true, "tt": true, "ul": true, "var": true,
}

// assertSafeHTML fails if out contains an element outside safeElements, an
// event handler, an attribute that can clobber DOM globals or carry CSS, or a
// URL that is not http(s), mailto or a same-origin path.
func assertSafeHTML(t *testing.T, out string) {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(out))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken && tt != html.EndTagToken {
			continue
		}
		tok := z.Token()
		if !safeElements[tok.Data] {
			t.Errorf("unsafe element <%s> in %q", tok.Data, out)
		}
		for _, a := range tok.Attr {
			key := strings.ToLower(a.Key)
			switch {
			case strings.HasPrefix(key, "on"), key == "id", key == "name", key == "class",
				key == "style", key == "target", key == "form", key == "formaction":
				t.Errorf("unsafe attribute %s=%q in %q", a.Key, a.Val, out)
			case key == "href" || key == "src":
				if !safeURL(a.Val) {
					t.Errorf("unsafe URL %s=%q in %q", a.Key, a.Val, out)
				}
			}
		}
	}
}

func safeURL(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	if strings.HasPrefix(v, "//") || strings.HasPrefix(v, `\`) || strings.HasPrefix(v, `/\`) {
		return false
	}
	if i := strings.IndexAny(v, ":/?#"); i >= 0 && v[i] == ':' {
		scheme := v[:i]
		return scheme == "http" || scheme == "https" || scheme == "mailto"
	}
	return true
}

func TestRenderHTML_XSSCorpus_IsNeutralized(t *testing.T) {
	cases := []renderCase{
		{name: "script tag", content: "<script>alert(1)</script>", wantNot: []string{"<script", "alert(1)"}},
		{name: "inline script in paragraph", content: "a <script>alert(1)</script> b", wantNot: []string{"<script", "alert(1)"}},
		{name: "img onerror", content: `<img src=x onerror=alert(1)>`, wantNot: []string{"onerror", "alert"}},
		{name: "javascript markdown link", content: "[x](javascript:alert(1))", wantNot: []string{"javascript", "href"}},
		{name: "mixed case javascript", content: `<a href="JaVaScRiPt:alert(1)">x</a>`, wantNot: []string{"avascript", "href"}},
		{name: "javascript with leading space", content: `<a href=" javascript:alert(1)">x</a>`, wantNot: []string{"javascript", "href"}},
		{name: "javascript with tab entity", content: `<a href="java&#x09;script:alert(1)">x</a>`, wantNot: []string{"script:", "href"}},
		{name: "entity-encoded javascript", content: `<a href="&#106;avascript:alert(1)">x</a>`, wantNot: []string{"avascript", "href"}},
		{name: "entity-encoded javascript markdown", content: "[x](&#106;avascript:alert(1))", wantNot: []string{"avascript", "href"}},
		{name: "entity-encoded colon", content: `<a href="javascript&colon;alert(1)">x</a>`, wantNot: []string{"javascript", "href"}},
		{name: "vbscript link", content: `<a href="vbscript:msgbox(1)">x</a>`, wantNot: []string{"vbscript", "href"}},
		{name: "data html link", content: "[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)", wantNot: []string{"data:", "href"}},
		{name: "data svg image", content: `<img src="data:image/svg+xml,<svg onload=alert(1)>">`, wantNot: []string{"data:", "<img"}},
		{name: "svg onload", content: `<svg onload=alert(1)><circle r="1"/></svg>`, wantNot: []string{"<svg", "onload", "<circle"}},
		{name: "script inside svg", content: `<svg><script>alert(1)</script></svg>`, wantNot: []string{"<svg", "<script", "alert(1)"}},
		{name: "math mxss", content: `<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>`, wantNot: []string{"<math", "<style", "onerror", "<mglyph"}},
		{name: "iframe", content: `<iframe src="https://evil.example"></iframe>`, wantNot: []string{"<iframe", "evil"}},
		{name: "srcdoc iframe", content: `<iframe srcdoc="<script>alert(1)</script>"></iframe>`, wantNot: []string{"<iframe", "srcdoc", "alert"}},
		{name: "object and embed", content: `<object data="x.swf"></object><embed src="x.swf">`, wantNot: []string{"<object", "<embed", "x.swf"}},
		{name: "form with inputs", content: `<form action="https://evil.example"><input name="q" type="text"><button formaction="https://evil.example">go</button></form>`, wantNot: []string{"<form", "action", "name=", "<button", "evil"}},
		{name: "text input dropped", content: `<input type="text" autofocus onfocus="alert(1)">`, wantNot: []string{"<input", "onfocus", "autofocus"}},
		{name: "style element", content: "<style>body{display:none}</style>", wantNot: []string{"<style", "display:none"}},
		{name: "link stylesheet", content: `<link rel="stylesheet" href="https://evil.example/x.css">`, wantNot: []string{"<link", "evil"}},
		{name: "base tag", content: `<base href="https://evil.example/">`, wantNot: []string{"<base", "evil"}},
		{name: "meta refresh", content: `<meta http-equiv="refresh" content="0;url=https://evil.example">`, wantNot: []string{"<meta", "refresh", "evil"}},
		{name: "id clobbers root", content: `<div id="root">x</div>`, want: []string{"<div>x</div>"}, wantNot: []string{"id="}},
		{name: "id clobbers mermaid renderer", content: `<span id="mermaid-renderer">x</span>`, wantNot: []string{"id=", "mermaid-renderer"}},
		{name: "name clobbering", content: `<img name="getElementById" src="x.png"><a name="cookie" href="/x">y</a>`, wantNot: []string{"name="}},
		{name: "class dropped", content: `<p class="hidden">x</p>`, want: []string{"<p>x</p>"}, wantNot: []string{"class="}},
		{name: "style with url", content: `<p style="background:url(javascript:alert(1))">x</p>`, want: []string{"<p>x</p>"}, wantNot: []string{"style=", "url("}},
		{name: "style with expression", content: `<span style="width:expression(alert(1))">x</span>`, wantNot: []string{"style=", "expression"}},
		{name: "target blank dropped", content: `<a href="https://example.com" target="_blank">x</a>`, want: []string{`<a href="https://example.com">x</a>`}, wantNot: []string{"target"}},
		{name: "details ontoggle", content: `<details open ontoggle="alert(1)"><summary>s</summary></details>`, wantNot: []string{"ontoggle", "alert"}},
		{name: "protocol-relative raw link", content: `<a href="//evil.example">x</a>`, wantNot: []string{"evil"}},
		{name: "backslash protocol-relative raw link", content: `<a href="\\evil.example">x</a>`, wantNot: []string{"evil"}},
		{name: "slash backslash raw link", content: `<a href="/\evil.example">x</a>`, wantNot: []string{"evil"}},
		{name: "protocol-relative raw image", content: `<img src="//evil.example/p.png">`, wantNot: []string{"evil", "<img"}},
		{name: "leading space protocol-relative raw link", content: `<a href=" //evil.example">x</a>`, wantNot: []string{"evil"}},
		{name: "tab inside protocol-relative raw link", content: `<a href="/&#x09;/evil.example">x</a>`, wantNot: []string{"evil"}},
		{name: "html comment", content: "<!-- <script>alert(1)</script> -->x", wantNot: []string{"<!--", "<script"}},
		{name: "textarea breakout", content: "<textarea></textarea><script>alert(1)</script>", wantNot: []string{"<textarea", "<script"}},
		{name: "noscript breakout", content: `<noscript><p title="</noscript><img src=x onerror=alert(1)>">`, wantNot: []string{"onerror", "<noscript"}},
		{name: "javascript autolink", content: "<javascript:alert(1)>", wantNot: []string{"href"}},
		{name: "javascript angle-bracket destination", content: "[x](<javascript:alert(1)>)", wantNot: []string{"href"}},
		{name: "javascript reference link", content: "[x][r]\n\n[r]: javascript:alert(1) \"t\"", wantNot: []string{"href", "javascript"}},
		{name: "percent-encoded scheme", content: "[p](%6Aavascript:alert(1)) <a href=\"%6Aavascript:alert(1)\">q</a>", wantNot: []string{"href"}},
		{name: "NUL inside scheme", content: "[n](java\x00script:alert(1)) <a href=\"java\x00script:alert(1)\">m</a>", wantNot: []string{"href"}},
		{name: "control char before scheme", content: "<a href=\"\x01javascript:alert(1)\">c</a>", wantNot: []string{"href"}},
		{name: "entity colon decimal", content: `<a href="javascript&#58;alert(1)">e</a>`, wantNot: []string{"href"}},
		{name: "image title with quotes", content: `![a](x.png "t\" onerror=alert(1) x=\"")`, want: []string{`title="t&#34; onerror=alert(1) x=&#34;"`}},
		{name: "link title with single quote", content: `[a](/x "t' onmouseover=alert(1) ")`, want: []string{`title="t&#39; onmouseover=alert(1) "`}},
		{name: "header attribute syntax", content: "# h {#root .c onclick=alert(1)}", want: []string{"<h1>h {#root .c onclick=alert(1)}</h1>"}},
		{name: "srcset dropped", content: `<img src="x.png" srcset="javascript:alert(1) 1x">`, wantNot: []string{"srcset", "javascript"}},
		{name: "cite dropped", content: `<q cite="javascript:alert(1)">q</q><blockquote cite="javascript:x">b</blockquote>`, wantNot: []string{"cite", "javascript"}},
		{name: "svg animate href", content: `<svg><animate attributeName=href values=javascript:alert(1) /><a><text>x</text></a></svg>`, wantNot: []string{"<svg", "<animate", "javascript"}},
		{name: "template script", content: "<template><script>alert(1)</script></template>", wantNot: []string{"<template", "alert"}},
		{name: "xmp raw text", content: "<xmp><img src=x onerror=alert(1)></xmp>", wantNot: []string{"<xmp", "<img"}},
		{name: "cdata section", content: "<![CDATA[<script>alert(1)</script>]]>", wantNot: []string{"<script", "CDATA"}},
		{name: "a ping and download dropped", content: `<a href="https://e.example" ping="https://e.example" download rel="opener">d</a>`, wantNot: []string{"ping", "download", "rel="}},
		{name: "checkbox with form attributes", content: `<input type=checkbox formaction=x form=f autofocus onfocus=alert(1)>`, want: []string{`<input type="checkbox"/>`}},
		{name: "wikilink resolving to a scheme stays a path", content: "[[JS]]", want: []string{`href="/wiki/javascript:alert%281%29"`}},
		{name: "wikilink resolving protocol-relative stays local", content: "[[Proto]]", want: []string{`href="/wiki/evil.example"`}},
		{name: "dot segments cannot build protocol-relative", content: "[x](/..//evil.example) [y](.//evil.example) [z](../../..//evil.example)", wantNot: []string{`href="//`}},
		{name: "wikilink alias with markup", content: `[[Foo|<img src=x onerror=alert(1)>]]`, want: []string{"&lt;img src=x onerror=alert(1)&gt;"}, wantNot: []string{"<img"}},
	}
	runRenderCases(t, cases, testOptions(map[string]string{
		"Foo": "/foo", "JS": "javascript:alert(1)", "Proto": "/../..//evil.example",
	}))
}

// previewSanitizeSchemaSHA256 pins the frontend sanitizer schema the crawler
// policy in policy.go is compared against. When the schema changes, check
// that the policy is still at least as strict (no id, name, class or style;
// no media; same schemes), then update the hash.
const previewSanitizeSchemaSHA256 = "3716d72d87b4b3250016788fa6863bd5a6201a57e8b963bc0ba5cc7be94f0300"

func TestPolicy_FrontendSchema_IsPinned(t *testing.T) {
	data, err := os.ReadFile("../../ui/leafwiki-ui/src/features/preview/previewSanitizeSchema.ts")
	if err != nil {
		t.Fatalf("read frontend schema: %v", err)
	}
	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	sum := sha256.Sum256([]byte(normalized))
	if got := hex.EncodeToString(sum[:]); got != previewSanitizeSchemaSHA256 {
		t.Fatalf("previewSanitizeSchema.ts changed (sha256 %s): re-check internal/render/policy.go against it, then update previewSanitizeSchemaSHA256", got)
	}
}
