package search

import (
	"fmt"
	"slices"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

// Substring matching (GitHub discussions #1826 and #1472): a literal word of
// at least minSubstringTermRunes runes also finds pages that contain it inside
// a longer token, through the pages_trigram table. Such pages rank after every
// page that matches the word as a token.

func searchIDs(t *testing.T, index *SQLiteIndex, query string, pageIDs []string) []string {
	t.Helper()
	result, err := index.Search(t.Context(), query, pageIDs, 0, 50)
	if err != nil {
		t.Fatalf("Search(%q) failed: %v", query, err)
	}
	got := resultPageIDs(result)
	if result.Count != len(got) {
		t.Fatalf("Search(%q): count %d but %d items %q", query, result.Count, len(got), got)
	}
	return got
}

func TestSQLiteIndex_Search_FindsAWordInsideALongerToken(t *testing.T) {
	index := newSubstringIndex(t,
		substringTestPage{id: "hyphenTitle", title: "Markdwon-Syntax", body: "How to write pages."},
		substringTestPage{id: "compound", title: "Wikisuche verwenden", body: "Die Volltextsuche findet Seiten."},
		substringTestPage{id: "dateHeading", title: "Logbuch", body: "## 2026-08-26\n\nFestplatte voll."},
		substringTestPage{id: "underscore", title: "Backup", body: "Cron runs backup_daily.sh at night."},
		substringTestPage{id: "camelCase", title: "API", body: "See buildFuzzyQuery in the code."},
		substringTestPage{id: "umlautCompound", title: "Störungsmeldung Straßenbeleuchtung", body: "Lampe defekt."},
		substringTestPage{id: "unrelated", title: "Unrelated", body: "Nothing to see here."},
	)

	tests := []struct {
		query string
		want  []string
	}{
		{query: "syntax", want: []string{"hyphenTitle"}},
		{query: "SYNTAX", want: []string{"hyphenTitle"}},
		{query: "suche", want: []string{"compound"}},
		{query: "08-26", want: []string{"dateHeading"}},
		{query: "-08-26", want: []string{"dateHeading"}},
		{query: "daily", want: []string{"underscore"}},
		{query: "fuzzy", want: []string{"camelCase"}},
		{query: "beleuchtung", want: []string{"umlautCompound"}},
		{query: "rungsmeld", want: []string{"umlautCompound"}},
		{query: "orungsmeld", want: []string{"umlautCompound"}},
		{query: "markdwon syntax", want: []string{"hyphenTitle"}},
		{query: "wwwnothing", want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			if got := searchIDs(t, index, tc.query, nil); !slices.Equal(got, tc.want) {
				t.Errorf("Search(%q) = %q, want %q", tc.query, got, tc.want)
			}
			ids, err := index.SearchPageIDs(t.Context(), tc.query, nil)
			if err != nil {
				t.Fatalf("SearchPageIDs(%q) failed: %v", tc.query, err)
			}
			if !slices.Equal(ids, tc.want) {
				t.Errorf("SearchPageIDs(%q) = %q, want %q", tc.query, ids, tc.want)
			}
		})
	}
}

func TestSQLiteIndex_Search_RanksSubstringOnlyMatchesAfterTokenMatches(t *testing.T) {
	// The substring-only page has the shortest title and the word in it, so
	// bm25 over one table would rank it first.
	index := newSubstringIndex(t,
		substringTestPage{id: "substringOnly", title: "Markdwon-Syntax", body: "Grammar."},
		substringTestPage{id: "prefixOnly", title: "Notes", body: "A long page about syntaxfehler and many other words."},
		substringTestPage{id: "wholeWord", title: "Notes", body: "A long page about the markdown syntax and many other words."},
	)

	want := []string{"wholeWord", "prefixOnly", "substringOnly"}
	if got := searchIDs(t, index, "syntax", nil); !slices.Equal(got, want) {
		t.Errorf("Search(syntax) = %q, want %q", got, want)
	}
	ids, err := index.SearchPageIDs(t.Context(), "syntax", nil)
	if err != nil {
		t.Fatalf("SearchPageIDs failed: %v", err)
	}
	if len(ids) != 3 || ids[2] != "substringOnly" {
		t.Errorf("SearchPageIDs(syntax) = %q, want the substring-only page last", ids)
	}
}

// TestSQLiteIndex_Search_PaginatesAcrossTheSubstringTier pages through a
// result whose token/substring boundary falls inside a page: every page
// reports the full count, and the pages together list every match exactly
// once, token matches first.
func TestSQLiteIndex_Search_PaginatesAcrossTheSubstringTier(t *testing.T) {
	index := newTestIndex(t)
	const perTier = 13
	var token, all []string
	for i := 0; i < perTier; i++ {
		tok, sub := fmt.Sprintf("token%02d", i), fmt.Sprintf("substring%02d", i)
		if err := index.IndexPage("docs/"+tok, "docs/"+tok+".md", tok, "Board", tree.NodeKindPage, "The syntax board."); err != nil {
			t.Fatalf("failed to index %s: %v", tok, err)
		}
		if err := index.IndexPage("docs/"+sub, "docs/"+sub+".md", sub, "Board", tree.NodeKindPage, "The markdown-syntax board."); err != nil {
			t.Fatalf("failed to index %s: %v", sub, err)
		}
		token = append(token, tok)
		all = append(all, tok, sub)
	}

	for _, limit := range []int{1, 5, 10, 50} {
		t.Run(fmt.Sprintf("limit %d", limit), func(t *testing.T) {
			var got []string
			for offset := 0; offset < len(all)+limit; offset += limit {
				result, err := index.Search(t.Context(), "syntax", nil, offset, limit)
				if err != nil {
					t.Fatalf("Search(offset %d) failed: %v", offset, err)
				}
				if result.Count != len(all) {
					t.Fatalf("Search(offset %d): count %d, want %d", offset, result.Count, len(all))
				}
				got = append(got, resultPageIDs(result)...)
			}
			if len(got) != len(all) {
				t.Fatalf("pages list %d results %q, want %d", len(got), got, len(all))
			}
			for _, id := range token {
				if rank := slices.Index(got, id); rank < 0 || rank >= perTier {
					t.Errorf("token match %q at rank %d, want within the first %d of %q", id, rank+1, perTier, got)
				}
			}
			seen := map[string]bool{}
			for _, id := range got {
				if seen[id] {
					t.Errorf("%q listed twice in %q", id, got)
				}
				seen[id] = true
			}
		})
	}
}

func TestSQLiteIndex_Search_SubstringMatchingOnlyForLiteralWordsOfThreeRunes(t *testing.T) {
	index := newSubstringIndex(t,
		substringTestPage{id: "hyphen", title: "Markdwon-Syntax", body: "Write pages."},
		substringTestPage{id: "piHyphen", title: "Raspberry Pi", body: "Uses the markdwon-syntax too."},
	)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "two runes match only as a prefix", query: "ax", want: []string{}},
		{name: "a short word must still match as a token", query: "pi syntax", want: []string{"piHyphen"}},
		{name: "column filter", query: "title:syntax", want: []string{}},
		{name: "explicit prefix", query: "syntax*", want: []string{}},
		{name: "quoted phrase", query: `"syntax"`, want: []string{}},
		{name: "boolean operator", query: "syntax OR zzz", want: []string{}},
		{name: "near call", query: "NEAR(syntax pages, 5)", want: []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := searchIDs(t, index, tc.query, nil); !slices.Equal(got, tc.want) {
				t.Errorf("Search(%q) = %q, want %q", tc.query, got, tc.want)
			}
		})
	}
}

func TestSQLiteIndex_Search_PageFilterAppliesToSubstringMatches(t *testing.T) {
	index := newSubstringIndex(t,
		substringTestPage{id: "a", title: "Markdwon-Syntax A", body: "x"},
		substringTestPage{id: "b", title: "Markdwon-Syntax B", body: "x"},
		substringTestPage{id: "c", title: "Plain syntax", body: "x"},
	)

	want := []string{"c", "b"}
	if got := searchIDs(t, index, "syntax", []string{"b", "c", "missing"}); !slices.Equal(got, want) {
		t.Errorf("Search with filter = %q, want %q", got, want)
	}
	ids, err := index.SearchPageIDs(t.Context(), "syntax", []string{"b", "c"})
	if err != nil {
		t.Fatalf("SearchPageIDs failed: %v", err)
	}
	if !slices.Equal(ids, want) {
		t.Errorf("SearchPageIDs with filter = %q, want %q", ids, want)
	}
}
