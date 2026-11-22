package search

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	sharederrors "github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	coresearch "github.com/perber/wiki/internal/search"
	coretags "github.com/perber/wiki/internal/tags"
	"github.com/perber/wiki/internal/test_utils"
)

func setupSearchUseCaseForTags(t *testing.T) (*SearchUseCase, *coretags.TagsService, *tree.TreeService) {
	t.Helper()

	dir := t.TempDir()

	treeSvc := tree.NewTreeService(dir)
	if err := treeSvc.LoadTree(); err != nil {
		t.Fatalf("LoadTree: %v", err)
	}

	index, err := coresearch.NewSQLiteIndex(dir)
	if err != nil {
		t.Fatalf("NewSQLiteIndex: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(index.Close, t) })

	tagsStore, err := coretags.NewTagsStore(dir)
	if err != nil {
		t.Fatalf("NewTagsStore: %v", err)
	}
	t.Cleanup(func() { test_utils.WrapCloseWithErrorCheck(tagsStore.Close, t) })

	tagsSvc := coretags.NewTagsService(tagsStore)
	return NewSearchUseCase(index, tagsSvc, treeSvc, slog.Default()), tagsSvc, treeSvc
}

func TestSearchUseCase_Execute_TagOnlySearchEscapesTitle(t *testing.T) {
	uc, tagsSvc, treeSvc := setupSearchUseCaseForTags(t)
	addSearchPage(t, treeSvc, tagsSvc, uc.index, searchPage{
		title: `<img src=x onerror="alert(1)">`, slug: "tagged-title", body: "# Page body", tags: []string{"docs"},
	})

	out, err := uc.Execute(context.Background(), SearchInput{
		Tags:  []string{"docs"},
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if out.Result == nil || len(out.Result.Items) != 1 {
		t.Fatalf("expected 1 result item, got %#v", out.Result)
	}

	got := out.Result.Items[0].Title
	if got != "&lt;img src=x onerror=&#34;alert(1)&#34;&gt;" {
		t.Fatalf("escaped title = %q", got)
	}
}

// TestSearchUseCase_Execute_RejectsOverLongQueries pins the query caps: at
// most maxQueryRunes runes after trimming and maxQueryTerms search terms.
// The use case runs without an index, so a query that passes the caps gets
// search_unavailable and one that is rejected proves the check runs before
// any SQL.
func TestSearchUseCase_Execute_RejectsOverLongQueries(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		tags     []string
		wantCode string
	}{
		{name: "many short words", query: strings.Repeat("pi ", 50), wantCode: ErrCodeSearchQueryTooLong},
		{name: "12 terms pass", query: strings.TrimSpace(strings.Repeat("pi ", 12)), wantCode: ErrCodeSearchUnavailable},
		{name: "13 terms fail", query: strings.TrimSpace(strings.Repeat("pi ", 13)), wantCode: ErrCodeSearchQueryTooLong},
		{name: "terms split by punctuation count", query: strings.Repeat("a,", 13), wantCode: ErrCodeSearchQueryTooLong},
		{name: "256 runes pass", query: strings.Repeat("a", 256), wantCode: ErrCodeSearchUnavailable},
		{name: "257 runes fail", query: strings.Repeat("a", 257), wantCode: ErrCodeSearchQueryTooLong},
		{name: "length counts runes, not bytes", query: strings.Repeat("ü", 256), wantCode: ErrCodeSearchUnavailable},
		{name: "surrounding spaces don't count", query: "   " + strings.Repeat("a", 256) + "   ", wantCode: ErrCodeSearchUnavailable},
		{name: "tag-only search is unaffected", tags: []string{"docs"}, wantCode: ErrCodeSearchUnavailable},
		{name: "whitespace-only query is not too long", query: strings.Repeat(" ", 300), wantCode: ErrCodeSearchUnavailable},
		{name: "257 multi-byte runes fail", query: strings.Repeat("ü", 257), wantCode: ErrCodeSearchQueryTooLong},
		{name: "256 emoji pass the rune cap", query: strings.Repeat("🙂", 256), wantCode: ErrCodeSearchUnavailable},
		{name: "257 emoji fail the rune cap", query: strings.Repeat("🙂", 257), wantCode: ErrCodeSearchQueryTooLong},
		{name: "12 terms across field kinds pass", query: `(a OR b) NOT title:c,d "e f g" h* i,j k l`, wantCode: ErrCodeSearchUnavailable},
		{name: "13 terms across field kinds fail", query: `(a OR b) NOT title:c,d "e f g" h* i,j k l m`, wantCode: ErrCodeSearchQueryTooLong},
		{name: "13 column filter words fail", query: "title:a,b,c,d,e,f,g,h,i,j,k,l,m", wantCode: ErrCodeSearchQueryTooLong},
		{name: "13 words in one quoted phrase fail", query: `"a b c d e f g h i j k l m"`, wantCode: ErrCodeSearchQueryTooLong},
		{name: "boolean keywords don't count", query: "a OR b OR c OR d OR e OR f OR g OR h OR i OR j OR k OR l", wantCode: ErrCodeSearchUnavailable},
		{name: "over-long query with tags is still rejected", query: strings.Repeat("pi ", 13), tags: []string{"docs"}, wantCode: ErrCodeSearchQueryTooLong},
	}
	uc := NewSearchUseCase(nil, nil, nil, slog.Default())
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := uc.Execute(context.Background(), SearchInput{Query: tc.query, Tags: tc.tags, Limit: 10})
			loc, ok := sharederrors.AsLocalizedError(err)
			if !ok {
				t.Fatalf("Execute error = %v, want a LocalizedError with code %s", err, tc.wantCode)
			}
			if loc.Code != tc.wantCode {
				t.Errorf("Execute error code = %s, want %s", loc.Code, tc.wantCode)
			}
		})
	}
}

// withSearchTimeout sets searchTimeout for the test and restores it after.
func withSearchTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	previous := searchTimeout
	searchTimeout = d
	t.Cleanup(func() { searchTimeout = previous })
}

// captureLogs sends the default logger to a buffer for the test, without
// timestamps, and restores it after.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &logs
}

// TestSearchUseCase_Execute_ContextEnds pins what a search returns when its
// context ends before the index answers: the search timeout and an expired
// request deadline become search_unavailable and log one warning with the
// query's size but not its text; a canceled request (the client is gone)
// returns context.Canceled and logs nothing.
func TestSearchUseCase_Execute_ContextEnds(t *testing.T) {
	const query = "secret words"
	tests := []struct {
		name     string
		timeout  time.Duration
		ctx      func(t *testing.T) context.Context
		wantCode string
		wantErr  error
		wantLog  string
	}{
		{
			name:     "search timeout elapses",
			timeout:  0,
			ctx:      func(t *testing.T) context.Context { return t.Context() },
			wantCode: ErrCodeSearchUnavailable,
			wantLog:  "level=WARN msg=\"search timed out\" component=SearchUseCase queryRunes=12 terms=2\n",
		},
		{
			name:    "request deadline has passed",
			timeout: time.Minute,
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
				t.Cleanup(cancel)
				return ctx
			},
			wantCode: ErrCodeSearchUnavailable,
			wantLog:  "level=WARN msg=\"search timed out\" component=SearchUseCase queryRunes=12 terms=2\n",
		},
		{
			name:    "client canceled the request",
			timeout: time.Minute,
			ctx: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withSearchTimeout(t, tc.timeout)
			uc, _, _ := setupSearchUseCaseForTags(t)
			if err := uc.index.IndexPage("docs/a", "docs/a.md", "a", "A", tree.NodeKindPage, query); err != nil {
				t.Fatalf("IndexPage: %v", err)
			}
			logs := captureLogs(t)
			uc = NewSearchUseCase(uc.index, uc.tags, uc.tree, slog.Default())

			out, err := uc.Execute(tc.ctx(t), SearchInput{Query: query, Limit: 10})

			if out != nil {
				t.Errorf("Execute output = %#v, want nil", out)
			}
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("Execute error = %v, want %v", err, tc.wantErr)
				}
			} else if loc, ok := sharederrors.AsLocalizedError(err); !ok || loc.Code != tc.wantCode {
				t.Errorf("Execute error = %v, want a LocalizedError with code %s", err, tc.wantCode)
			}
			if got := logs.String(); got != tc.wantLog {
				t.Errorf("logs = %q, want %q", got, tc.wantLog)
			}
		})
	}
}
