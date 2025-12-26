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

	// Ensure the schema is created
	if err := s.ensureSchema(); err != nil {
		return nil, err
	}

	// Delete all existing entries
	// This is a cleanup step to ensure the table is empty before indexing new data
	return s, s.Clear()
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
            CREATE VIRTUAL TABLE IF NOT EXISTS pages USING fts5(
                path UNINDEXED,
                filepath UNINDEXED,
                pageID,
                title,
                content
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

func (s *SQLiteIndex) GetDB() *sql.DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db
}

func (s *SQLiteIndex) IndexPage(path string, filePath string, pageID string, title string, content string) error {
	plaintext := string(blackfriday.Run([]byte(content)))
	sanitized := bluemonday.StrictPolicy().Sanitize(plaintext)

	return s.withDB(func(db *sql.DB) error {
		if _, err := db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID); err != nil {
			return err
		}

		_, err := db.Exec(`
            INSERT INTO pages (path, filepath, pageID, title, content)
            VALUES (?, ?, ?, ?, ?);
        `, path, filePath, pageID, title, sanitized)

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
	sr := &SearchResult{}

	err := s.withDB(func(db *sql.DB) error {
		// 1. Count total matches
		var total int
		countQuery := `SELECT COUNT(*) FROM pages WHERE pages MATCH ?;`
		if err := db.QueryRow(countQuery, query).Scan(&total); err != nil {
			return err
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

		rows, err := db.Query(searchQuery, query, limit, offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		var results []SearchResultItem
		for rows.Next() {
			var r SearchResultItem
			if err := rows.Scan(&r.PageID, &r.Path, &r.Title, &r.Excerpt, &r.Rank); err != nil {
				return err
			}
			results = append(results, r)
		}

		if results == nil {
			results = []SearchResultItem{}
		}

		// Boost, wenn Query im Titel vorkommt
		for i := range results {
			if strings.Contains(strings.ToLower(results[i].Title), strings.ToLower(query)) {
				results[i].Rank += 1000
			}
		}

		// Sort results by rank in descending order
		for i := 0; i < len(results)-1; i++ {
			for j := i + 1; j < len(results); j++ {
				if results[i].Rank < results[j].Rank {
					results[i], results[j] = results[j], results[i]
				}
			}
		}

		sr.Items = results
		sr.Limit = limit
		sr.Offset = offset

		return nil
	})

	return sr, err
}
