package search

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
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

func NewSQLiteIndex(storageDir string) (*SQLiteIndex, error) {
	s := &SQLiteIndex{
		storageDir: storageDir,
		filename:   "search.db",
	}

	err := sqliteutil.OpenWithPolicy(searchIndexDatabasePath(s.storageDir, s.filename), sqliteutil.RebuildOnCorruption, func() error {
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

// searchSchemaVersion is stored in search.db's PRAGMA user_version. Bump it
// whenever the columns of a search table change, or the text written into
// them does, so an existing search.db is rebuilt instead of read in a shape
// or normalisation this binary doesn't expect (ADR-0022).
const searchSchemaVersion = 2

// searchSchema creates every table of search.db. ensureSchema compares the
// schema SQLite stores for it against the one in the file, so it is the
// single description of what a current search.db looks like.
var searchSchema = `
	CREATE VIRTUAL TABLE pages USING fts5(
		path UNINDEXED,
		filepath UNINDEXED,
		pageID,
		kind UNINDEXED,
		title,
		headings,
		content,
		tokenize = "unicode61 tokenchars '` + searchTokenChars + `'"
	);
`

// ensureSchema keeps search.db as it is when its user_version and its schema
// match this binary, and otherwise drops every table and recreates them
// empty. The schema check matters on its own: binaries before ADR-0022 drop
// and recreate pages in their own shape without touching user_version.
//
// The check and the rebuild run in one BEGIN IMMEDIATE transaction on one
// connection. Taking the write lock up front honours busy_timeout, whereas a
// deferred transaction that upgrades from read to write in WAL mode fails
// with SQLITE_BUSY at once when another opener wrote first. Checking inside
// the lock lets a waiting opener see the schema the first one just rebuilt
// and keep it.
//
// Boot still clears and refills the index (IndexAllPagesContext) right after
// this, so keeping the rows only saves the DROP for now; the fingerprint sync
// planned in ADR-0022 is what will make boot trust them.
func (s *SQLiteIndex) ensureSchema() error {
	want, err := expectedSearchSchema()
	if err != nil {
		return fmt.Errorf("build expected search schema: %w", err)
	}
	return s.withDB(func(db *sql.DB) error {
		return inImmediateTx(db, func(ctx context.Context, conn *sql.Conn) error {
			var version int
			if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
				return err
			}
			got, err := readSchema(conn)
			if err != nil {
				return err
			}
			if version == searchSchemaVersion && got == want {
				// Kept tables must still open. Without this a corrupt FTS5
				// table behind an intact sqlite_master passes the check, and
				// every later query fails without RebuildOnCorruption seeing it.
				return openVirtualTables(ctx, conn)
			}
			dropped, err := recreateSearchSchema(ctx, conn)
			if err != nil {
				return err
			}
			if dropped > 0 {
				slog.Default().Info("search index schema changed, rebuilding it empty",
					"storedVersion", version, "schemaVersion", searchSchemaVersion)
			}
			return nil
		})
	})
}

// inImmediateTx runs fn in a BEGIN IMMEDIATE transaction on one connection
// and commits it, or rolls it back if fn or the commit fails.
func inImmediateTx(db *sql.DB, fn func(ctx context.Context, conn *sql.Conn) error) error {
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()
	if err := fn(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return err
	}
	committed = true
	return nil
}

// ownSchemaObjects selects the objects that make up our schema: our tables,
// views, indexes and triggers and the FTS5 shadow tables (pages_*). SQLite's
// internal sqlite_* objects (sqlite_stat1 after ANALYZE or PRAGMA optimize,
// sqlite_sequence, sqlite_autoindex_*) are left out: SQLite manages them, and
// the ones that follow from our DDL are covered by comparing that DDL.
const ownSchemaObjects = `name NOT LIKE 'sqlite\_%' ESCAPE '\'`

// expectedSearchSchema creates searchSchema in an empty in-memory database
// and returns what SQLite stores for it. Comparing SQLite's own text instead
// of our DDL string sidesteps how SQLite normalises CREATE statements and
// includes the FTS5 shadow tables.
func expectedSearchSchema() (string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()
	// Every pooled connection would get its own empty :memory: database.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(searchSchema); err != nil {
		return "", err
	}
	return readSchema(db)
}

// schemaConn is what readSchema and recreateSearchSchema need from a
// *sql.DB or a *sql.Conn.
type schemaConn interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// readSchema returns our schema objects (see ownSchemaObjects) in a stable
// order.
func readSchema(db schemaConn) (string, error) {
	rows, err := db.QueryContext(context.Background(), `SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master WHERE `+ownSchemaObjects+` ORDER BY type, name`)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()
	var b strings.Builder
	for rows.Next() {
		var typ, name, table, ddl string
		if err := rows.Scan(&typ, &name, &table, &ddl); err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "%s|%s|%s|%s\n", typ, name, table, ddl)
	}
	return b.String(), rows.Err()
}

// openVirtualTables reads one row of every virtual table in our schema, which
// runs its module's constructor: FTS5 reads its %_config table and fails with
// SQLITE_CORRUPT if that is unreadable. It is deliberately not an FTS5
// integrity-check, whose cost grows with the index; corruption deeper in the
// index surfaces when a query or write first reaches it.
func openVirtualTables(ctx context.Context, conn schemaConn) error {
	rows, err := conn.QueryContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type = 'table' AND `+ownSchemaObjects+` AND sql LIKE 'CREATE VIRTUAL TABLE%'
		ORDER BY name`)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return err
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, name := range names {
		var rowid int64
		err := conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT rowid FROM %s LIMIT 1`, quoteIdentifier(name))).Scan(&rowid)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

// quoteIdentifier quotes a table or view name for SQL.
func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// recreateSearchSchema drops every table and view of our schema (see
// ownSchemaObjects), creates searchSchema and stamps searchSchemaVersion, and
// returns how many objects it dropped. It runs inside the caller's
// transaction. Virtual tables are dropped first, because dropping one also
// drops its shadow tables.
//
// A virtual table whose module this binary lacks cannot be dropped. That is
// reported as sqliteutil.ErrUnusableDatabase, so NewSQLiteIndex's
// RebuildOnCorruption replaces the file: search.db is derived.
func recreateSearchSchema(ctx context.Context, conn schemaConn) (int, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT type, name FROM sqlite_master
		WHERE type IN ('table', 'view') AND `+ownSchemaObjects+`
		ORDER BY sql NOT LIKE 'CREATE VIRTUAL TABLE%', name`)
	if err != nil {
		return 0, err
	}
	var drops []string
	for rows.Next() {
		var typ, name string
		if err := rows.Scan(&typ, &name); err != nil {
			_ = rows.Close()
			return 0, err
		}
		drops = append(drops, fmt.Sprintf(`DROP %s IF EXISTS %s`, strings.ToUpper(typ), quoteIdentifier(name)))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	for _, stmt := range drops {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			// SQLite reports this as a plain SQLITE_ERROR; the text is the
			// only way to tell it apart.
			if strings.Contains(err.Error(), "no such module") {
				return 0, fmt.Errorf("%w: %s: %w", sqliteutil.ErrUnusableDatabase, stmt, err)
			}
			return 0, err
		}
	}
	if _, err := conn.ExecContext(ctx, searchSchema); err != nil {
		return 0, err
	}
	_, err = conn.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, searchSchemaVersion))
	return len(drops), err
}

// Clear empties the index by dropping and recreating every search table
// rather than deleting rows, which keeps damaged FTS5 shadow tables around.
// It is the healing point: boot and resync run Clear plus a full reindex, so
// damage that ensureSchema's cheap open check cannot see (ADR-0022) heals
// there. Anything that replaces that full reindex (the fingerprint sync
// planned in ADR-0022) has to fall back to Clear plus a full reindex on a
// corruption error. Searches wait on s.mu while Clear runs.
func (s *SQLiteIndex) Clear() error {
	return s.withDB(func(db *sql.DB) error {
		return inImmediateTx(db, func(ctx context.Context, conn *sql.Conn) error {
			_, err := recreateSearchSchema(ctx, conn)
			return err
		})
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
		content = excerpt.NormalizeMarkdownBody(stripHighlightMarkers(content))
		prepped = append(prepped, prepared{
			path:          in.Path,
			filePath:      in.FilePath,
			pageID:        in.PageID,
			title:         stripHighlightMarkers(in.Title),
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

// Search returns one page of the hits for query, restricted to pageIDs when
// that is non-nil. Every statement runs under ctx: once ctx ends, the running
// statement is interrupted and Search returns ctx's error and no result.
func (s *SQLiteIndex) Search(ctx context.Context, query string, pageIDs []string, offset, limit int) (*SearchResult, error) {
	query = strings.TrimSpace(query)
	// ftsQuery, not the raw query, decides whether there's anything to MATCH
	// on: a query built entirely from characters outside the tokenizer's
	// word alphabet (e.g. "," or "'" alone) sanitizes to "" even though the
	// raw query wasn't empty, and `pages MATCH ''` is itself an FTS5 syntax
	// error — the same crash class #1577 reported, for the simplest
	// possible input.
	parsed := parseSearchQuery(query)
	ftsQuery := parsed.fts

	if len(pageIDs) == 0 && pageIDs != nil {
		return &SearchResult{
			Count:     0,
			Items:     []SearchResultItem{},
			Offset:    offset,
			Limit:     limit,
			TagFacets: []SearchTagFacet{},
		}, nil
	}

	if ftsQuery == "" && len(pageIDs) == 0 {
		return &SearchResult{
			Count:     0,
			Items:     []SearchResultItem{},
			Offset:    offset,
			Limit:     limit,
			TagFacets: []SearchTagFacet{},
		}, nil
	}

	sr := &SearchResult{TagFacets: []SearchTagFacet{}}

	err := s.withDBRead(func(db *sql.DB) error {
		var total int
		whereClause, whereArgs := buildSearchWhereClause(ftsQuery, pageIDs)

		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM pages WHERE %s;`, whereClause)
		if err := db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
			return err
		}
		sr.Count = total

		orderBy, orderArgs := searchOrderBy(parsed)
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
			searchTitleExpr(ftsQuery != ""),
			searchExcerptExpr(ftsQuery != ""),
			searchRankExpr(ftsQuery != ""),
			whereClause,
			orderBy,
		)

		// ctx bounds this statement only because its heavy work (MATCH, bm25,
		// highlight, snippet and the sort) runs in the first sqlite3_step,
		// which the driver interrupts when ctx ends; later rows.Next steps
		// only read the sorted rows and are not interrupted. An ORDER BY that
		// an index satisfies under the LIMIT, or SQLITE_ENABLE_SORTER_REFERENCES,
		// would move that work into those uninterruptible steps.
		queryArgs := append(append(append([]interface{}{}, whereArgs...), orderArgs...), limit, offset)
		rows, err := db.QueryContext(ctx, searchQuery, queryArgs...)
		if err != nil {
			return err
		}
		defer func() {
			if err := rows.Close(); err != nil {
				slog.Default().Error("could not close rows", "error", err)
			}
		}()

		// Non-nil so a query without a match serializes as [] and not null.
		results := []SearchResultItem{}
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

			if ftsQuery == "" {
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
	if err != nil {
		return nil, err
	}
	return sr, nil
}

// SearchPageIDs returns the IDs of all pages matching query, best first,
// restricted to pageIDs when that is non-nil. It honours ctx like Search.
func (s *SQLiteIndex) SearchPageIDs(ctx context.Context, query string, pageIDs []string) ([]string, error) {
	query = strings.TrimSpace(query)
	ftsQuery := buildFuzzyQuery(query)

	if len(pageIDs) == 0 && pageIDs != nil {
		return []string{}, nil
	}

	if ftsQuery == "" && len(pageIDs) == 0 {
		return []string{}, nil
	}

	var result []string

	err := s.withDBRead(func(db *sql.DB) error {
		whereClause, whereArgs := buildSearchWhereClause(ftsQuery, pageIDs)
		searchQuery := fmt.Sprintf(`
		SELECT pageID, %s AS bm25_score
		FROM pages
		WHERE %s
		ORDER BY %s;
	`, searchRankExpr(ftsQuery != ""), whereClause, searchOrderByExpr(ftsQuery != ""))

		rows, err := db.QueryContext(ctx, searchQuery, whereArgs...)
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
	if err != nil {
		return nil, err
	}
	return result, nil
}

// hitStart and hitEnd are the single-byte markers highlight() puts around a
// hit in the title (searchTitleExpr); sanitizeSearchTitle turns them into
// <b>/</b>.
const (
	hitStart = "\u0002"
	hitEnd   = "\u0003"
)

// highlightMarkerReplacer replaces the hit markers in page text with a
// space, so page text can never fake a highlight. A space rather than
// nothing keeps the words on either side apart.
var highlightMarkerReplacer = strings.NewReplacer(hitStart, " ", hitEnd, " ")

func stripHighlightMarkers(text string) string {
	return highlightMarkerReplacer.Replace(text)
}

func sanitizeSearchTitle(title string) string {
	return htmlutil.EscapeTextWithAllowedMarkers(
		title,
		htmlutil.AllowedMarker{Marker: hitStart, HTML: "<b>"},
		htmlutil.AllowedMarker{Marker: hitEnd, HTML: "</b>"},
	)
}
