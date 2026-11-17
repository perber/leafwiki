package search

import (
	"slices"
	"testing"
)

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

func TestParseSearchQuery_LiteralOnlyPerFieldKind(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "empty query has nothing to match", input: "", want: false},
		{name: "plain words", input: "foo bar", want: true},
		{name: "punctuation is split, not syntax", input: "it's bar,baz", want: true},
		{name: "punctuation-only query", input: ",", want: true},
		{name: "malformed operator falls back to literal", input: "report.v2*", want: true},
		{name: "lone paren falls back to literal", input: "(foo", want: true},
		{name: "dangling keyword falls back to literal", input: "AND foo", want: true},
		{name: "unbalanced quote falls back to literal", input: `"foo`, want: true},
		{name: "colon without a real column", input: "http://example.com", want: true},
		{name: "prefix operator", input: "foo*", want: false},
		{name: "one operator field among literals", input: "foo* bar", want: false},
		{name: "paren group", input: "(apple OR cherry) AND banana", want: false},
		{name: "keyword between operands", input: "foo AND bar", want: false},
		{name: "quoted phrase", input: `"foo bar"`, want: false},
		{name: "column filter", input: "content:search", want: false},
		{name: "NEAR call", input: "NEAR(foo bar, 5)", want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseSearchQuery(tc.input).literalOnly; got != tc.want {
				t.Errorf("parseSearchQuery(%q).literalOnly = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseSearchQuery_SplitsLiteralWordsByLength(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantSubstring []string
		wantShort     []string
	}{
		{name: "empty", input: ""},
		{name: "words keep their order", input: "syntax markdown", wantSubstring: []string{"syntax", "markdown"}},
		{name: "hyphenated word is one term", input: "Markdwon-Syntax", wantSubstring: []string{"Markdwon-Syntax"}},
		{name: "length counts runes, not bytes", input: "über äö", wantSubstring: []string{"über"}, wantShort: []string{"äö"}},
		{name: "three runes is a substring term", input: "abc", wantSubstring: []string{"abc"}},
		{name: "two-rune words are short terms", input: "go is fun", wantSubstring: []string{"fun"}, wantShort: []string{"go", "is"}},
		{name: "punctuation splits words", input: "it's bar,baz", wantSubstring: []string{"bar", "baz"}, wantShort: []string{"it", "s"}},
		{name: "punctuation-only query has no terms", input: ","},
		{name: "operator field contributes no terms", input: "foo* barbaz", wantSubstring: []string{"barbaz"}},
		{name: "keyword between operands contributes no terms", input: "foo AND bar", wantSubstring: []string{"foo", "bar"}},
		{name: "dangling keyword is a literal word", input: "AND foo", wantSubstring: []string{"AND", "foo"}},
		{name: "quoted phrase contributes no terms", input: `"foo bar" baz`, wantSubstring: []string{"baz"}},
		{name: "column filter contributes no terms", input: "content:search page", wantSubstring: []string{"page"}},
		{name: "NEAR call contributes no terms", input: "NEAR(foo bar, 5)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseSearchQuery(tc.input)
			if !slices.Equal(got.substringTerms, tc.wantSubstring) {
				t.Errorf("parseSearchQuery(%q).substringTerms = %q, want %q", tc.input, got.substringTerms, tc.wantSubstring)
			}
			if !slices.Equal(got.shortTerms, tc.wantShort) {
				t.Errorf("parseSearchQuery(%q).shortTerms = %q, want %q", tc.input, got.shortTerms, tc.wantShort)
			}
		})
	}
}
