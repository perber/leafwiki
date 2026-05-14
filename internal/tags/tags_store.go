package tags

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

type TagsStore struct {
	mu  sync.Mutex
	db  *sql.DB
}

type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func NewTagsStore(storageDir string) (*TagsStore, error) {
	normalized := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	dbPath := filepath.Join(normalized, "tags.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open tags database: %w", err)
	}

	s := &TagsStore{db: db}
	if err := s.ensureSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *TagsStore) ensureSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS page_tags (
			page_id TEXT NOT NULL,
			tag     TEXT NOT NULL COLLATE NOCASE,
			PRIMARY KEY (page_id, tag)
		);
		CREATE INDEX IF NOT EXISTS idx_page_tags_tag ON page_tags(tag);
	`)
	return err
}

// SetTagsForPage replaces all tags for the given page atomically.
// Tags are stored as-is — normalization (lowercase, dedup, trim) is the caller's responsibility.
// All write paths go through TagsService.SetTagsForPage or ExtractTagsFromContent, which enforce this.
func (s *TagsStore) SetTagsForPage(pageID string, tags []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM page_tags WHERE page_id = ?`, pageID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to clear tags for page %s: %w", pageID, err)
	}

	if len(tags) > 0 {
		stmt, err := tx.Prepare(`INSERT OR IGNORE INTO page_tags(page_id, tag) VALUES (?, ?)`)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to prepare tag insert: %w", err)
		}
		defer stmt.Close()

		for _, tag := range tags {
			if _, err := stmt.Exec(pageID, tag); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("failed to insert tag %q for page %s: %w", tag, pageID, err)
			}
		}
	}

	return tx.Commit()
}

func (s *TagsStore) DeleteTagsForPage(pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM page_tags WHERE page_id = ?`, pageID)
	return err
}

func (s *TagsStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM page_tags`)
	return err
}

// GetAllTags returns tags with page count, optionally filtered by prefix. limit <= 0 means no limit.
func (s *TagsStore) GetAllTags(filter string, limit int) ([]TagCount, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `
		SELECT tag, COUNT(DISTINCT page_id) AS count
		FROM page_tags
		WHERE tag LIKE ? || '%' ESCAPE '\'
		GROUP BY tag
		ORDER BY count DESC, tag ASC
	`
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query, escapeLikePrefix(filter))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []TagCount
	for rows.Next() {
		var tc TagCount
		if err := rows.Scan(&tc.Tag, &tc.Count); err != nil {
			return nil, err
		}
		result = append(result, tc)
	}
	return result, rows.Err()
}

// GetPageIDsByTags returns page IDs that have ALL of the given tags (AND logic).
func (s *TagsStore) GetPageIDsByTags(tags []string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(tags) == 0 {
		return nil, nil
	}

	args := make([]any, 0, len(tags)+1)
	for _, t := range tags {
		args = append(args, t)
	}
	args = append(args, len(tags))

	placeholders := strings.TrimRight(strings.Repeat("?,", len(tags)), ",")
	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT page_id
		FROM page_tags
		WHERE tag IN (%s)
		GROUP BY page_id
		HAVING COUNT(DISTINCT tag) = ?
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pageIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		pageIDs = append(pageIDs, id)
	}
	return pageIDs, rows.Err()
}

// GetTagsForPages returns a map of pageID → tags for the given page IDs.
func (s *TagsStore) GetTagsForPages(pageIDs []string) (map[string][]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(pageIDs) == 0 {
		return map[string][]string{}, nil
	}

	placeholders := strings.TrimRight(strings.Repeat("?,", len(pageIDs)), ",")
	args := make([]any, len(pageIDs))
	for i, id := range pageIDs {
		args[i] = id
	}

	rows, err := s.db.Query(fmt.Sprintf(`
		SELECT page_id, tag FROM page_tags
		WHERE page_id IN (%s)
		ORDER BY page_id, tag ASC
	`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string)
	for rows.Next() {
		var pageID, tag string
		if err := rows.Scan(&pageID, &tag); err != nil {
			return nil, err
		}
		result[pageID] = append(result[pageID], tag)
	}
	return result, rows.Err()
}

// escapeLikePrefix escapes LIKE special characters in a prefix filter so that
// '%', '_', and '\' are treated as literals and not as SQL wildcards.
func escapeLikePrefix(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

func (s *TagsStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return err
		}
		s.db = nil
	}
	return nil
}
