package search

import (
	"bytes"
	"database/sql"
	"path"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
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

	err := s.Connect()
	if err != nil {
		return nil, err
	}

	// Ensure the schema is created
	if err = s.ensureSchema(); err != nil {
		return nil, err
	}

	// Delete all existing entries
	// This is a cleanup step to ensure the table is empty before indexing new data
	return s, s.Clear()

}

func (s *SQLiteIndex) Connect() error {
	// Database is already open and connected
	if s.db != nil {
		return nil
	}
	// Connect to the database
	db, err := sql.Open("sqlite", path.Join(s.storageDir, s.filename))
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

func (s *SQLiteIndex) ensureSchema() error {
	err := s.Connect()
	if err != nil {
		return err
	}
	// Create the users table if it doesn't exist
	_, err = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS pages USING fts5(
			path UNINDEXED,
			filepath UNINDEXED,
			pageID,
			title,
			headings,
			content
		);
	`)
	return err
}

func (s *SQLiteIndex) Clear() error {
	_, err := s.db.Exec(`DELETE FROM pages`)
	return err
}

func (s *SQLiteIndex) Close() error {
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}

func (s *SQLiteIndex) GetDB() *sql.DB {
	if s.db == nil {
		return nil
	}
	return s.db
}

func (s *SQLiteIndex) IndexPage(path string, filePath string, pageID string, title string, content string) error {
	if s.db == nil {
		return sql.ErrConnDone
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID)
	if err != nil {
		return err
	}

	// Headings extracted from the Markdown
	headings := extractHeadings(content)

	// Body as plain text (existing logic)
	html := blackfriday.Run([]byte(content))
	sanitizedBody := sanitize.Sanitize(string(html))

	_, err = s.db.Exec(`
		INSERT INTO pages (path, filepath, pageID, title, headings, content)
		VALUES (?, ?, ?, ?, ?, ?);
	`, path, filePath, pageID, title, headings, sanitizedBody)

	return err
}

func (s *SQLiteIndex) RemovePage(pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID)
	return err
}

func (s *SQLiteIndex) RemovePageByFilePath(filePath string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM pages WHERE filepath = ?`, filePath)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *SQLiteIndex) Search(query string, offset, limit int) (*SearchResult, error) {
	if s.db == nil {
		return nil, sql.ErrConnDone
	}

	sr := &SearchResult{}

	ftsQuery := buildFuzzyQuery(query)

	// 1. Count total matches
	var total int
	countQuery := `SELECT COUNT(*) FROM pages WHERE pages MATCH ?;`
	if err := s.db.QueryRow(countQuery, ftsQuery).Scan(&total); err != nil {
		return nil, err
	}
	sr.Count = total

	searchQuery := `
		SELECT 
			pageID,
			path,
			highlight(pages, 3, '<b>', '</b>') AS highlighted_title,
			snippet(pages, 5, '<b>', '</b>', '...', 16) AS excerpt,
			bm25(pages, 0.0, 5.0, 3.0, 1.0) AS bm25_score
		FROM pages
		WHERE pages MATCH ?
		ORDER BY bm25_score ASC
		LIMIT ? OFFSET ?;
	`

	rows, err := s.db.Query(searchQuery, ftsQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResultItem
	for rows.Next() {
		var r SearchResultItem
		var bm25Score float64

		if err := rows.Scan(&r.PageID, &r.Path, &r.Title, &r.Excerpt, &bm25Score); err != nil {
			return nil, err
		}
		// Convert bm25 score to a rank (lower score = higher rank)
		if bm25Score < 0 {
			bm25Score = 0
		}
		r.Rank = 1.0 / (1.0 + bm25Score)

		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []SearchResultItem{}
	}

	sr.Items = results
	sr.Limit = limit
	sr.Offset = offset

	return sr, nil
}
