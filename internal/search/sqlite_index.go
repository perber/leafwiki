package search

import (
	"database/sql"
	"path"

	_ "modernc.org/sqlite" // Import SQLite driver
)

type SQLiteIndex struct {
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

func (s *SQLiteIndex) IndexPage(path string, pageID string, title string, content string) error {
	if s.db == nil {
		return sql.ErrConnDone
	}

	_, err := s.db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`
		INSERT INTO pages (path, pageID, title, content)
		VALUES (?, ?, ?, ?);
	`, path, pageID, title, content)

	return err
}
