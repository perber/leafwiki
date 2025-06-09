package search

import (
	"database/sql"
	"path"
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	_ "modernc.org/sqlite" // Import SQLite driver
)

type SQLiteIndex struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
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

	plaintext := string(blackfriday.Run([]byte(content)))
	sanitized := bluemonday.StrictPolicy().Sanitize(plaintext)

	_, err = s.db.Exec(`
		INSERT INTO pages (path, filepath, pageID, title, content)
		VALUES (?, ?, ?, ?, ?);
	`, path, filePath, pageID, title, sanitized)

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

	// 1. Count total matches
	var total int
	countQuery := `SELECT COUNT(*) FROM pages WHERE pages MATCH ?;`
	if err := s.db.QueryRow(countQuery, query).Scan(&total); err != nil {
		return nil, err
	}

	sr.Count = total

	searchQuery := `
		SELECT pageID, 
			path, 
			highlight(pages, 3, '<b>', '</b>') AS highlighted_title,
			snippet(pages, 4, '<b>', '</b>', '...', 16) AS excerpt,
			bm25(pages, 10.0, 1.0) AS rank
		FROM pages
		WHERE pages MATCH ?
		ORDER BY rank ASC
		LIMIT ? OFFSET ?;
	`

	rows, err := s.db.Query(searchQuery, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResultItem
	for rows.Next() {
		var r SearchResultItem
		if err := rows.Scan(&r.PageID, &r.Path, &r.Title, &r.Excerpt, &r.Rank); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	if results == nil {
		results = []SearchResultItem{}
	}

	// Order the results by rank
	// When the query is matching the title it should be ranked higher
	for i := range results {
		// Check if the query is part of the title
		if strings.Contains(strings.ToLower(results[i].Title), strings.ToLower(query)) {
			results[i].Rank += 1000 // Boost rank for titles that match the query
		}
	}

	// Sort results by rank in descending order
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Rank < results[j].Rank {
				results[i], results[j] = results[j], results[i] // Swap
			}
		}
	}

	sr.Items = results
	sr.Limit = limit
	sr.Offset = offset

	return sr, err
}
