package markdown

import (
	"reflect"
	"testing"
)

func TestParseWikiLinks(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    []WikiLink
	}{
		{"empty", "", nil},
		{"no wikilinks", "No wiki links here.", nil},
		{"single", "See [[Foo]].", []WikiLink{{Target: "Foo", Start: 4, End: 11}}},
		{"alias", "[[Foo|the foo]]", []WikiLink{{Target: "Foo", Alias: "the foo", Start: 0, End: 15}}},
		{"path hint", "[[Folder/Page]]", []WikiLink{{Target: "Folder/Page", Start: 0, End: 15}}},
		{"trailing space trimmed", "[[Foo ]]", []WikiLink{{Target: "Foo", Start: 0, End: 8}}},
		{"unicode offsets are bytes", "Ü [[Bar]]", []WikiLink{{Target: "Bar", Start: 3, End: 10}}},
		{"duplicates kept in order", "[[A]] [[B]] [[A]]", []WikiLink{
			{Target: "A", Start: 0, End: 5},
			{Target: "B", Start: 6, End: 11},
			{Target: "A", Start: 12, End: 17},
		}},
		// The \S first character rejects bash-style [[ -n ... ]] conditionals.
		{"bash conditional", "if [[ -n $x ]]; then", nil},
		{"leading space", "[[ Foo ]]", nil},
		{"fragment not matched", "[[Foo#bar]]", nil},
		{"newline not matched", "[[Foo\nBar]]", nil},
		{"empty brackets", "[[]]", nil},
		// Code spans are not excluded here; callers that care skip them.
		{"inside inline code", "`[[Foo]]`", []WikiLink{{Target: "Foo", Start: 1, End: 8}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseWikiLinks(tc.content)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseWikiLinks(%q) = %#v, want %#v", tc.content, got, tc.want)
			}
		})
	}
}

func TestNormalizeRoutePath(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"root", "/", "/"},
		{"adds leading slash", "docs/page", "/docs/page"},
		{"strips trailing slash", "/docs/page/", "/docs/page"},
		{"strips several trailing slashes", "/docs///", "/docs"},
		{"collapses double slashes", "//docs//page", "/docs/page"},
		{"strips fragment", "/docs/page#section", "/docs/page"},
		{"strips query", "/docs/page?x=1", "/docs/page"},
		{"strips query before fragment", "/docs/page?x=1#y", "/docs/page"},
		{"only fragment becomes root", "#top", "/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeRoutePath(tc.in); got != tc.want {
				t.Fatalf("NormalizeRoutePath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveFolderRelative(t *testing.T) {
	cases := []struct {
		name        string
		currentPath string
		href        string
		want        string
	}{
		{"child of current page", "/docs", "intro", "/docs/intro"},
		{"dot slash child", "/docs", "./intro", "/docs/intro"},
		{"sibling via parent", "/docs/a", "../b", "/docs/b"},
		{"parent above root clamps", "/docs", "../../x", "/x"},
		{"absolute href", "/docs/a", "/other/page", "/other/page"},
		{"current path without leading slash", "docs", "intro", "/docs/intro"},
		{"current path with trailing slash", "/docs/", "intro", "/docs/intro"},
		{"root current path", "/", "intro", "/intro"},
		{"empty current path", "", "intro", "/intro"},
		{"trailing slash in href", "/docs", "intro/", "/docs/intro"},
		{"query and fragment dropped", "/docs", "intro?x=1#y", "/docs/intro"},
		{"percent-encoded segment decoded", "/docs", "my%20page", "/docs/my page"},
		{"dot is current page", "/docs", ".", "/docs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveFolderRelative(tc.currentPath, tc.href)
			if err != nil {
				t.Fatalf("ResolveFolderRelative(%q, %q) error: %v", tc.currentPath, tc.href, err)
			}
			if got != tc.want {
				t.Fatalf("ResolveFolderRelative(%q, %q) = %q, want %q", tc.currentPath, tc.href, got, tc.want)
			}
		})
	}
}

func TestResolveFolderRelative_InvalidHrefReturnsError(t *testing.T) {
	if _, err := ResolveFolderRelative("/docs", "%zz"); err == nil {
		t.Fatal("expected an error for an unparsable href")
	}
}
