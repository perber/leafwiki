package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func getMetricsBody(t *testing.T, handler http.Handler) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d: %s", rec.Code, rec.Body.String())
	}

	return rec.Body.String()
}

func TestHTTPMetrics_ExportsPrometheusMetrics(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /health, got %d: %s", rec.Code, rec.Body.String())
	}

	body := getMetricsBody(t, metrics.HTTPHandler())
	if !strings.Contains(body, "leafwiki_http_requests_total") {
		t.Fatalf("expected metrics output to contain request counter, got: %s", body)
	}
	if !strings.Contains(body, "leafwiki_http_request_duration_seconds") {
		t.Fatalf("expected metrics output to contain duration histogram, got: %s", body)
	}
	if !strings.Contains(body, "leafwiki_http_requests_in_flight") {
		t.Fatalf("expected metrics output to contain in-flight gauge, got: %s", body)
	}
}

func TestHTTPMetrics_UsesNormalizedRouteLabel(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/api/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/health, got %d: %s", rec.Code, rec.Body.String())
	}

	body := getMetricsBody(t, metrics.HTTPHandler())
	expected := `leafwiki_http_requests_total{method="GET",route="/api/health",status="200"} 1`
	if !strings.Contains(body, expected) {
		t.Fatalf("expected metrics output to contain %q, got: %s", expected, body)
	}
}

func TestHTTPMetrics_UsesRoutePatternForPathParams(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/api/pages/:id", func(c *gin.Context) {
		c.String(http.StatusOK, c.Param("id"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/pages/abc123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/pages/:id, got %d: %s", rec.Code, rec.Body.String())
	}

	body := getMetricsBody(t, metrics.HTTPHandler())
	expected := `leafwiki_http_requests_total{method="GET",route="/api/pages/:id",status="200"} 1`
	if !strings.Contains(body, expected) {
		t.Fatalf("expected metrics output to contain %q, got: %s", expected, body)
	}
	if strings.Contains(body, "/api/pages/abc123") {
		t.Fatalf("expected metrics output not to contain concrete page ID, got: %s", body)
	}
}

func TestHTTPMetrics_UsesUnmatchedLabelForUnknownRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())

	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist/123", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown route, got %d: %s", rec.Code, rec.Body.String())
	}

	body := getMetricsBody(t, metrics.HTTPHandler())
	expected := `leafwiki_http_requests_total{method="GET",route="unmatched",status="404"} 1`
	if !strings.Contains(body, expected) {
		t.Fatalf("expected metrics output to contain %q, got: %s", expected, body)
	}
}
