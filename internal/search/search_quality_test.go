package search

// Search-quality regression set.
//
// Search changes (GitHub discussions #1826 and #1472) are measured before they
// are made: this file indexes a small, realistic German/English wiki through
// the real SQLiteIndex and runs the queries people actually type against it.
// Every case is either expected to hit (a failure is a regression) or marked
// knownMiss with the reason it fails today. A knownMiss case that starts to
// pass also fails the test, so an improvement has to remove the mark and is
// locked in from then on. The test logs the hit rate per category.
//
// To extend the set, add a page to qualityCorpus or a row to qualityCases;
// run `go test -run TestSearchQuality -v ./internal/search/` to see the table.

import (
	"context"
	"fmt"
	"html"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/test_utils"
)

type qualityPage struct {
	path  string // slug path as the tree computes it
	title string
	kind  tree.NodeKind
	tags  []string // frontmatter only, like on disk
	body  string
}

var qualityCorpus = []qualityPage{
	{path: "handbuch", title: "Handbuch", kind: tree.NodeKindSection,
		body: "# Handbuch\n\nAlles zur Bedienung."},
	{path: "handbuch/markdown-syntax", title: "Markdown-Syntax", kind: tree.NodeKindPage, tags: []string{"markdown", "doku"},
		body: "# Markdown-Syntax\n\nLeafWiki nutzt GitHub-Flavored-Markdown.\n\n## Tabellen\n\nTabellen mit `|`.\n\n## Code-Blöcke\n\nDrei Backticks."},
	{path: "handbuch/markdown-cheatsheet", title: "Markdown Syntax Cheatsheet", kind: tree.NodeKindPage, tags: []string{"markdown"},
		body: "Kurzreferenz für die Markdown Syntax: **fett**, _kursiv_, [Link](https://example.com)."},
	{path: "handbuch/wikisuche", title: "Wikisuche verwenden", kind: tree.NodeKindPage, tags: []string{"suche"},
		body: "# Wikisuche\n\nDie Volltextsuche findet Seiten. Suchbegriffe werden mit UND verknüpft.\n\n## Suchoperatoren\n\n`title:` filtert nach Titel."},
	{path: "handbuch/installation-windows", title: "Installation unter Windows", kind: tree.NodeKindPage, tags: []string{"installation", "windows"},
		body: "# Installation\n\nLade `leafwiki-windows-amd64.exe` herunter und starte es.\n\nDie Konfiguration liegt in `C:\\ProgramData\\LeafWiki\\config.yaml`."},
	{path: "handbuch/raspberry-pi", title: "Raspberry Pi einrichten", kind: tree.NodeKindPage, tags: []string{"installation", "raspi"},
		body: "# Raspberry Pi\n\nAuf dem Raspberry-Pi 4 installieren wir das arm64-Binary.\n\n```bash\nsudo systemctl enable leafwiki.service\n```\n\nDer Dienst lauscht auf 192.168.1.42:8080."},
	{path: "betrieb", title: "Betrieb", kind: tree.NodeKindSection,
		body: "# Betrieb\n\nRunbooks und Protokolle."},
	{path: "betrieb/protokolle-2026", title: "Protokolle 2026", kind: tree.NodeKindSection},
	{path: "betrieb/protokolle-2026/logbuch-august", title: "Logbuch August", kind: tree.NodeKindPage, tags: []string{"log"},
		body: "# Logbuch\n\n## 2026-08-26\n\nBackup-Job fehlgeschlagen, Festplatte voll.\n\n## 2026-08-27\n\nSpeicherplatz erweitert."},
	{path: "betrieb/protokolle-2026/logbuch-november", title: "Logbuch November", kind: tree.NodeKindPage, tags: []string{"log"},
		body: "# Logbuch\n\n## 2026-11-03\n\nUpdate auf v0.14.2 eingespielt.\n\n## 2026-11-12\n\nZertifikat erneuert (Let's Encrypt)."},
	{path: "betrieb/backup", title: "Backup und Wiederherstellung", kind: tree.NodeKindPage, tags: []string{"backup"},
		body: "# Backup\n\nDas Skript `backup_daily.sh` sichert `/var/lib/leafwiki`.\n\nWiederherstellen mit `leafwiki restore-snapshot`."},
	{path: "betrieb/datenbankserver", title: "Datenbankserver", kind: tree.NodeKindPage, tags: []string{"postgres"},
		body: "# Datenbankserver\n\nUnser PostgreSQL-Server läuft auf db01. Die Server werden nachts gepatcht.\n\nVerbindung: `postgres://app@db01:5432/wiki`."},
	{path: "betrieb/stoerung-strasse", title: "Störungsmeldung Straßenbeleuchtung", kind: tree.NodeKindPage,
		body: "# Störung\n\nDie Straßenbeleuchtung in der Hauptstraße ist ausgefallen. Größere Reparatur nötig."},
	{path: "entwicklung", title: "Entwicklung", kind: tree.NodeKindSection,
		body: "# Entwicklung\n\nAlles für Entwickler."},
	{path: "entwicklung/api-referenz", title: "API-Referenz", kind: tree.NodeKindPage, tags: []string{"api"},
		body: "# API-Referenz\n\nDer Endpunkt `GET /api/search?q=` liefert Treffer.\n\nDie Funktion `buildFuzzyQuery` baut die Abfrage. Siehe auch `SearchPageIDs` und `user_id`."},
	{path: "entwicklung/cpp-csharp", title: "C++ und C# Snippets", kind: tree.NodeKindPage, tags: []string{"code"},
		body: "# Snippets\n\nC++ Templates und C# LINQ Beispiele.\n\n```cpp\nstd::vector<int> v;\n```"},
	{path: "entwicklung/release-prozess", title: "Release-Prozess", kind: tree.NodeKindPage, tags: []string{"release"},
		body: "# Release\n\nVersionen folgen SemVer, z. B. v1.2.3 oder 0.14.2.\n\nDas Changelog steht in CHANGELOG.md."},
	{path: "entwicklung/pipeline", title: "Pipeline", kind: tree.NodeKindPage, tags: []string{"ci"},
		body: "# Pipeline\n\nDie Build-Pipeline läuft bei jedem Push auf main."},
	{path: "entwicklung/kubernetes", title: "Kubernetes Deployment", kind: tree.NodeKindPage, tags: []string{"k8s", "deployment"},
		body: "# Kubernetes\n\nWe deploy with Helm charts. The ingress-nginx controller terminates TLS.\n\nPods are restarted via `kubectl rollout restart deployment/leafwiki`."},
	{path: "entwicklung/meeting-notes-q3", title: "Meeting Notes Q3", kind: tree.NodeKindPage, tags: []string{"meeting"},
		body: "# Meeting notes\n\nWe discussed onboarding, the e-mail templates and the self-hosted setup.\n\nAction item: improve searching for hyphenated words."},
	{path: "onboarding", title: "Onboarding neuer Mitarbeiter", kind: tree.NodeKindPage, tags: []string{"hr"},
		body: "# Onboarding\n\nNeue Mitarbeiterinnen und Mitarbeiter erhalten einen Laptop. Der Mitarbeiterausweis kommt per Post.\n\nAnsprechpartnerin: Jördis Müller-Lüdenscheidt."},
	{path: "urlaubsantrag", title: "Urlaubsantrag", kind: tree.NodeKindPage, tags: []string{"hr"},
		body: "# Urlaub\n\nUrlaubsanträge bitte drei Wochen vorher stellen. Resturlaub verfällt am 31.03."},
	{path: "glossar", title: "Glossar", kind: tree.NodeKindPage, tags: []string{"doku"},
		body: "# Glossar\n\n- **Seite**: ein Markdown-Dokument\n- **Abschnitt**: eine Seite mit Unterseiten\n- **Permalink**: stabile URL\n\nSiehe [Markdown-Syntax](/handbuch/markdown-syntax)."},
	{path: "faq", title: "FAQ", kind: tree.NodeKindPage, tags: []string{"doku"},
		body: "# FAQ\n\n## Wie ändere ich mein Passwort?\n\nIm Profil.\n\n## Why can't I log in?\n\nCheck the e-mail address. It's case-insensitive."},
	{path: "japanese", title: "日本語ページ", kind: tree.NodeKindPage,
		body: "# 日本語\n\n検索エンジンのテストです。東京オフィス。"},
	{path: "emoji-status", title: "Emoji Statusseite", kind: tree.NodeKindPage,
		body: "# Status 🚀\n\nAlles grün ✅. Deploy läuft 🙂."},
}

// qualityCase is one query and what a user expects from it. The case hits
// when one of want is among the first maxRank results (0 means 1) and none of
// notWant is in the result at all; an empty want expects no result.
type qualityCase struct {
	category  string
	name      string // subtest name; defaults to the query, set it when the query is awkward in a -run pattern
	query     string
	want      []string
	maxRank   int
	notWant   []string
	knownMiss string // why the case fails today; empty for cases that must hit
}

var qualityCases = []qualityCase{
	{category: "Teilwort", query: "syntax", want: []string{"Markdown-Syntax"},
		knownMiss: "part of a hyphenated title token (markdown-syntax), #1826"},
	{category: "Teilwort", query: "suche", want: []string{"Wikisuche verwenden"},
		knownMiss: "no substring match inside a compound (Wikisuche)"},
	{category: "Teilwort", query: "08-26", want: []string{"Logbuch August"},
		knownMiss: "suffix of a date token (2026-08-26), #1472"},
	{category: "Teilwort", query: "beleuchtung", want: []string{"Störungsmeldung Straßenbeleuchtung"},
		knownMiss: "no substring match inside a compound (Straßenbeleuchtung)"},
	{category: "Teilwort", query: "ausweis", want: []string{"Onboarding neuer Mitarbeiter"},
		knownMiss: "no substring match inside a compound (Mitarbeiterausweis)"},
	{category: "Teilwort", query: "nginx", want: []string{"Kubernetes Deployment"},
		knownMiss: "part of a hyphenated token (ingress-nginx), #1826"},
	{category: "Teilwort", query: "daily", want: []string{"Backup und Wiederherstellung"},
		knownMiss: "part of an underscore token (backup_daily.sh), #1826"},
	{category: "Teilwort", query: "fuzzy", want: []string{"API-Referenz"},
		knownMiss: "no substring match inside a camelCase identifier (buildFuzzyQuery), #1826"},
	{category: "Teilwort", query: "postgresql", want: []string{"Datenbankserver"}},
	{category: "Teilwort", query: "pi", want: []string{"Raspberry Pi einrichten"}},

	{category: "Präfix", query: "markd", want: []string{"Markdown-Syntax", "Markdown Syntax Cheatsheet"}},
	{category: "Präfix", query: "install", want: []string{"Installation unter Windows"}},

	// The Flexion rows pass through prefix matching ("query"*), not stemming:
	// each query is a prefix of the inflected word in the page.
	{category: "Flexion", query: "server", want: []string{"Datenbankserver"}},
	{category: "Flexion", query: "urlaubsantrag", want: []string{"Urlaubsantrag"}},
	{category: "Flexion", query: "mitarbeiterin", want: []string{"Onboarding neuer Mitarbeiter"}},
	{category: "Flexion", query: "installieren", want: []string{"Installation unter Windows", "Raspberry Pi einrichten"}},
	{category: "Flexion", query: "searching", want: []string{"Meeting Notes Q3"}},

	{category: "Umlaut", query: "strasse", want: []string{"Störungsmeldung Straßenbeleuchtung"},
		knownMiss: "no ß→ss folding"},
	{category: "Umlaut", query: "stoerung", want: []string{"Störungsmeldung Straßenbeleuchtung"},
		knownMiss: "no ö→oe folding"},
	{category: "Umlaut", query: "storung", want: []string{"Störungsmeldung Straßenbeleuchtung"}},
	{category: "Umlaut", query: "muller", want: []string{"Onboarding neuer Mitarbeiter"}},
	{category: "Umlaut", query: "Müller", want: []string{"Onboarding neuer Mitarbeiter"}},

	{category: "Tippfehler", query: "markdwon", want: []string{"Markdown-Syntax", "Markdown Syntax Cheatsheet"},
		knownMiss: "no typo tolerance"},
	{category: "Tippfehler", query: "kubernets", want: []string{"Kubernetes Deployment"},
		knownMiss: "no typo tolerance"},

	{category: "Technik", query: "v0.14.2", want: []string{"Logbuch November"}},
	{category: "Technik", query: "0.14", want: []string{"Logbuch November", "Release-Prozess"}},
	{category: "Technik", query: "192.168.1.42", want: []string{"Raspberry Pi einrichten"}},
	{category: "Technik", query: "8080", want: []string{"Raspberry Pi einrichten"}},
	{category: "Technik", query: "C++", want: []string{"C++ und C# Snippets"}},
	{category: "Technik", query: "C#", want: []string{"C++ und C# Snippets"}},
	{category: "Technik", query: "user_id", want: []string{"API-Referenz"}},
	{category: "Technik", name: "api path", query: "/api/search", want: []string{"API-Referenz"}},
	{category: "Technik", query: "config.yaml", want: []string{"Installation unter Windows"}},
	{category: "Technik", query: "leafwiki.service", want: []string{"Raspberry Pi einrichten"}},
	{category: "Technik", query: "std::vector", want: []string{"C++ und C# Snippets"}},
	{category: "Technik", query: "CHANGELOG.md", want: []string{"Release-Prozess"}},
	{category: "Technik", query: "e-mail", want: []string{"Meeting Notes Q3", "FAQ"}},
	{category: "Technik", query: "email", want: []string{"Meeting Notes Q3", "FAQ"},
		knownMiss: "e-mail is one token, the unhyphenated spelling does not match, #1826"},

	{category: "Phrase", query: `"Festplatte voll"`, want: []string{"Logbuch August"},
		knownMiss: `the index keeps the trailing period ("voll."), so the phrase has no exact token`},
	{category: "Phrase", query: `"Helm charts"`, want: []string{"Kubernetes Deployment"},
		knownMiss: `the index keeps the trailing period ("charts."), so the phrase has no exact token`},
	{category: "Phrase", query: `"Markdown Syntax"`, want: []string{"Markdown Syntax Cheatsheet"}},
	{category: "Phrase", query: `"ingress-nginx controller"`, want: []string{"Kubernetes Deployment"}},
	{category: "Phrase", query: "Festplatte voll", want: []string{"Logbuch August"}},
	{category: "Phrase", query: "can't log in", want: []string{"FAQ"}},

	{category: "Operator", query: "title:backup", want: []string{"Backup und Wiederherstellung"},
		notWant: []string{"Logbuch August"}},
	{category: "Operator", query: "headings:suchoperatoren", want: []string{"Wikisuche verwenden"}},
	{category: "Operator", query: "markdown NOT cheatsheet", want: []string{"Markdown-Syntax"},
		notWant: []string{"Markdown Syntax Cheatsheet"}},
	{category: "Operator", query: "windows OR raspberry", want: []string{"Installation unter Windows", "Raspberry Pi einrichten"}},
	{category: "Operator", query: "markdown -cheatsheet", want: []string{"Markdown-Syntax"},
		notWant:   []string{"Markdown Syntax Cheatsheet"},
		knownMiss: "-term exclusion is not supported, -cheatsheet is searched as a literal token"},
	{category: "Operator", query: "tag:hr", want: []string{"Urlaubsantrag", "Onboarding neuer Mitarbeiter"},
		knownMiss: "tags live in the frontmatter, which the index strips; no tag: filter in the query"},
	{category: "Operator", query: "#hr", want: []string{"Urlaubsantrag", "Onboarding neuer Mitarbeiter"},
		knownMiss: "tags live in the frontmatter, which the index strips"},

	{category: "Struktur", query: "Protokolle", want: []string{"Protokolle 2026"}},
	{category: "Struktur", query: "logbuch betrieb", want: []string{"Logbuch August", "Logbuch November"},
		knownMiss: "the path column is not indexed, so a parent section name does not match"},

	{category: "Ranking", query: "markdown", want: []string{"Markdown-Syntax", "Markdown Syntax Cheatsheet"}},
	{category: "Ranking", query: "backup", want: []string{"Backup und Wiederherstellung"}},
	// "Pipeline" matches pi only as a prefix and has the shorter title, so
	// bm25 alone ranks it first.
	{category: "Ranking", query: "pi", want: []string{"Raspberry Pi einrichten"}},
	{category: "Ranking", query: "logbuch", want: []string{"Logbuch August", "Logbuch November"}, maxRank: 2},

	{category: "Sonstiges", query: "日本語", want: []string{"日本語ページ"}},
	{category: "Sonstiges", query: "東京", want: []string{"日本語ページ"}},
	{category: "Sonstiges", query: "🚀", want: []string{"Emoji Statusseite"},
		knownMiss: "emoji are no word characters for the tokenizer or the query sanitizer"},
	// FTS5's only boolean operators are AND, OR and NOT: German users typing
	// UND get a word search, so every page containing "und" is an equally fine hit.
	{category: "Sonstiges", query: "UND", want: []string{
		"Backup und Wiederherstellung", "C++ und C# Snippets", "Betrieb", "Wikisuche verwenden",
		"Installation unter Windows", "Onboarding neuer Mitarbeiter", "API-Referenz",
	}},
	{category: "Sonstiges", query: "...", want: nil},
}

// newQualityIndex indexes qualityCorpus into a fresh SQLiteIndex the way the
// page-save side effect does: raw file content with frontmatter, slug path,
// path + ".md" as file path and an opaque page ID.
func newQualityIndex(t *testing.T) *SQLiteIndex {
	t.Helper()
	index, err := NewSQLiteIndex(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create SQLiteIndex: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(index.Close, t) })

	inputs := make([]IndexPageInput, 0, len(qualityCorpus))
	for i, p := range qualityCorpus {
		// An opaque ID like the real shortids: the pageID column is indexed,
		// so a slug-like ID would let queries match the ID instead of the page.
		id := fmt.Sprintf("qc%02d", i)
		raw := fmt.Sprintf("---\nleafwiki_id: %s\nleafwiki_title: %q\n", id, p.title)
		if len(p.tags) > 0 {
			raw += "tags: [" + strings.Join(p.tags, ", ") + "]\n"
		}
		raw += "---\n" + p.body
		inputs = append(inputs, IndexPageInput{
			Path: p.path, FilePath: p.path + ".md", PageID: id, Title: p.title, Kind: p.kind, Raw: raw,
		})
	}
	failures, err := index.IndexPages(inputs)
	if err != nil {
		t.Fatalf("IndexPages failed: %v", err)
	}
	if len(failures) > 0 {
		t.Fatalf("IndexPages skipped %d pages, first: %s: %v", len(failures), failures[0].PageID, failures[0].Err)
	}
	return index
}

// searchTitles returns the plain titles of all results for query, best first.
func searchTitles(index *SQLiteIndex, query string) ([]string, error) {
	result, err := index.Search(context.Background(), query, nil, 0, len(qualityCorpus))
	if err != nil {
		return nil, err
	}
	titles := make([]string, len(result.Items))
	for i, item := range result.Items {
		plain := strings.NewReplacer("<b>", "", "</b>", "").Replace(item.Title)
		titles[i] = html.UnescapeString(plain)
	}
	return titles, nil
}

// evaluate reports whether got satisfies tc and, if not, why.
func (tc qualityCase) evaluate(got []string) (bool, string) {
	if len(tc.want) == 0 {
		if len(got) > 0 {
			return false, fmt.Sprintf("want no result, got %q", got)
		}
		return true, ""
	}
	for _, title := range tc.notWant {
		if rank := indexOf(got, title) + 1; rank > 0 {
			return false, fmt.Sprintf("%q must not be in the result, found at rank %d", title, rank)
		}
	}
	maxRank := tc.maxRank
	if maxRank == 0 {
		maxRank = 1
	}
	best := 0
	for _, title := range tc.want {
		if rank := indexOf(got, title) + 1; rank > 0 && (best == 0 || rank < best) {
			best = rank
		}
	}
	switch {
	case best == 0:
		return false, fmt.Sprintf("none of %q found", tc.want)
	case best > maxRank:
		return false, fmt.Sprintf("best of %q at rank %d, want <= %d", tc.want, best, maxRank)
	}
	return true, ""
}

func indexOf(titles []string, title string) int {
	for i, got := range titles {
		if got == title {
			return i
		}
	}
	return -1
}

func top(titles []string, n int) []string {
	if len(titles) > n {
		return titles[:n]
	}
	return titles
}

func TestSearchQuality_UserQueries_HitOrAreKnownMisses(t *testing.T) {
	index := newQualityIndex(t)

	type tally struct{ hits, total int }
	perCategory := map[string]*tally{}
	var categories []string

	for _, tc := range qualityCases {
		name := tc.name
		if name == "" {
			name = tc.query
		}
		t.Run(tc.category+"/"+name, func(t *testing.T) {
			c, ok := perCategory[tc.category]
			if !ok {
				c = &tally{}
				perCategory[tc.category] = c
				categories = append(categories, tc.category)
			}
			c.total++

			got, err := searchTitles(index, tc.query)
			if err != nil {
				t.Fatalf("Search(%q) failed: %v", tc.query, err)
			}
			hit, why := tc.evaluate(got)
			if hit {
				c.hits++
			}

			switch {
			case !hit && tc.knownMiss == "":
				t.Errorf("search regression for %q: %s; top results %q", tc.query, why, top(got, 5))
			case hit && tc.knownMiss != "":
				t.Errorf("%q now hits (top results %q): remove its knownMiss mark (%q) so the improvement stays locked in",
					tc.query, top(got, 5), tc.knownMiss)
			}
		})
	}

	var hits, total int
	var summary strings.Builder
	summary.WriteString("search quality hit rate:\n")
	for _, name := range categories {
		c := perCategory[name]
		hits += c.hits
		total += c.total
		fmt.Fprintf(&summary, "  %-10s %2d/%-2d %5.1f%%\n", name, c.hits, c.total, percent(c.hits, c.total))
	}
	fmt.Fprintf(&summary, "  %-10s %2d/%-2d %5.1f%%", "overall", hits, total, percent(hits, total))
	t.Log(summary.String())
}

func percent(hits, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(hits) / float64(total)
}

func TestSearchQuality_TitleMatch_RanksAboveBodyOnlyMatch(t *testing.T) {
	index := newQualityIndex(t)

	tests := []struct {
		name     string
		query    string
		titleHit string
		bodyOnly string
	}{
		{name: "backup page above the log that mentions a backup job", query: "backup",
			titleHit: "Backup und Wiederherstellung", bodyOnly: "Logbuch August"},
		{name: "markdown pages above the glossary that links to one", query: "markdown",
			titleHit: "Markdown Syntax Cheatsheet", bodyOnly: "Glossar"},
		{name: "installation page above the page that says installieren", query: "install",
			titleHit: "Installation unter Windows", bodyOnly: "Raspberry Pi einrichten"},
		{name: "protocol section above its parent section", query: "protokolle",
			titleHit: "Protokolle 2026", bodyOnly: "Betrieb"},
		{name: "onboarding page above the meeting notes that mention onboarding", query: "onboarding",
			titleHit: "Onboarding neuer Mitarbeiter", bodyOnly: "Meeting Notes Q3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := searchTitles(index, tc.query)
			if err != nil {
				t.Fatalf("Search(%q) failed: %v", tc.query, err)
			}
			titleRank := indexOf(got, tc.titleHit)
			if titleRank < 0 {
				t.Fatalf("Search(%q): title match %q not found in %q", tc.query, tc.titleHit, got)
			}
			bodyRank := indexOf(got, tc.bodyOnly)
			if bodyRank < 0 {
				t.Fatalf("Search(%q): body-only match %q not found in %q", tc.query, tc.bodyOnly, got)
			}
			if bodyRank < titleRank {
				t.Errorf("Search(%q): body-only match %q at rank %d above title match %q at rank %d",
					tc.query, tc.bodyOnly, bodyRank+1, tc.titleHit, titleRank+1)
			}
		})
	}
}
