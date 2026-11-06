package render

import "testing"

func TestRenderHTML_Links_ResolveFolderRelativeWithBasePath(t *testing.T) {
	cases := []renderCase{
		{name: "relative is below the page", content: "[x](child)", want: []string{`<a href="/wiki/docs/page/child">x</a>`}},
		{name: "dot relative", content: "[x](./child)", want: []string{`<a href="/wiki/docs/page/child">x</a>`}},
		{name: "parent relative", content: "[x](../other)", want: []string{`<a href="/wiki/docs/other">x</a>`}},
		{name: "above root stays at root", content: "[x](../../../top)", want: []string{`<a href="/wiki/top">x</a>`}},
		{name: "absolute route", content: "[x](/guide/setup)", want: []string{`<a href="/wiki/guide/setup">x</a>`}},
		{name: "trailing slash dropped", content: "[x](/guide/)", want: []string{`<a href="/wiki/guide">x</a>`}},
		{name: "query and fragment kept", content: "[x](child?a=1&b=2#sec)", want: []string{`<a href="/wiki/docs/page/child?a=1&amp;b=2#sec">x</a>`}},
		{name: "escaped space kept escaped", content: "[x](my%20page)", want: []string{`<a href="/wiki/docs/page/my%20page">x</a>`}},
		{name: "angle bracket space escaped", content: "[x](</a b>)", want: []string{`<a href="/wiki/a%20b">x</a>`}},
		{name: "unicode escaped", content: "[x](/über)", want: []string{`<a href="/wiki/%C3%BCber">x</a>`}},
		{name: "percent sign escaped", content: "[x](/100%25)", want: []string{`<a href="/wiki/100%25">x</a>`}},
		{name: "fragment only unchanged", content: "[x](#sec)", want: []string{`<a href="#sec">x</a>`}},
		{name: "https unchanged", content: "[x](https://example.com/a?b=1)", want: []string{`<a href="https://example.com/a?b=1">x</a>`}},
		{name: "mailto unchanged", content: "[x](mailto:a@example.com)", want: []string{`<a href="mailto:a@example.com">x</a>`}},
		{name: "relative asset link", content: "[f](assets/p1/file.pdf)", want: []string{`<a href="/wiki/assets/p1/file.pdf">f</a>`}},
		{name: "absolute asset link", content: "[f](/assets/p1/file.pdf)", want: []string{`<a href="/wiki/assets/p1/file.pdf">f</a>`}},
		{name: "protocol-relative dropped", content: "[x](//evil.example)", want: []string{"<p>x</p>"}, wantNot: []string{"evil"}},
		{name: "backslash protocol-relative dropped", content: `[x](/\evil.example)`, wantNot: []string{`href="/\`, `href="\`}},
	}
	runRenderCases(t, cases, testOptions(nil))
}

func TestRenderHTML_Links_BasePathForms(t *testing.T) {
	cases := []struct {
		name     string
		basePath string
		want     string
	}{
		{name: "no base path", basePath: "", want: `<a href="/docs/page/child">x</a>`},
		{name: "root base path", basePath: "/", want: `<a href="/docs/page/child">x</a>`},
		{name: "trailing slash", basePath: "/wiki/", want: `<a href="/wiki/docs/page/child">x</a>`},
		{name: "missing leading slash", basePath: "wiki", want: `<a href="/wiki/docs/page/child">x</a>`},
	}
	for _, tc := range cases {
		opt := testOptions(nil)
		opt.BasePath = tc.basePath
		runRenderCases(t, []renderCase{{name: tc.name, content: "[x](child)", want: []string{tc.want}}}, opt)
	}
}

func TestRenderHTML_Images_UseBasePathForAssets(t *testing.T) {
	cases := []renderCase{
		{name: "relative asset", content: "![pic](assets/p1/x.png)", want: []string{`<img src="/wiki/assets/p1/x.png" alt="pic"`}},
		{name: "absolute asset", content: "![pic](/assets/p1/x.png)", want: []string{`<img src="/wiki/assets/p1/x.png" alt="pic"`}},
		{name: "api url", content: "![pic](/api/branding/logo.png)", want: []string{`<img src="/wiki/api/branding/logo.png" alt="pic"`}},
		{name: "external unchanged", content: "![pic](https://example.com/x.png)", want: []string{`<img src="https://example.com/x.png" alt="pic"`}},
		{name: "other relative unchanged", content: "![pic](x.png)", want: []string{`<img src="x.png" alt="pic"`}},
		{name: "title kept", content: `![pic](assets/x.png "Title")`, want: []string{`title="Title"`}},
		{name: "raw img kept with size", content: `<img src="https://example.com/x.png" alt="a" width="100" height="50%">`, want: []string{`<img src="https://example.com/x.png" alt="a" width="100" height="50%"/>`}},
		{name: "protocol-relative src dropped", content: "![pic](//evil.example/x.png)", want: []string{`<img alt="pic"/>`}, wantNot: []string{"src=", "evil"}},
	}
	runRenderCases(t, cases, testOptions(nil))
}
