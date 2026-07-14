package pagesave

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/perber/wiki/internal/core/revision"
	"github.com/perber/wiki/internal/core/tree"
	httpmetrics "github.com/perber/wiki/internal/http/metrics"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/search"
)

func TestNewLinkIndexSideEffect_DefaultsLogger(t *testing.T) {
	treeService := tree.NewTreeService(t.TempDir())
	store, err := links.NewLinksStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLinksStore failed: %v", err)
	}
	effect := NewLinkIndexSideEffect(links.NewLinkService(t.TempDir(), treeService, store), nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewRevisionSideEffect_DefaultsLogger(t *testing.T) {
	treeService := tree.NewTreeService(t.TempDir())
	effect := NewRevisionSideEffect(revision.NewService(t.TempDir(), treeService, nil, revision.ServiceOptions{}), nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewSearchIndexSideEffect_DefaultsLogger(t *testing.T) {
	treeService := tree.NewTreeService(t.TempDir())
	index, err := search.NewSQLiteIndex(t.TempDir())
	if err != nil {
		t.Fatalf("NewSQLiteIndex failed: %v", err)
	}
	defer func() {
		if err := index.Close(); err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	}()

	effect := NewSearchIndexSideEffect(index, treeService, nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewTagsSideEffect_DefaultsLogger(t *testing.T) {
	effect := NewTagsSideEffect(nil, nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

func TestNewPropertiesSideEffect_DefaultsLogger(t *testing.T) {
	effect := NewPropertiesSideEffect(nil, nil, nil)
	if effect.log == nil {
		t.Fatal("expected default logger to be set")
	}
	if effect.log != slog.Default() {
		t.Fatal("expected slog.Default() logger")
	}
}

type metricTestEffect struct {
	metrics *httpmetrics.HTTPMetrics
}

func (e metricTestEffect) Name() string {
	return "search"
}

func (e metricTestEffect) Apply(event PageSaveEvent) {
	e.metrics.IncPageSaveSideEffectFailure(string(event.Operation), e.Name())
}

func metricsBody(t *testing.T, metrics *httpmetrics.HTTPMetrics) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	metrics.HTTPHandler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected metrics endpoint to return 200, got %d", rec.Code)
	}
	return rec.Body.String()
}

func TestPageSaveOrchestrator_RecordsSideEffectDurationAndFailures(t *testing.T) {
	metrics := httpmetrics.NewHTTPMetrics()
	orchestrator := NewPageSaveOrchestrator(metrics, metricTestEffect{metrics: metrics})

	orchestrator.Run(PageSaveEvent{Operation: PageOperationUpdate})

	body := metricsBody(t, metrics)
	if !strings.Contains(body, `leafwiki_pagesave_sideeffect_duration_seconds_bucket{operation="update",side_effect="search"`) {
		t.Fatalf("expected side-effect duration metric, got: %s", body)
	}
	expectedFailure := `leafwiki_pagesave_sideeffect_failures_total{operation="update",side_effect="search"} 1`
	if !strings.Contains(body, expectedFailure) {
		t.Fatalf("expected side-effect failure metric %q, got: %s", expectedFailure, body)
	}
}
