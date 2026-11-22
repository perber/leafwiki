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
		{name: "lowercase keyword is an ordinary word", input: "foo and bar", want: `"foo"* "and"* "bar"*`},
		{name: "mixed-case keyword is an ordinary word", input: "foo Or bar", want: `"foo"* "Or"* "bar"*`},
		{name: "trailing keyword is literal", input: "foo NOT", want: `"foo"* "NOT"*`},
		{name: "combining mark splits the word", input: "e\u0301clair", want: `"e"* "clair"*`},
		{name: "emoji-only query is empty", input: "🙂🙂🙂", want: ""},
		{name: "no-break space separates words", input: "foo\u00a0bar", want: `"foo"* "bar"*`},
		{name: "quote inside a word splits it", input: `abc"def`, want: `"abc"* "def"*`},
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
		{name: "whitespace-only query", input: " \t ", want: false},
		{name: "emoji-only query has no words but is literal", input: "🙂", want: true},
		{name: "lowercase keyword is an ordinary word", input: "foo and bar", want: true},
		{name: "quote inside a word", input: `abc"def`, want: true},
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
		{name: "whitespace only", input: " \t\n "},
		{name: "two-rune CJK word is a short term", input: "日本 日本語", wantSubstring: []string{"日本語"}, wantShort: []string{"日本"}},
		{name: "combining mark splits the word", input: "e\u0301clair", wantSubstring: []string{"clair"}, wantShort: []string{"e"}},
		{name: "token punctuation counts as word characters", input: "c++ ... #", wantSubstring: []string{"c++", "..."}, wantShort: []string{"#"}},
		{name: "emoji is dropped", input: "🙂 abc", wantSubstring: []string{"abc"}},
		{name: "literals around an operator field keep their order", input: "it's foo* barbaz", wantSubstring: []string{"barbaz"}, wantShort: []string{"it", "s"}},
		{name: "column filter value contributes no terms", input: "title:abc def", wantSubstring: []string{"def"}},
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

// TestQueryTermCount_CountsSearchTermsPerFieldKind pins how many terms each
// kind of field adds to the count the search use case caps: every word that
// reaches FTS5 as something to match counts once, boolean keywords and the
// NEAR keyword itself don't.
func TestQueryTermCount_CountsSearchTermsPerFieldKind(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{name: "empty", input: "", want: 0},
		{name: "whitespace only", input: " \t ", want: 0},
		{name: "plain words", input: "foo bar baz", want: 3},
		{name: "comma splits words", input: "a,b", want: 2},
		{name: "hyphenated word is one term", input: "foo-bar", want: 1},
		{name: "keyword between operands counts nothing", input: "x OR y", want: 2},
		{name: "dangling keyword is a literal word", input: "AND foo", want: 2},
		{name: "column filter counts its value words", input: "title:foo", want: 1},
		{name: "column filter with several value words", input: "title:foo,bar", want: 2},
		{name: "quoted phrase counts its words", input: `"a b"`, want: 2},
		{name: "operator field counts its barewords", input: "(apple OR cherry*) AND banana", want: 3},
		{name: "NEAR call counts its words but not NEAR or the distance", input: "NEAR(a b, 5)", want: 2},
		{name: "punctuation-only query", input: ",", want: 0},
		{name: "one-letter words each count", input: "a b c d", want: 4},
		{name: "every comma-separated word counts", input: "a,b,c,d,e,f,g,h,i,j,k,l,m", want: 13},
		{name: "dotted and slashed word is one term", input: "foo-bar-baz.v2/x", want: 1},
		{name: "each column filter counts its own words", input: "title:a title:b content:c,d", want: 4},
		{name: "unknown column counts as literal words", input: "nope:a,b", want: 3},
		{name: "nested groups count only barewords", input: "((a OR b) AND (c OR d*))", want: 4},
		{name: "prefix operators count their words", input: "a* b* c*", want: 3},
		{name: "long quoted phrase counts every word", input: `"a b c d e f g h i j k l m"`, want: 13},
		{name: "quoted phrase prefix counts its words", input: `"a b"*`, want: 2},
		{name: "unbalanced quote counts the literal words", input: `"a b c`, want: 3},
		{name: "NOT between operands counts nothing", input: "a NOT b", want: 2},
		{name: "doubled keyword counts as literal words", input: "a AND AND b", want: 4},
		{name: "mixed operators, filters and phrases", input: `(a OR b) NOT title:c,d "e f" g*`, want: 7},
		{name: "CJK runs count as one term each", input: "日本語 中文", want: 2},
		{name: "emoji are separators", input: "🙂foo🙂bar🙂", want: 2},
		{name: "NEAR with many words", input: "NEAR(a b c d e f g h i j k l m)", want: 13},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := QueryTermCount(tc.input); got != tc.want {
				t.Errorf("QueryTermCount(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TestSearchTiers_MatchExpressions pins the MATCH arguments of the rank
// tiers, best tier first. They are built only from splitIntoIndexWords output,
// so quotes, apostrophes and SQL punctuation can never reach them as syntax.
func TestSearchTiers_MatchExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "empty query has no tier", input: ""},
		{name: "single word has only the whole-word tier", input: "pi", want: []string{`"pi"`}},
		{name: "two words add the last-word-prefix tier", input: "pi setup", want: []string{`"pi" "setup"`, `"pi" "setup"*`}},
		{name: "only the last word gets the prefix", input: "a b c", want: []string{`"a" "b" "c"`, `"a" "b" "c"*`}},
		// FTS5 ANDs a repeated term, so a page needs the word only once:
		// "Raspberry Pi Setup" has set only as a prefix of setup, which
		// leaves it in the last tier.
		{name: "a repeated word stays in every tier", input: "set pi set", want: []string{`"set" "pi" "set"`, `"set" "pi" "set"*`}},
		{name: "apostrophe splits words", input: "it's", want: []string{`"it" "s"`, `"it" "s"*`}},
		{name: "quote inside a word splits it", input: `abc"def`, want: []string{`"abc" "def"`, `"abc" "def"*`}},
		{name: "unbalanced quote is literal", input: `"pi`, want: []string{`"pi"`}},
		{name: "SQL punctuation is dropped", input: `pi'); DROP TABLE pages;--`,
			want: []string{`"pi" "DROP" "TABLE" "pages" "--"`, `"pi" "DROP" "TABLE" "pages" "--"*`}},
		{name: "emoji is not a word", input: "pi 🚀", want: []string{`"pi"`}},
		{name: "punctuation-only query has no tier", input: ","},
		{name: "prefix operator has no tier", input: "pi*"},
		{name: "quoted phrase has no tier", input: `"pi"`},
		{name: "column filter has no tier", input: "title:pi"},
		{name: "keyword between operands has no tier", input: "pi OR x"},
		{name: "lowercase keyword is a tier word", input: "pi and setup", want: []string{`"pi" "and" "setup"`, `"pi" "and" "setup"*`}},
		{name: "one operator field disables the tier for the whole query", input: "pi* setup"},
		{name: "NEAR call has no tier", input: "NEAR(pi setup, 5)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, tier := range searchTiers(parseSearchQuery(tc.input)) {
				if len(tier.args) != 1 {
					t.Fatalf("searchTiers(%q): tier %q has %d args, want its one MATCH expression", tc.input, tier.cond, len(tier.args))
				}
				got = append(got, tier.args[0].(string))
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("searchTiers(%q) MATCH expressions = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
