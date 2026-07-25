package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const unmatchedRoute = "unmatched"

// HTTPMetrics owns the Prometheus registry and the small set of router-level
// metrics used for first-pass API load testing.
type HTTPMetrics struct {
	registry              *prometheus.Registry
	buildInfo             *prometheus.GaugeVec
	requestsTotal         *prometheus.CounterVec
	requestDuration       *prometheus.HistogramVec
	requestsInFlight      prometheus.Gauge
	pagesaveDuration      *prometheus.HistogramVec
	pagesaveOpsTotal      *prometheus.CounterVec
	pagesaveFailures      *prometheus.CounterVec
	sideEffectDuration    *prometheus.HistogramVec
	sideEffectFailures    *prometheus.CounterVec
	refactorAffectedPages *prometheus.HistogramVec
	refactorMatchedLinks  *prometheus.HistogramVec
	refactorDuration      *prometheus.HistogramVec
	resyncDuration        *prometheus.HistogramVec
	resyncRuns            *prometheus.CounterVec
	resyncFailures        *prometheus.CounterVec
	authLoginAttempts     *prometheus.CounterVec
	authTOTPVerifications *prometheus.CounterVec
	authSessions          *prometheus.CounterVec
	authTOTPEnrollment    *prometheus.CounterVec
	handler               http.Handler
}

// NewHTTPMetrics creates the Prometheus registry and metrics. version is
// reported via the leafwiki_build_info gauge so it can be scraped alongside
// runtime metrics.
func NewHTTPMetrics(version string) *HTTPMetrics {
	registry := prometheus.NewRegistry()

	buildInfo := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "leafwiki",
			Name:      "build_info",
			Help:      "Build information, constant 1, labeled by version.",
		},
		[]string{"version"},
	)
	buildInfo.WithLabelValues(version).Set(1)

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests by method, route, and status code.",
		},
		[]string{"method", "route", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds by method, route, and status code.",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "route", "status"},
	)

	requestsInFlight := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "leafwiki",
			Name:      "http_requests_in_flight",
			Help:      "Current number of in-flight HTTP requests.",
		},
	)

	pagesaveDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "pagesave_duration_seconds",
			Help:      "Page save workflow duration in seconds by operation and result.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"operation", "result"},
	)

	pagesaveOpsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "pagesave_operations_total",
			Help:      "Total number of page save workflows by operation and result.",
		},
		[]string{"operation", "result"},
	)

	pagesaveFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "pagesave_failures_total",
			Help:      "Total number of failed page save workflows by operation and result.",
		},
		[]string{"operation", "result"},
	)

	sideEffectDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "pagesave_sideeffect_duration_seconds",
			Help:      "Page save side-effect duration in seconds by operation and side effect.",
			Buckets:   []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"operation", "side_effect"},
	)

	sideEffectFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "pagesave_sideeffect_failures_total",
			Help:      "Total number of swallowed page save side-effect failures by operation and side effect.",
		},
		[]string{"operation", "side_effect"},
	)

	refactorAffectedPages := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "refactor_affected_pages",
			Help:      "Distribution of affected page counts for rename and move refactors.",
			Buckets:   []float64{0, 1, 2, 5, 10, 25, 50, 100, 250},
		},
		[]string{"kind", "rewrite_links"},
	)

	refactorMatchedLinks := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "refactor_matched_links",
			Help:      "Distribution of matched link counts for rename and move refactors.",
			Buckets:   []float64{0, 1, 2, 5, 10, 25, 50, 100, 250, 500},
		},
		[]string{"kind", "rewrite_links"},
	)

	refactorDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "refactor_duration_seconds",
			Help:      "Rename and move refactor duration in seconds.",
			Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"kind", "rewrite_links"},
	)

	resyncDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "leafwiki",
			Name:      "resync_duration_seconds",
			Help:      "Filesystem resync duration in seconds by result.",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"result"},
	)

	resyncRuns := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "resync_runs_total",
			Help:      "Total number of filesystem resync runs by result.",
		},
		[]string{"result"},
	)

	resyncFailures := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "resync_failures_total",
			Help:      "Total number of failed filesystem resync runs by result.",
		},
		[]string{"result"},
	)

	authLoginAttempts := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "auth_login_attempts_total",
			Help:      "Total number of login attempts by outcome.",
		},
		[]string{"outcome"},
	)

	authTOTPVerifications := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "auth_totp_verifications_total",
			Help:      "Total number of TOTP/recovery-code verifications during login by result.",
		},
		[]string{"result"},
	)

	authSessions := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "auth_sessions_total",
			Help:      "Total number of session lifecycle events by event type.",
		},
		[]string{"event"},
	)

	authTOTPEnrollment := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "leafwiki",
			Name:      "auth_totp_enrollment_total",
			Help:      "Total number of TOTP enrollment changes by event type.",
		},
		[]string{"event"},
	)

	registry.MustRegister(
		buildInfo,
		requestsTotal,
		requestDuration,
		requestsInFlight,
		pagesaveDuration,
		pagesaveOpsTotal,
		pagesaveFailures,
		sideEffectDuration,
		sideEffectFailures,
		refactorAffectedPages,
		refactorMatchedLinks,
		refactorDuration,
		resyncDuration,
		resyncRuns,
		resyncFailures,
		authLoginAttempts,
		authTOTPVerifications,
		authSessions,
		authTOTPEnrollment,
	)

	return &HTTPMetrics{
		registry:              registry,
		buildInfo:             buildInfo,
		requestsTotal:         requestsTotal,
		requestDuration:       requestDuration,
		requestsInFlight:      requestsInFlight,
		pagesaveDuration:      pagesaveDuration,
		pagesaveOpsTotal:      pagesaveOpsTotal,
		pagesaveFailures:      pagesaveFailures,
		sideEffectDuration:    sideEffectDuration,
		sideEffectFailures:    sideEffectFailures,
		refactorAffectedPages: refactorAffectedPages,
		refactorMatchedLinks:  refactorMatchedLinks,
		refactorDuration:      refactorDuration,
		resyncDuration:        resyncDuration,
		resyncRuns:            resyncRuns,
		resyncFailures:        resyncFailures,
		authLoginAttempts:     authLoginAttempts,
		authTOTPVerifications: authTOTPVerifications,
		authSessions:          authSessions,
		authTOTPEnrollment:    authTOTPEnrollment,
		handler:               promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
	}
}

func (m *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.requestsInFlight.Inc()
		defer m.requestsInFlight.Dec()

		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = unmatchedRoute
		}

		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method
		durationSeconds := time.Since(start).Seconds()

		m.requestsTotal.WithLabelValues(method, route, status).Inc()
		m.requestDuration.WithLabelValues(method, route, status).Observe(durationSeconds)
	}
}

func (m *HTTPMetrics) HTTPHandler() http.Handler {
	return m.handler
}

func (m *HTTPMetrics) ObservePageSaveWorkflow(operation string, err error, started time.Time) {
	if m == nil {
		return
	}
	result := resultLabel(err)
	m.pagesaveOpsTotal.WithLabelValues(operation, result).Inc()
	m.pagesaveDuration.WithLabelValues(operation, result).Observe(time.Since(started).Seconds())
	if err != nil {
		m.pagesaveFailures.WithLabelValues(operation, result).Inc()
	}
}

func (m *HTTPMetrics) ObservePageSaveSideEffect(operation, sideEffect string, started time.Time) {
	if m == nil {
		return
	}
	m.sideEffectDuration.WithLabelValues(operation, sideEffect).Observe(time.Since(started).Seconds())
}

func (m *HTTPMetrics) IncPageSaveSideEffectFailure(operation, sideEffect string) {
	if m == nil {
		return
	}
	m.sideEffectFailures.WithLabelValues(operation, sideEffect).Inc()
}

func (m *HTTPMetrics) ObserveRefactor(kind string, rewriteLinks bool, affectedPages, matchedLinks int, started time.Time) {
	if m == nil {
		return
	}
	rewrite := strconv.FormatBool(rewriteLinks)
	m.refactorAffectedPages.WithLabelValues(kind, rewrite).Observe(float64(affectedPages))
	m.refactorMatchedLinks.WithLabelValues(kind, rewrite).Observe(float64(matchedLinks))
	m.refactorDuration.WithLabelValues(kind, rewrite).Observe(time.Since(started).Seconds())
}

func (m *HTTPMetrics) ObserveResyncTriggerAccepted() {
	if m == nil {
		return
	}
	m.resyncRuns.WithLabelValues("accepted").Inc()
}

func (m *HTTPMetrics) ObserveResyncRun(err error, started time.Time) {
	if m == nil {
		return
	}
	result := resultLabel(err)
	m.resyncRuns.WithLabelValues(result).Inc()
	m.resyncDuration.WithLabelValues(result).Observe(time.Since(started).Seconds())
	if err != nil {
		m.resyncFailures.WithLabelValues(result).Inc()
	}
}

func (m *HTTPMetrics) IncAuthLoginAttempt(outcome string) {
	if m == nil {
		return
	}
	m.authLoginAttempts.WithLabelValues(outcome).Inc()
}

func (m *HTTPMetrics) IncAuthTOTPVerification(result string) {
	if m == nil {
		return
	}
	m.authTOTPVerifications.WithLabelValues(result).Inc()
}

func (m *HTTPMetrics) IncAuthSession(event string) {
	if m == nil {
		return
	}
	m.authSessions.WithLabelValues(event).Inc()
}

func (m *HTTPMetrics) IncAuthTOTPEnrollment(event string) {
	if m == nil {
		return
	}
	m.authTOTPEnrollment.WithLabelValues(event).Inc()
}

func resultLabel(err error) string {
	if err != nil {
		return "error"
	}
	return "success"
}
