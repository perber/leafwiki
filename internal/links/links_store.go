package links

import (
	"database/sql"
	"errors"
	"fmt"
	"path"
	"sync"

	_ "modernc.org/sqlite" // Import SQLite driver
)

type LinksStore struct {
	mu         sync.Mutex
	storageDir string
	filename   string
	db         *sql.DB
}

func NewLinksStore(storageDir string) (*LinksStore, error) {
	s := &LinksStore{
		storageDir: storageDir,
		filename:   "links.db",
	}

	err := s.Connect()
	if err != nil {
		return nil, err
	}

	// Ensure the schema is created
	if err = s.ensureSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *LinksStore) Connect() error {
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

func (s *LinksStore) ensureSchema() error {
	err := s.Connect()
	if err != nil {
		return err
	}
	// Create the users table if it doesn't exist
	_, err = s.db.Exec(`
        CREATE TABLE IF NOT EXISTS links (
            from_page_id TEXT NOT NULL,
            to_page_id   TEXT,
			to_path	  	 TEXT NOT NULL,
            from_title   TEXT,
			broken 	     INTEGER NOT NULL DEFAULT 0,
            PRIMARY KEY (from_page_id, to_path)
        );

		CREATE INDEX IF NOT EXISTS idx_links_to_page_id ON links(to_page_id);
		CREATE INDEX IF NOT EXISTS idx_links_to_path    ON links(to_path);
		CREATE INDEX IF NOT EXISTS idx_links_broken     ON links(broken);
	`)
	return err
}

// DeleteOutgoingLinks removes all links originating from the given page.
// This is correct when the page itself is deleted (no source markdown left).
func (s *LinksStore) DeleteOutgoingLinks(fromPageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM links WHERE from_page_id = ?`, fromPageID)
	return err
}

// MarkIncomingLinksBroken marks links pointing to the given page as broken,
// but keeps them (because the source markdown still contains them).
func (s *LinksStore) MarkIncomingLinksBroken(toPageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE links
		SET to_page_id = NULL,
		    broken    = 1
		WHERE to_page_id = ?
		  AND broken = 0
	`, toPageID)

	return err
}

// MarkLinksBrokenForPath marks links that point to an exact path as broken.
// Useful for delete/rename in strict mode.
func (s *LinksStore) MarkLinksBrokenForPath(toPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE links
		SET to_page_id = NULL,
		    broken    = 1
		WHERE to_path = ?
		  AND broken  = 0
	`, toPath)

	return err
}

// MarkLinksBrokenForPrefix marks links whose to_path is under the given prefix as broken.
// Boundary-safe: matches either the prefix itself, or prefix + "/...".
func (s *LinksStore) MarkLinksBrokenForPrefix(oldPrefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE links
		SET to_page_id = NULL,
		    broken    = 1
		WHERE broken = 0
		  AND (
		    to_path = ?
		    OR to_path LIKE ? || '/%'
		  )
	`, oldPrefix, oldPrefix)

	return err
}

func (s *LinksStore) AddLinks(fromPageID string, fromTitle string, toLinks []TargetLink) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	// Clean up existing links to avoid duplicates for the same from_page_id
	_, err = tx.Exec(`DELETE FROM links WHERE from_page_id = ?`, fromPageID)
	if err != nil {
		rbErr := tx.Rollback()
		base := fmt.Errorf("failed to clear existing links for page %s", fromPageID)

		if rbErr != nil {
			return errors.Join(base, err, rbErr)
		}
		return errors.Join(base, err)
	}

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO links(from_page_id, to_page_id, to_path, from_title, broken) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		rbErr := tx.Rollback()
		base := fmt.Errorf("failed to prepare insert statement for links from page %s", fromPageID)

		if rbErr != nil {
			return errors.Join(base, err, rbErr)
		}
		return errors.Join(base, err)
	}
	defer stmt.Close()

	for _, link := range toLinks {
		brokenInt := 0
		if link.Broken {
			brokenInt = 1
		}

		_, err := stmt.Exec(fromPageID, link.TargetPageID, link.TargetPagePath, fromTitle, brokenInt)
		if err != nil {
			rbErr := tx.Rollback()
			base := fmt.Errorf("failed to insert link from %s to %s", fromPageID, link.TargetPageID)

			if rbErr != nil {
				return errors.Join(base, err, rbErr)
			}
			return errors.Join(base, err)
		}
	}

	return tx.Commit()
}

func (s *LinksStore) GetBacklinksForPage(pageID string) ([]Backlink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT from_page_id, to_page_id, from_title FROM links WHERE to_page_id = ? and broken = 0`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backlinks []Backlink
	for rows.Next() {
		var b Backlink
		var toPageID sql.NullString
		if err := rows.Scan(&b.FromPageID, &toPageID, &b.FromTitle); err != nil {
			return nil, err
		}
		if toPageID.Valid {
			b.ToPageID = toPageID.String
		} else {
			b.ToPageID = ""
		}
		backlinks = append(backlinks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return backlinks, nil
}

func (s *LinksStore) GetOutgoingLinksForPage(pageID string) ([]Outgoing, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
        SELECT from_page_id, to_page_id, to_path, from_title, broken
        FROM links
        WHERE from_page_id = ?
    `, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var outgoings []Outgoing
	for rows.Next() {
		var o Outgoing
		var toPageID sql.NullString
		var brokenInt int

		if err := rows.Scan(&o.FromPageID, &toPageID, &o.ToPath, &o.FromTitle, &brokenInt); err != nil {
			return nil, err
		}

		if toPageID.Valid {
			o.ToPageID = toPageID.String
		} else {
			o.ToPageID = ""
		}

		o.Broken = brokenInt != 0
		outgoings = append(outgoings, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return outgoings, nil
}

func (s *LinksStore) GetBrokenIncomingForPath(toPath string) ([]Backlink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT from_page_id, to_page_id, from_title
		FROM links
		WHERE to_path = ? AND broken = 1
		ORDER BY from_title ASC
	`, toPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var backlinks []Backlink
	for rows.Next() {
		var b Backlink
		var toPageID sql.NullString
		if err := rows.Scan(&b.FromPageID, &toPageID, &b.FromTitle); err != nil {
			return nil, err
		}
		if toPageID.Valid {
			b.ToPageID = toPageID.String
		} else {
			b.ToPageID = ""
		}
		backlinks = append(backlinks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return backlinks, nil
}

func (s *LinksStore) HealLinksForPath(toPath string, pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		UPDATE links
		SET to_page_id = ?, broken = 0
		WHERE to_path = ? AND broken = 1
	`, pageID, toPath)

	return err
}

func (s *LinksStore) Clear() error {
	_, err := s.db.Exec(`DELETE FROM links`)
	return err
}

func (s *LinksStore) Close() error {
	if s.db != nil {
		err := s.db.Close()
		if err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}

func (s *LinksStore) GetDB() *sql.DB {
	if s.db == nil {
		return nil
	}
	return s.db
}
