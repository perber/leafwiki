package search

import (
	"bytes"
	"database/sql"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/perber/wiki/internal/core/frontmatter"
	"github.com/russross/blackfriday/v2"
	_ "modernc.org/sqlite" // Import SQLite driver
)

var sanitize = bluemonday.StrictPolicy()

type SQLiteIndex struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
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
		db, err := sql.Open("sqlite", path.Join(s.storageDir, s.filename))
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

func (s *SQLiteIndex) IndexPage(path string, filePath string, pageID string, title string, raw string) error {
	_, content, _, err := frontmatter.ParseFrontmatter(raw)
	if err != nil {
		return err
	}

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
		INSERT INTO pages (path, filepath, pageID, title, headings, content)
		VALUES (?, ?, ?, ?, ?, ?);
	`, path, filePath, pageID, title, headings, sanitizedBody)

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
			highlight(pages, 3, '<b>', '</b>') AS highlighted_title,
			snippet(pages, 5, '<b>', '</b>', '...', 16) AS excerpt,
			bm25(pages,
				0.0,  -- path
				0.0,  -- filepath
				0.0,  -- pageID
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
		defer rows.Close()

		var results []SearchResultItem
		for rows.Next() {
			var r SearchResultItem
			var bm25Score float64

			if err := rows.Scan(&r.PageID, &r.Path, &r.Title, &r.Excerpt, &bm25Score); err != nil {
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
