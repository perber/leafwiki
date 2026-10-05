package links

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	_ "modernc.org/sqlite" // Import SQLite driver
)

type LinksStore struct {
	mu         sync.RWMutex
	storageDir string
	filename   string
	db         *sql.DB
}

const (
	maxOutgoingLinksQueryArgs = 900
	logCloseRowsFailed        = "could not close rows"
	logCloseStatementFailed   = "could not close statement"
)

type PageLinkUpdate struct {
	FromPageID string
	FromTitle  string
	ToPath     string
	Targets    []TargetLink
}

type BrokenLink struct {
	FromPageID string `json:"from_page_id"`
	FromTitle  string `json:"from_title"`
	FromPath   string `json:"from_path"`
	ToPath     string `json:"to_path"`
}

func linksDatabasePath(storageDir string, filename string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalizedStorageDir, filename)
}

func NewLinksStore(storageDir string) (*LinksStore, error) {
	s := &LinksStore{
		storageDir: storageDir,
		filename:   "links.db",
	}

	// ensureSchema calls Connect() itself (idempotently), so it alone is
	// enough to bring the store to a working state.
	err := sqliteutil.RetryOnCorruption(linksDatabasePath(s.storageDir, s.filename), func() error {
		if err := s.ensureSchema(); err != nil {
			if s.db != nil {
				_ = s.db.Close()
				s.db = nil
			}
			return err
		}
		return nil
	})
	if err != nil {
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
	db, err := sql.Open("sqlite", linksDatabasePath(s.storageDir, s.filename)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
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
	if err := s.dropLegacySchema(); err != nil {
		return err
	}
	_, err = s.db.Exec(`
        CREATE TABLE IF NOT EXISTS links (
            from_page_id TEXT NOT NULL,
            to_page_id   TEXT,
			to_path	  	 TEXT NOT NULL,
            from_title   TEXT,
			state 	     INTEGER NOT NULL DEFAULT 0,
            PRIMARY KEY (from_page_id, to_path)
        );

		CREATE INDEX IF NOT EXISTS idx_links_to_page_id ON links(to_page_id);
		CREATE INDEX IF NOT EXISTS idx_links_to_path    ON links(to_path);
		CREATE INDEX IF NOT EXISTS idx_links_to_path_from_page_id ON links(to_path, from_page_id);
		CREATE INDEX IF NOT EXISTS idx_links_state      ON links(state);
		CREATE INDEX IF NOT EXISTS idx_links_to_path_lower ON links(LOWER(to_path));
	`)
	return err
}

// dropLegacySchema drops a links table from before the 3-state `state` column
// (it had a boolean `broken` column). The links index is derived data and is
// rebuilt from the markdown files on every startup (IndexAllPages), so the
// table can simply be recreated instead of migrated.
func (s *LinksStore) dropLegacySchema() error {
	var legacy int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('links') WHERE name = 'broken'`).Scan(&legacy)
	if err != nil {
		return err
	}
	if legacy == 0 {
		return nil
	}
	slog.Default().Info("dropping legacy links table, it is rebuilt from the markdown files", "table", "links")
	_, err = s.db.Exec(`DROP TABLE links; DROP INDEX IF EXISTS idx_links_broken;`)
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
		    state     = 1
		WHERE to_page_id = ?
		  AND state = 0
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
		    state     = 1
		WHERE to_path = ?
		  AND state   = 0
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
		    state     = 1
		WHERE state = 0
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

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO links(from_page_id, to_page_id, to_path, from_title, state) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		rbErr := tx.Rollback()
		base := fmt.Errorf("failed to prepare insert statement for links from page %s", fromPageID)

		if rbErr != nil {
			return errors.Join(base, err, rbErr)
		}
		return errors.Join(base, err)
	}
	defer func() {
		if err := stmt.Close(); err != nil {
			slog.Default().Error(logCloseStatementFailed, "error", err)
		}
	}()

	for _, link := range toLinks {
		_, err := stmt.Exec(fromPageID, link.TargetPageID, link.TargetPagePath, fromTitle, int(link.State))
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

func (s *LinksStore) ReplaceLinksAndHeal(updates []PageLinkUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	if err := s.replaceLinksAndHealTx(tx, updates); err != nil {
		rbErr := tx.Rollback()
		if rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}

	return tx.Commit()
}

func (s *LinksStore) replaceLinksAndHealTx(tx *sql.Tx, updates []PageLinkUpdate) error {
	deleteStmt, err := tx.Prepare(`DELETE FROM links WHERE from_page_id = ?`)
	if err != nil {
		return fmt.Errorf("failed to prepare delete statement for batched link update: %w", err)
	}
	defer func() {
		if err := deleteStmt.Close(); err != nil {
			slog.Default().Error(logCloseStatementFailed, "error", err)
		}
	}()

	insertStmt, err := tx.Prepare(`INSERT OR REPLACE INTO links(from_page_id, to_page_id, to_path, from_title, state) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement for batched link update: %w", err)
	}
	defer func() {
		if err := insertStmt.Close(); err != nil {
			slog.Default().Error(logCloseStatementFailed, "error", err)
		}
	}()

	healStmt, err := tx.Prepare(`
		UPDATE links
		SET to_page_id = ?, state = 0
		WHERE to_path = ? AND state = 1
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare heal statement for batched link update: %w", err)
	}
	defer func() {
		if err := healStmt.Close(); err != nil {
			slog.Default().Error(logCloseStatementFailed, "error", err)
		}
	}()

	for _, update := range updates {
		if _, err := deleteStmt.Exec(update.FromPageID); err != nil {
			return fmt.Errorf("failed to clear existing links for page %s: %w", update.FromPageID, err)
		}

		for _, link := range update.Targets {
			if _, err := insertStmt.Exec(update.FromPageID, link.TargetPageID, link.TargetPagePath, update.FromTitle, int(link.State)); err != nil {
				return fmt.Errorf("failed to insert link from %s to %s: %w", update.FromPageID, link.TargetPageID, err)
			}
		}
	}

	for _, update := range updates {
		if _, err := healStmt.Exec(update.FromPageID, update.ToPath); err != nil {
			return fmt.Errorf("failed to heal links for path %s: %w", update.ToPath, err)
		}
	}

	return nil
}

func (s *LinksStore) GetBacklinksForPage(pageID string) ([]Backlink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT from_page_id, to_page_id, from_title FROM links WHERE to_page_id = ? AND state = 0`, pageID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

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
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
        SELECT from_page_id, to_page_id, to_path, from_title, state
        FROM links
        WHERE from_page_id = ?
    `, pageID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	var outgoings []Outgoing
	for rows.Next() {
		var o Outgoing
		var toPageID sql.NullString
		var stateInt int

		if err := rows.Scan(&o.FromPageID, &toPageID, &o.ToPath, &o.FromTitle, &stateInt); err != nil {
			return nil, err
		}

		if toPageID.Valid {
			o.ToPageID = toPageID.String
		} else {
			o.ToPageID = ""
		}

		o.State = LinkState(stateInt)
		outgoings = append(outgoings, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return outgoings, nil
}

func (s *LinksStore) GetOutgoingLinksForPages(pageIDs []string) (map[string][]Outgoing, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(pageIDs) == 0 {
		return map[string][]Outgoing{}, nil
	}

	outgoingByPageID := make(map[string][]Outgoing, len(pageIDs))
	for start := 0; start < len(pageIDs); start += maxOutgoingLinksQueryArgs {
		end := start + maxOutgoingLinksQueryArgs
		if end > len(pageIDs) {
			end = len(pageIDs)
		}

		if err := s.appendOutgoingLinksForPageBatch(outgoingByPageID, pageIDs[start:end]); err != nil {
			return nil, err
		}
	}

	return outgoingByPageID, nil
}

func (s *LinksStore) appendOutgoingLinksForPageBatch(outgoingByPageID map[string][]Outgoing, pageIDs []string) error {
	placeholders := strings.TrimRight(strings.Repeat("?,", len(pageIDs)), ",")
	args := make([]any, 0, len(pageIDs))
	for _, pageID := range pageIDs {
		args = append(args, pageID)
	}

	rows, err := s.db.Query(`
        SELECT from_page_id, to_page_id, to_path, from_title, state
        FROM links
        WHERE from_page_id IN (`+placeholders+`)
        ORDER BY from_page_id
    `, args...)
	if err != nil {
		return err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	for rows.Next() {
		var outgoing Outgoing
		var toPageID sql.NullString
		var stateInt int

		if err := rows.Scan(&outgoing.FromPageID, &toPageID, &outgoing.ToPath, &outgoing.FromTitle, &stateInt); err != nil {
			return err
		}

		if toPageID.Valid {
			outgoing.ToPageID = toPageID.String
		}
		outgoing.State = LinkState(stateInt)
		outgoingByPageID[outgoing.FromPageID] = append(outgoingByPageID[outgoing.FromPageID], outgoing)
	}

	return rows.Err()
}

func (s *LinksStore) GetRefactorMatchesForPrefix(oldPrefix string) ([]RefactorLinkMatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT from_page_id, from_title, to_path, state
		FROM links
		WHERE to_path = ? OR to_path LIKE ?
	`, oldPrefix, oldPrefix+"/%")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	var matches []RefactorLinkMatch
	for rows.Next() {
		var match RefactorLinkMatch
		var stateInt int
		if err := rows.Scan(&match.FromPageID, &match.FromTitle, &match.ToPath, &stateInt); err != nil {
			return nil, err
		}
		match.Broken = LinkState(stateInt) == LinkBroken
		matches = append(matches, match)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

func (s *LinksStore) GetRefactorSourcePageIDsForPrefix(oldPrefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT DISTINCT from_page_id
		FROM links
		WHERE to_path = ? OR to_path LIKE ?
	`, oldPrefix, oldPrefix+"/%")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	var pageIDs []string
	for rows.Next() {
		var pageID string
		if err := rows.Scan(&pageID); err != nil {
			return nil, err
		}
		pageIDs = append(pageIDs, pageID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pageIDs, nil
}

func (s *LinksStore) GetRefactorSourcePageIDsForWikiLinkTitle(title string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT DISTINCT from_page_id
		FROM links
		WHERE LOWER(to_path) = ?
	`, strings.ToLower(wikilinkSentinelPrefix+title))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	var pageIDs []string
	for rows.Next() {
		var pageID string
		if err := rows.Scan(&pageID); err != nil {
			return nil, err
		}
		pageIDs = append(pageIDs, pageID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pageIDs, nil
}

func (s *LinksStore) GetBrokenIncomingForPath(toPath string) ([]Backlink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT from_page_id, to_page_id, from_title
		FROM links
		WHERE to_path = ? AND state = 1
		ORDER BY from_title ASC
	`, toPath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

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
		b.Broken = true
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
		SET to_page_id = ?, state = 0
		WHERE to_path = ? AND state = 1
	`, pageID, toPath)

	return err
}

// sentinelKey builds the lower-cased "wikilink:<title>" key for comparison
// with SQLite's LOWER(to_path). SQLite's built-in LOWER() folds ASCII only, so
// the key must be folded the same way: strings.ToLower would also fold
// non-ASCII capitals (Ä → ä) and then never match the stored value.
func sentinelKey(title string) string {
	key := wikilinkSentinelPrefix + strings.TrimSpace(title)
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, key)
}

// ReconcileWikiLinksForTitle applies the state computed by the caller
// (classifyTitleMatches) to every stored wiki-link record whose to_path is
// "wikilink:<title>", using a case-insensitive match that mirrors
// FindPagesByTitle. All wiki-links to one title share that key, so a single
// UPDATE covers every source page. pageID is only set for LinkResolved.
func (s *LinksStore) ReconcileWikiLinksForTitle(title string, state LinkState, pageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var target any
	if state == LinkResolved {
		target = pageID
	}

	_, err := s.db.Exec(`
		UPDATE links
		SET to_page_id = ?, state = ?
		WHERE LOWER(to_path) = ?
	`, target, int(state), sentinelKey(title))

	return err
}

// GetAmbiguousIncomingForTitle returns the sources of ambiguous [[title]]
// wiki-links (the title matches several pages).
func (s *LinksStore) GetAmbiguousIncomingForTitle(title string) ([]Backlink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT from_page_id, from_title
		FROM links
		WHERE LOWER(to_path) = ? AND state = 2
		ORDER BY from_title ASC
	`, sentinelKey(title))
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			slog.Default().Error(logCloseRowsFailed, "error", err)
		}
	}()

	var backlinks []Backlink
	for rows.Next() {
		var b Backlink
		if err := rows.Scan(&b.FromPageID, &b.FromTitle); err != nil {
			return nil, err
		}
		backlinks = append(backlinks, b)
	}
	return backlinks, rows.Err()
}

func (s *LinksStore) GetBrokenLinks() ([]BrokenLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
        SELECT from_page_id, from_title, to_path
        FROM links
        WHERE state = 1
        ORDER BY to_path, from_title
    `)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var result []BrokenLink

	for rows.Next() {
		var link BrokenLink

		if err := rows.Scan(
			&link.FromPageID,
			&link.FromTitle,
			&link.ToPath,
		); err != nil {
			return nil, err
		}

		result = append(result, link)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (s *LinksStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM links`)
	return err
}

func (s *LinksStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

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
