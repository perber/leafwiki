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
	"github.com/perber/wiki/internal/core/shared/sqliteutil"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	_ "modernc.org/sqlite"
)

type SQLiteIndex struct {
	mu         sync.Mutex
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

	if err := s.ensureSchema(); err != nil {
		// Only attempt recovery for genuine SQLite I/O or corruption errors
		// (SQLITE_IOERR=10, SQLITE_CORRUPT=11, SQLITE_NOTADB=26).
		// Transient errors like SQLITE_BUSY are returned immediately.
		if !sqliteutil.IsSQLiteRecoverableError(err) {
			return nil, err
		}
		slog.Default().Warn("search index initialization failed, removing corrupt database and retrying", "error", err)
		if closeErr := s.Close(); closeErr != nil {
			slog.Default().Warn("failed to close corrupt search database before recovery", "error", closeErr)
		}
		sqliteutil.RemoveSQLiteFiles(searchIndexDatabasePath(s.storageDir, s.filename))
		if err2 := s.ensureSchema(); err2 != nil {
			_ = s.Close()
			return nil, err2
		}
	}

	return s, nil
}


func (s *SQLiteIndex) withDB(fn func(db *sql.DB) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		db, err := sql.Open("sqlite", searchIndexDatabasePath(s.storageDir, s.filename))
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
	return s.withDB(func(db *sql.DB) error {
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

func (s *SQLiteIndex) IndexPage(path string, filePath string, pageID string, title string, kind tree.NodeKind, raw string) error {
	_, content, _, err := markdown.ParseFrontmatter(raw)
	if err != nil {
		return err
	}

	content = excerpt.NormalizeMarkdownBody(content)

	// Headings extracted from the Markdown
	headings := extractHeadings(content)

	sanitizedBody := excerpt.PlainTextFromMarkdown(content)

	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(`DELETE FROM pages WHERE pageID = ?`, pageID)
		if err != nil {
			return err
		}

		_, err = db.Exec(`
		INSERT INTO pages (path, filepath, pageID, kind, title, headings, content)
		VALUES (?, ?, ?, ?, ?, ?, ?);
	`, path, filePath, pageID, string(kind), title, headings, sanitizedBody)

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

	err := s.withDB(func(db *sql.DB) error {
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

	err := s.withDB(func(db *sql.DB) error {
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
		return "highlight(pages, 4, '<b>', '</b>')"
	}
	return "title"
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
