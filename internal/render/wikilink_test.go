package render

import (
	"reflect"
	"testing"
)

func TestRenderHTML_WikiLinks_ResolveThroughCallback(t *testing.T) {
	pages := map[string]string{
		"Foo":         "/docs/foo",
		"Folder/Page": "/folder/page",
		"Spaced":      "/my page/über",
		"No Slash":    "plain",
	}
	cases := []renderCase{
		{name: "resolved", content: "See [[Foo]].", want: []string{`See <a href="/wiki/docs/foo">Foo</a>.`}},
		{name: "alias is the text", content: "[[Foo|the foo]]", want: []string{`<a href="/wiki/docs/foo">the foo</a>`}},
		{name: "alias is trimmed", content: "[[Foo| the foo ]]", want: []string{`<a href="/wiki/docs/foo">the foo</a>`}},
		{name: "path hint passed unchanged", content: "[[Folder/Page]]", want: []string{`<a href="/wiki/folder/page">Folder/Page</a>`}},
		{name: "route segments escaped", content: "[[Spaced]]", want: []string{`<a href="/wiki/my%20page/%C3%BCber">Spaced</a>`}},
		{name: "route without leading slash", content: "[[No Slash]]", want: []string{`<a href="/wiki/plain">No Slash</a>`}},
		{name: "unresolved is plain text", content: "x [[Missing]] y", want: []string{"<p>x Missing y</p>"}, wantNot: []string{"<a"}},
		{name: "unresolved alias is plain text", content: "[[Missing|shown]]", want: []string{"<p>shown</p>"}, wantNot: []string{"Missing", "<a"}},
		{name: "unresolved text escaped", content: "[[<b>x</b>]]", want: []string{"&lt;b&gt;x&lt;/b&gt;"}, wantNot: []string{"<b>"}},
		{name: "inline code untouched", content: "`[[Foo]]`", want: []string{"<code>[[Foo]]</code>"}, wantNot: []string{"<a"}},
		{name: "fenced code untouched", content: "```\n[[Foo]]\n```", want: []string{"<pre><code>[[Foo]]\n</code></pre>"}, wantNot: []string{"<a"}},
		{name: "bash conditional is not a link", content: "if [[ -n $x ]]; then", want: []string{"[[ -n $x ]]"}, wantNot: []string{"<a"}},
		{name: "several on one line", content: "[[Foo]] and [[Missing]] and [[Foo|again]]", want: []string{`<a href="/wiki/docs/foo">Foo</a> and Missing and <a href="/wiki/docs/foo">again</a>`}},
		{name: "normal link still parsed", content: "[[Foo]] [text](/x)", want: []string{`<a href="/wiki/docs/foo">Foo</a>`, `<a href="/wiki/x">text</a>`}},
		{name: "inside emphasis", content: "*see [[Foo]]*", want: []string{`<em>see <a href="/wiki/docs/foo">Foo</a></em>`}},
	}
	runRenderCases(t, cases, testOptions(pages))
}

func TestRenderHTML_WikiLinks_CallbackSeesTargetsOutsideCodeOnly(t *testing.T) {
	var got []string
	opt := testOptions(nil)
	opt.ResolveWikiLink = func(target string) (string, bool) {
		got = append(got, target)
		return "", false
	}
	renderForTest(t, "[[A]] `[[B]]` [[C|alias]]\n\n```\n[[D]]\n```\n\n    [[E]]\n", opt)
	if want := []string{"A", "C"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("resolver called with %q, want %q", got, want)
	}
}

func TestRenderHTML_WikiLinks_NilResolverRendersText(t *testing.T) {
	opt := testOptions(nil)
	opt.ResolveWikiLink = nil
	runRenderCases(t, []renderCase{
		{name: "plain text", content: "[[Foo|bar]]", want: []string{"<p>bar</p>"}, wantNot: []string{"<a"}},
	}, opt)
}
