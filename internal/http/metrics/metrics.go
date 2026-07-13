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
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight prometheus.Gauge
	handler          http.Handler
}

func NewHTTPMetrics() *HTTPMetrics {
	registry := prometheus.NewRegistry()

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

	registry.MustRegister(requestsTotal, requestDuration, requestsInFlight)

	return &HTTPMetrics{
		requestsTotal:    requestsTotal,
		requestDuration:  requestDuration,
		requestsInFlight: requestsInFlight,
		handler:          promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
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

func (m *HTTPMetrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.handler.ServeHTTP(c.Writer, c.Request)
	}
}
