package search

import "testing"

// TestBuildFuzzyQuery_TranslatesFieldsToFTS5 pins the exact MATCH argument
// for each kind of field (literal text, trusted operator syntax, column
// filter, NEAR call), so restructuring the parser can be checked for
// byte-identical output. Whether the output is valid FTS5 is covered by the
// SQLiteIndex.Search tests in sqlite_index_test.go.
func TestBuildFuzzyQuery_TranslatesFieldsToFTS5(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "single word becomes quoted prefix", input: "search", want: `"search"*`},
		{name: "surrounding whitespace is trimmed", input: "  search  ", want: `"search"*`},
		{name: "words are ANDed", input: "foo bar", want: `"foo"* "bar"*`},
		{name: "comma splits words", input: "bar,baz", want: `"bar"* "baz"*`},
		{name: "apostrophe splits words", input: "it's", want: `"it"* "s"*`},
		{name: "hyphenated word stays one token", input: "Markdwon-Syntax", want: `"Markdwon-Syntax"*`},
		{name: "non-ASCII letters are word characters", input: "über", want: `"über"*`},
		{name: "punctuation-only query is empty", input: ",", want: ""},
		{name: "punctuation-only field leaves an empty slot", input: "foo ,", want: `"foo"* `},
		{name: "prefix operator is trusted", input: "foo*", want: "foo*"},
		{name: "operator field does not vouch for its neighbour", input: "foo* bar,baz", want: `foo* "bar"* "baz"*`},
		{name: "operator with token punctuation is literal", input: "report.v2*", want: `"report.v2"*`},
		{name: "bare star is dropped", input: "*", want: ""},
		{name: "balanced paren group is trusted", input: "(apple OR cherry) AND banana", want: `(apple OR cherry) AND "banana"*`},
		{name: "lone open paren is literal", input: "(foo", want: `"foo"*`},
		{name: "keyword between operands is trusted", input: "foo AND bar", want: `"foo"* AND "bar"*`},
		{name: "dangling keyword is literal", input: "AND foo", want: `"AND"* "foo"*`},
		{name: "doubled keyword is literal", input: "foo AND AND bar", want: `"foo"* "AND"* "AND"* "bar"*`},
		{name: "balanced quoted phrase is trusted", input: `"foo bar"`, want: `"foo bar"`},
		{name: "quoted phrase prefix is trusted", input: `"foo bar"*`, want: `"foo bar"*`},
		{name: "unbalanced quote is literal", input: `"foo`, want: `"foo"*`},
		{name: "column filter is rewritten per word", input: "content:search", want: `content:("search"*)`},
		{name: "column filter keeps column case and splits value", input: "Title:foo,bar", want: `Title:("foo"* "bar"*)`},
		{name: "colon without a real column is literal", input: "http://example.com", want: `"http"* "//example.com"*`},
		{name: "column filter without words is literal", input: "title:,", want: `"title"*`},
		{name: "NEAR call is trusted verbatim", input: "NEAR(foo bar, 5)", want: "NEAR(foo bar, 5)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildFuzzyQuery(tc.input); got != tc.want {
				t.Errorf("buildFuzzyQuery(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
