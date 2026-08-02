package search

import (
	"bytes"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"

	"github.com/perber/wiki/internal/core/excerpt"
	"github.com/perber/wiki/internal/core/markdown"
	"github.com/perber/wiki/internal/core/shared/htmlutil"
	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	_ "modernc.org/sqlite"
)

type SQLiteIndex struct {
	mu         sync.RWMutex
	storageDir string
	filename   string
	db         *sql.DB
}

func searchIndexDatabasePath(storageDir string, filename string) string {
	normalizedStorageDir := filepath.FromSlash(strings.ReplaceAll(storageDir, `\`, `/`))
	return filepath.Join(normalizedStorageDir, filename)
}

var headingParser = goldmark.New()

func extractHeadings(src string) string {
	srcBytes := []byte(src)
	reader := text.NewReader(srcBytes)
	doc := headingParser.Parser().Parse(reader)

	var buf bytes.Buffer

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if _, ok := n.(*ast.Heading); !ok {
			return ast.WalkContinue, nil
		}

		var headingText bytes.Buffer
		_ = ast.Walk(n, func(child ast.Node, childEntering bool) (ast.WalkStatus, error) {
			if !childEntering || child == n {
				return ast.WalkContinue, nil
			}
			if t, ok := child.(*ast.Text); ok {
				headingText.Write(t.Segment.Value(srcBytes))
				headingText.WriteByte(' ')
			}
			return ast.WalkContinue, nil
		})

		headingStr := strings.TrimSpace(headingText.String())
		if headingStr != "" {
			buf.WriteString(headingStr)
			buf.WriteByte('\n')
		}

		return ast.WalkSkipChildren, nil
	})

	return excerpt.PlainTextFromMarkdown(buf.String())
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

	err := sqliteutil.RetryOnCorruption(searchIndexDatabasePath(s.storageDir, s.filename), func() error {
		if err := s.ensureSchema(); err != nil {
			if closeErr := s.Close(); closeErr != nil {
				slog.Default().Warn("failed to close corrupt search database before recovery", "error", closeErr)
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

// connect returns the open *sql.DB, opening it on first use. Safe for
// concurrent callers on both the read and write paths: the common case
// (already open) only takes the cheap RLock; opening the connection is
// the one operation that mutates s.db itself, so it's re-checked under
// the exclusive Lock (standard double-checked locking) rather than
// requiring every caller — including read-only ones — to hold the
// exclusive lock just to be safe against a lazy-init race that almost
// never happens after the first call.
func (s *SQLiteIndex) connect() (*sql.DB, error) {
	s.mu.RLock()
	if s.db != nil {
		db := s.db
		s.mu.RUnlock()
		return db, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		db, err := sql.Open("sqlite", searchIndexDatabasePath(s.storageDir, s.filename)+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
		if err != nil {
			return nil, err
		}
		s.db = db
	}
	return s.db, nil
}

// withDB runs fn under the exclusive lock — use for anything that writes.
func (s *SQLiteIndex) withDB(fn func(db *sql.DB) error) error {
	db, err := s.connect()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return fn(db)
}

// withDBRead runs fn under a shared read lock — use for read-only
// queries (Search, SearchPageIDs, Ping) so concurrent readers don't
// serialize behind each other the way withDB's writers must. Load-tested:
// this is what fixed Search's reader-self-contention (see
// loadtest/k6/search-only.js results), the same fix already applied to
// internal/links/links_store.go for the identical symptom.
func (s *SQLiteIndex) withDBRead(fn func(db *sql.DB) error) error {
	db, err := s.connect()
	if err != nil {
		return err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return fn(db)
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

func (s *SQLiteIndex) Ping() error {
	return s.withDBRead(func(db *sql.DB) error {
		return db.Ping()
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

// IndexPageInput bundles one page's worth of IndexPages input.
type IndexPageInput struct {
	Path     string
	FilePath string
	PageID   string
	Title    string
	Kind     tree.NodeKind
	Raw      string
}

// IndexFailure pairs a failed input with why it failed, returned by
// IndexPages for inputs skipped due to a per-page problem (e.g.
// unparsable frontmatter) rather than a transaction-level failure.
type IndexFailure struct {
	PageID string
	Err    error
}

// IndexPages indexes multiple pages in a single transaction — used for
// subtree operations (recursive delete healing/move) where a naive
// one-IndexPage-call-per-page loop would pay for one commit per page.
// Load-tested: for a 200-page subtree this cut the search side effect's
// share of total Move/Delete cost from ~75-93% to a small fraction (see
// the Delete/Move/Rename load-test results).
//
// Inputs whose frontmatter fails to parse are skipped and reported via
// the returned []IndexFailure rather than aborting the whole batch — the
// rest still commit together. Only a transaction-level failure (e.g. the
// commit itself failing) returns a non-nil error, in which case none of
// the inputs were indexed. This preserves IndexPage's per-page fault
// isolation for the most likely failure mode (bad content) while still
// getting the throughput win for the common case (everything valid).
func (s *SQLiteIndex) IndexPages(inputs []IndexPageInput) ([]IndexFailure, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	type prepared struct {
		path, filePath, pageID, title string
		kind                          tree.NodeKind
		headings, sanitizedBody       string
	}

	var failures []IndexFailure
	prepped := make([]prepared, 0, len(inputs))
	for _, in := range inputs {
		_, content, _, err := markdown.ParseFrontmatter(in.Raw)
		if err != nil {
			failures = append(failures, IndexFailure{PageID: in.PageID, Err: err})
			continue
		}
		content = excerpt.NormalizeMarkdownBody(content)
		prepped = append(prepped, prepared{
			path:          in.Path,
			filePath:      in.FilePath,
			pageID:        in.PageID,
			title:         in.Title,
			kind:          in.Kind,
			headings:      extractHeadings(content),
			sanitizedBody: excerpt.PlainTextForSearch(content),
		})
	}

	if len(prepped) == 0 {
		return failures, nil
	}

	err := s.withDB(func(db *sql.DB) error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer func() {
			_ = tx.Rollback()
		}()

		deleteStmt, err := tx.Prepare(`DELETE FROM pages WHERE pageID = ?`)
		if err != nil {
			return err
		}
		defer func() {
			_ = deleteStmt.Close()
		}()

		insertStmt, err := tx.Prepare(`
			INSERT INTO pages (path, filepath, pageID, kind, title, headings, content)
			VALUES (?, ?, ?, ?, ?, ?, ?);
		`)
		if err != nil {
			return err
		}
		defer func() {
			_ = insertStmt.Close()
		}()

		for _, p := range prepped {
			if _, err := deleteStmt.Exec(p.pageID); err != nil {
				return err
			}
			if _, err := insertStmt.Exec(p.path, p.filePath, p.pageID, string(p.kind), p.title, p.headings, p.sanitizedBody); err != nil {
				return err
			}
		}

		return tx.Commit()
	})

	return failures, err
}

func (s *SQLiteIndex) IndexPage(path string, filePath string, pageID string, title string, kind tree.NodeKind, raw string) error {
	failures, err := s.IndexPages([]IndexPageInput{{Path: path, FilePath: filePath, PageID: pageID, Title: title, Kind: kind, Raw: raw}})
	if err != nil {
		return err
	}
	if len(failures) > 0 {
		return failures[0].Err
	}
	return nil
}

// RemovePages removes multiple pages from the index in a single
// transaction — the batch counterpart to RemovePage, used for subtree
// deletes for the same reason IndexPages exists (one commit instead of
// one per page).
func (s *SQLiteIndex) RemovePages(pageIDs []string) error {
	if len(pageIDs) == 0 {
		return nil
	}
	return s.withDB(func(db *sql.DB) error {
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		defer func() {
			_ = tx.Rollback()
		}()

		stmt, err := tx.Prepare(`DELETE FROM pages WHERE pageID = ?`)
		if err != nil {
			return err
		}
		defer func() {
			_ = stmt.Close()
		}()

		for _, pageID := range pageIDs {
			if _, err := stmt.Exec(pageID); err != nil {
				return err
			}
		}

		return tx.Commit()
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

func (s *SQLiteIndex) Search(query string, pageIDs []string, offset, limit int) (*SearchResult, error) {
	query = strings.TrimSpace(query)

	if len(pageIDs) == 0 && pageIDs != nil {
		return &SearchResult{
			Count:     0,
			Items:     []SearchResultItem{},
			Offset:    offset,
			Limit:     limit,
			TagFacets: []SearchTagFacet{},
		}, nil
	}

	if query == "" && len(pageIDs) == 0 {
		return &SearchResult{
			Count:     0,
			Items:     []SearchResultItem{},
			Offset:    offset,
			Limit:     limit,
			TagFacets: []SearchTagFacet{},
		}, nil
	}

	sr := &SearchResult{TagFacets: []SearchTagFacet{}}
	ftsQuery := buildFuzzyQuery(query)

	err := s.withDBRead(func(db *sql.DB) error {
		var total int
		whereClause, whereArgs := buildSearchWhereClause(query, ftsQuery, pageIDs)

		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM pages WHERE %s;`, whereClause)
		if err := db.QueryRow(countQuery, whereArgs...).Scan(&total); err != nil {
			return err
		}
		sr.Count = total

		searchQuery := fmt.Sprintf(`
		SELECT 
			pageID,
			path,
			kind,
			%s AS highlighted_title,
			%s AS excerpt,
			content,
			%s AS bm25_score
		FROM pages
		WHERE %s
		ORDER BY %s
		LIMIT ? OFFSET ?;
	`,
			searchTitleExpr(query != ""),
			searchExcerptExpr(query != ""),
			searchRankExpr(query != ""),
			whereClause,
			searchOrderByExpr(query != ""),
		)

		queryArgs := append(append([]interface{}{}, whereArgs...), limit, offset)
		rows, err := db.Query(searchQuery, queryArgs...)
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
			var content string

			if err := rows.Scan(&r.PageID, &r.Path, &r.Kind, &r.Title, &r.Excerpt, &content, &bm25Score); err != nil {
				return err
			}
			r.Title = sanitizeSearchTitle(r.Title)
			if strings.TrimSpace(r.Excerpt) == "" {
				r.Excerpt = excerpt.FromBody(content)
			}

			if query == "" {
				r.Rank = 1
			} else {
				// Convert bm25 score to a rank (lower score = higher rank)
				if bm25Score < 0 {
					bm25Score = 0
				}
				r.Rank = 1.0 / (1.0 + bm25Score)
			}

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

func (s *SQLiteIndex) SearchPageIDs(query string, pageIDs []string) ([]string, error) {
	query = strings.TrimSpace(query)

	if len(pageIDs) == 0 && pageIDs != nil {
		return []string{}, nil
	}

	if query == "" && len(pageIDs) == 0 {
		return []string{}, nil
	}

	ftsQuery := buildFuzzyQuery(query)
	var result []string

	err := s.withDBRead(func(db *sql.DB) error {
		whereClause, whereArgs := buildSearchWhereClause(query, ftsQuery, pageIDs)
		searchQuery := fmt.Sprintf(`
		SELECT pageID, %s AS bm25_score
		FROM pages
		WHERE %s
		ORDER BY %s;
	`, searchRankExpr(query != ""), whereClause, searchOrderByExpr(query != ""))

		rows, err := db.Query(searchQuery, whereArgs...)
		if err != nil {
			return err
		}
		defer func() {
			if err := rows.Close(); err != nil {
				slog.Default().Error("could not close rows", "error", err)
			}
		}()

		for rows.Next() {
			var pageID string
			var bm25Score float64
			if err := rows.Scan(&pageID, &bm25Score); err != nil {
				return err
			}
			result = append(result, pageID)
		}

		return rows.Err()
	})

	return result, err
}

func buildSearchWhereClause(query string, ftsQuery string, pageIDs []string) (string, []interface{}) {
	clauses := make([]string, 0, 2)
	args := make([]interface{}, 0, 1+len(pageIDs))

	if query != "" {
		clauses = append(clauses, "pages MATCH ?")
		args = append(args, ftsQuery)
	}

	if len(pageIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(pageIDs)), ",")
		clauses = append(clauses, fmt.Sprintf("pageID IN (%s)", placeholders))
		for _, pageID := range pageIDs {
			args = append(args, pageID)
		}
	}

	return strings.Join(clauses, " AND "), args
}

func searchTitleExpr(hasQuery bool) string {
	if hasQuery {
		return "highlight(pages, 4, char(2), char(3))"
	}
	return "title"
}

func sanitizeSearchTitle(title string) string {
	return htmlutil.EscapeTextWithAllowedMarkers(
		title,
		htmlutil.AllowedMarker{Marker: "\u0002", HTML: "<b>"},
		htmlutil.AllowedMarker{Marker: "\u0003", HTML: "</b>"},
	)
}

func searchExcerptExpr(hasQuery bool) string {
	if hasQuery {
		return "snippet(pages, 6, '<b>', '</b>', '...', 16)"
	}
	return "''"
}

func searchRankExpr(hasQuery bool) string {
	if hasQuery {
		return `bm25(pages,
				0.0,  -- path
				0.0,  -- filepath
				0.0,  -- pageID
				0.0,  -- kind
				20.0, -- title
				5.0,   -- headings
				1.0    -- content
			)`
	}
	return "0.0"
}

func searchOrderByExpr(hasQuery bool) string {
	if hasQuery {
		return "bm25_score ASC"
	}
	return "title COLLATE NOCASE ASC, path COLLATE NOCASE ASC"
}
