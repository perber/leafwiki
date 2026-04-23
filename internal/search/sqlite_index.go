package search

import (
	"bytes"
	"database/sql"
	"log"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/russross/blackfriday/v2"
	_ "modernc.org/sqlite" // Import SQLite driver
)

var sanitize = bluemonday.StrictPolicy()

var (
	searchShoutoutOpenPattern  = regexp.MustCompile(`^(?P<indent> {0,3}):::\s*(?P<type>[A-Za-z][\w-]*)\s*$`)
	searchShoutoutClosePattern = regexp.MustCompile(`^(?P<indent> {0,3}):::\s*$`)
	searchFencePattern         = regexp.MustCompile(`^(?P<indent> {0,3})(?P<marker>` + "`{3,}|~{3,}" + `)(?P<rest>.*)$`)
)

type SQLiteIndex struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
}

func searchIndexDatabasePath(storageDir string, filename string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalizedStorageDir, filename)
}

func extractHeadings(markdown string) string {
	node := blackfriday.New(blackfriday.WithExtensions(
		blackfriday.CommonExtensions | blackfriday.AutoHeadingIDs,
	)).Parse([]byte(markdown))

	var buf bytes.Buffer

	node.Walk(func(n *blackfriday.Node, entering bool) blackfriday.WalkStatus {
		if !entering || n.Type != blackfriday.Heading {
			return blackfriday.GoToNext
		}

		var headingText bytes.Buffer

		var walk func(c *blackfriday.Node)
		walk = func(c *blackfriday.Node) {
			for ; c != nil; c = c.Next {
				if c.Literal != nil {
					headingText.Write(c.Literal)
					headingText.WriteByte(' ')
				}
				if c.FirstChild != nil {
					walk(c.FirstChild)
				}
			}
		}
		walk(n.FirstChild)

		text := strings.TrimSpace(headingText.String())
		if text != "" {
			buf.WriteString(text)
			buf.WriteByte('\n')
		}

		return blackfriday.GoToNext
	})

	return sanitize.Sanitize(buf.String())
}

type fenceState struct {
	markerChar   byte
	markerLength int
}

func getFenceState(line string, currentFence *fenceState) *fenceState {
	match := searchFencePattern.FindStringSubmatch(line)
	if match == nil {
		return currentFence
	}

	marker := match[searchFencePattern.SubexpIndex("marker")]
	if marker == "" {
		return currentFence
	}

	if currentFence == nil {
		return &fenceState{
			markerChar:   marker[0],
			markerLength: len(marker),
		}
	}

	if marker[0] == currentFence.markerChar && len(marker) >= currentFence.markerLength {
		return nil
	}

	return currentFence
}

func normalizeSearchMarkdownShoutouts(content string) string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	output := make([]string, 0, len(lines))
	var outerFence *fenceState

	for _, line := range lines {
		if outerFence != nil {
			output = append(output, line)
			outerFence = getFenceState(line, outerFence)
			continue
		}

		if match := searchShoutoutOpenPattern.FindStringSubmatch(line); match != nil {
			output = append(output, match[searchShoutoutOpenPattern.SubexpIndex("type")])
			continue
		}

		if searchShoutoutClosePattern.MatchString(line) {
			continue
		}

		output = append(output, line)
		outerFence = getFenceState(line, outerFence)
	}

	return strings.Join(output, "\n")
}

func buildFuzzyQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}

	// if the query contains special FTS5 syntax, return as is
	if strings.ContainsAny(q, "\"*():") ||
		strings.Contains(strings.ToUpper(q), " OR ") ||
		strings.Contains(strings.ToUpper(q), " AND ") {
		return q
	}

	if strings.ContainsAny(q, "-_+#/.") {
		return `"` + q + `"`
	}

	// Append wildcard to each term
	terms := strings.Fields(q)
	for i, t := range terms {
		// Skip if already has wildcard
		if strings.Contains(t, "*") {
			continue
		}
		terms[i] = t + "*"
	}

	return strings.Join(terms, " ")
}

func NewSQLiteIndex(storageDir string) (*SQLiteIndex, error) {
	s := &SQLiteIndex{
		storageDir: storageDir,
		filename:   "search.db",
	}

	// Ensure the schema is created
	// Drops and recreates the pages table
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SQLiteIndex) withDB(fn func(db *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		db, err := sql.Open("sqlite", searchIndexDatabasePath(s.storageDir, s.filename))
		if err != nil {
			return err
		}
		s.db = db
	}

	return fn(s.db)
}

func (s *SQLiteIndex) ensureSchema() error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`
			DROP TABLE IF EXISTS pages;
			CREATE VIRTUAL TABLE IF NOT EXISTS pages USING fts5(
				path UNINDEXED,
				filepath UNINDEXED,
				pageID,
				kind UNINDEXED,
				title,
				headings,
				content,
				tokenize = "unicode61 tokenchars '-_/+#.'"
			);
        `)
		return err
	})
}

func (s *SQLiteIndex) Clear() error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM pages`)
		return err
	})
}

func (s *SQLiteIndex) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

func (s *SQLiteIndex) IndexPage(path string, filePath string, pageID string, title string, kind tree.NodeKind, raw string) error {
	_, content, _, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		return err
	}

	content = normalizeSearchMarkdownShoutouts(content)

	// Headings extracted from the Markdown
	headings := extractHeadings(content)

	// Body as plain text (existing logic)
	html := blackfriday.Run([]byte(content))
	sanitizedBody := sanitize.Sanitize(string(html))

	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID)
		if err != nil {
			return err
		}

		_, err = db.Exec(`
		INSERT INTO pages (path, filepath, pageID, kind, title, headings, content)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`, path, filePath, pageID, string(kind), title, headings, sanitizedBody)

		return err
	})
}

func (s *SQLiteIndex) RemovePage(pageID string) error {
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID)
		return err
	})
}

func (s *SQLiteIndex) RemovePageByFilePath(filePath string) (int64, error) {
	var rows int64
	err := s.withDB(func(db *sql.DB) error {
		res, err := db.Exec(`DELETE FROM pages WHERE filepath = ?`, filePath)
		if err != nil {
			return err
		}
		r, err := res.RowsAffected()
		if err != nil {
			return err
		}
		rows = r
		return nil
	})
	return rows, err
}

func (s *SQLiteIndex) Search(query string, offset, limit int) (*SearchResult, error) {

	if strings.TrimSpace(query) == "" {
		return &SearchResult{
			Count:  0,
			Items:  []SearchResultItem{},
			Offset: offset,
			Limit:  limit,
		}, nil
	}

	sr := &SearchResult{}
	ftsQuery := buildFuzzyQuery(query)

	err := s.withDB(func(db *sql.DB) error {
		var total int

		countQuery := `SELECT COUNT(*) FROM pages WHERE pages MATCH ?;`
		if err := db.QueryRow(countQuery, ftsQuery).Scan(&total); err != nil {
			return err
		}
		sr.Count = total

		searchQuery := `
		SELECT 
			pageID,
			path,
			kind,
			highlight(pages, 4, '<b>', '</b>') AS highlighted_title,
			snippet(pages, 6, '<b>', '</b>', '...', 16) AS excerpt,
			bm25(pages,
				0.0,  -- path
				0.0,  -- filepath
				0.0,  -- pageID
				0.0,  -- kind
				20.0, -- title
				5.0,   -- headings
				1.0    -- content
			) AS bm25_score
		FROM pages
		WHERE pages MATCH ?
		ORDER BY bm25_score ASC
		LIMIT ? OFFSET ?;
	`

		rows, err := db.Query(searchQuery, ftsQuery, limit, offset)
		if err != nil {
			return err
		}
		defer func() {
			if err := rows.Close(); err != nil {
				slog.Default().Error("could not close rows", "error", err)
			}
		}()

		var results []SearchResultItem
		for rows.Next() {
			var r SearchResultItem
			var bm25Score float64

			if err := rows.Scan(&r.PageID, &r.Path, &r.Kind, &r.Title, &r.Excerpt, &bm25Score); err != nil {
				return err
			}

			// Convert bm25 score to a rank (lower score = higher rank)
			if bm25Score < 0 {
				bm25Score = 0
			}
			r.Rank = 1.0 / (1.0 + bm25Score)

			log.Printf("pageID=%s title=%q bm25=%f rank=%f", r.PageID, r.Title, bm25Score, r.Rank)

			results = append(results, r)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		sr.Items = results
		sr.Offset = offset
		sr.Limit = limit
		return nil
	})

	return sr, err
}
