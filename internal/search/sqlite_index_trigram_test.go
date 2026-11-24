package search

import (
	"database/sql"
	"slices"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
)

// pages_trigram holds a row for every pages row, under the same rowid, for
// substring matching (ADR-0025). Every write path must change both tables.

type substringTestPage struct{ id, title, body string }

func newSubstringIndex(t *testing.T, pages ...substringTestPage) *SQLiteIndex {
	t.Helper()
	index := newTestIndex(t)
	for _, p := range pages {
		if err := index.IndexPage("docs/"+p.id, "docs/"+p.id+".md", p.id, p.title, tree.NodeKindPage, p.body); err != nil {
			t.Fatalf("failed to index %s: %v", p.id, err)
		}
	}
	return index
}

func trigramRowCount(t *testing.T, index *SQLiteIndex) (pages, trigram int) {
	t.Helper()
	if err := index.withDB(func(db *sql.DB) error {
		if err := db.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&pages); err != nil {
			return err
		}
		return db.QueryRow(`SELECT COUNT(*) FROM pages_trigram`).Scan(&trigram)
	}); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return pages, trigram
}

// trigramMatches returns the pageIDs whose pages_trigram row matches the
// substring phrase, by joining on the shared rowid.
func trigramMatches(t *testing.T, index *SQLiteIndex, phrase string) []string {
	t.Helper()
	var ids []string
	if err := index.withDB(func(db *sql.DB) error {
		rows, err := db.Query(`
			SELECT p.pageID FROM pages_trigram CROSS JOIN pages AS p ON p.rowid = pages_trigram.rowid
			WHERE pages_trigram MATCH ? ORDER BY p.pageID`, `"`+phrase+`"`)
		if err != nil {
			return err
		}
		defer closeRows(rows)
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		t.Fatalf("query pages_trigram: %v", err)
	}
	return ids
}

func TestSQLiteIndex_TrigramIndexHoldsTheTextOfEveryPage(t *testing.T) {
	index := newSubstringIndex(t,
		substringTestPage{id: "title", title: "Markdwon-Syntax", body: "x"},
		substringTestPage{id: "heading", title: "Log", body: "## 2026-08-26\n\nFull disk."},
		substringTestPage{id: "body", title: "Lights", body: "Störung der Straßenbeleuchtung & mehr"},
	)

	tests := []struct{ phrase, want string }{
		{phrase: "syntax", want: "title"},
		{phrase: "08-26", want: "heading"},
		{phrase: "beleuchtung", want: "body"},
		{phrase: "storung", want: "body"},
		// The text is stored unescaped: "&" is there, its entity is not.
		{phrase: "& mehr", want: "body"},
	}
	for _, tc := range tests {
		t.Run(tc.phrase, func(t *testing.T) {
			if got := trigramMatches(t, index, tc.phrase); len(got) != 1 || got[0] != tc.want {
				t.Errorf("pages_trigram MATCH %q = %q, want [%q]", tc.phrase, got, tc.want)
			}
		})
	}
	if got := trigramMatches(t, index, "amp;"); len(got) != 0 {
		t.Errorf("pages_trigram MATCH \"amp;\" = %q, want no match inside an entity", got)
	}
}

func TestSQLiteIndex_WritePathsKeepTheTrigramIndexInSync(t *testing.T) {
	pages := []substringTestPage{
		{id: "a", title: "Markdwon-Syntax A", body: "x"},
		{id: "b", title: "Markdwon-Syntax B", body: "x"},
		{id: "c", title: "Markdwon-Syntax C", body: "x"},
	}
	tests := []struct {
		name  string
		write func(t *testing.T, index *SQLiteIndex)
		want  []string
		rows  int // pages rows left; 0 means len(want)
	}{
		{name: "reindex replaces the old text", want: []string{"b", "c"}, rows: 3, write: func(t *testing.T, index *SQLiteIndex) {
			if err := index.IndexPage("docs/a", "docs/a.md", "a", "Renamed", tree.NodeKindPage, "x"); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "IndexPages replaces the old text", want: []string{"c"}, rows: 3, write: func(t *testing.T, index *SQLiteIndex) {
			if _, err := index.IndexPages([]IndexPageInput{
				{Path: "docs/a", FilePath: "docs/a.md", PageID: "a", Title: "Renamed", Kind: tree.NodeKindPage, Raw: "x"},
				{Path: "docs/b", FilePath: "docs/b.md", PageID: "b", Title: "Renamed", Kind: tree.NodeKindPage, Raw: "x"},
			}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "RemovePage", want: []string{"b", "c"}, write: func(t *testing.T, index *SQLiteIndex) {
			if err := index.RemovePage("a"); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "RemovePages", want: []string{"b"}, write: func(t *testing.T, index *SQLiteIndex) {
			if err := index.RemovePages([]string{"a", "c"}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "RemovePageByFilePath", want: []string{"a", "c"}, write: func(t *testing.T, index *SQLiteIndex) {
			n, err := index.RemovePageByFilePath("docs/b.md")
			if err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("RemovePageByFilePath removed %d rows, want 1", n)
			}
		}},
		{name: "Clear", want: nil, write: func(t *testing.T, index *SQLiteIndex) {
			if err := index.Clear(); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			index := newSubstringIndex(t, pages...)
			tc.write(t, index)

			if got := trigramMatches(t, index, "syntax"); !slices.Equal(got, tc.want) {
				t.Errorf("pages_trigram MATCH syntax after %s = %q, want %q", tc.name, got, tc.want)
			}
			rows := tc.rows
			if rows == 0 {
				rows = len(tc.want)
			}
			if p, tg := trigramRowCount(t, index); p != rows || tg != p {
				t.Errorf("after %s: %d pages rows, %d pages_trigram rows, want %d of each", tc.name, p, tg, rows)
			}
		})
	}
}
