package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all the Prometheus metrics for the application
type Metrics struct {
	// HTTP metrics
	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDuration   *prometheus.HistogramVec
	HTTPRequestsInFlight  prometheus.Gauge
	HTTPRequestSize       *prometheus.HistogramVec
	HTTPResponseSize      *prometheus.HistogramVec

	// MCP-specific metrics
	MCPSessionsActive     prometheus.Gauge
	MCPSessionsTotal      *prometheus.CounterVec
	MCPSessionDuration    *prometheus.HistogramVec
	MCPToolExecutions     *prometheus.CounterVec
	MCPToolDuration       *prometheus.HistogramVec

	// System metrics
	GoRoutines            prometheus.Gauge
	MemoryUsage          prometheus.Gauge
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics() *Metrics {
	return &Metrics{
		// HTTP metrics
		HTTPRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		HTTPRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),
		HTTPRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed",
			},
		),
		HTTPRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "Size of HTTP requests in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "endpoint"},
		),
		HTTPResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "Size of HTTP responses in bytes",
				Buckets: prometheus.ExponentialBuckets(100, 10, 6),
			},
			[]string{"method", "endpoint"},
		),

		// MCP-specific metrics
		MCPSessionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "mcp_sessions_active",
				Help: "Number of active MCP sessions",
			},
		),
		MCPSessionsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_sessions_total",
				Help: "Total number of MCP sessions created",
			},
			[]string{"action"}, // created, deleted, expired
		),
		MCPSessionDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_session_duration_seconds",
				Help:    "Duration of MCP sessions in seconds",
				Buckets: []float64{60, 300, 600, 1800, 3600, 7200}, // 1m, 5m, 10m, 30m, 1h, 2h
			},
			[]string{"reason"}, // expired, deleted
		),
		MCPToolExecutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_tool_executions_total",
				Help: "Total number of MCP tool executions",
			},
			[]string{"tool_name", "status"}, // success, error
		),
		MCPToolDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_tool_execution_duration_seconds",
				Help:    "Duration of MCP tool executions in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"tool_name"},
		),

		// System metrics
		GoRoutines: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "go_goroutines_current",
				Help: "Number of goroutines that currently exist",
			},
		),
		MemoryUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),
	}
}

// RecordHTTPRequest records metrics for an HTTP request
func (m *Metrics) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration, requestSize, responseSize int64) {
	m.HTTPRequestsTotal.WithLabelValues(method, endpoint, statusCode).Inc()
	m.HTTPRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
	m.HTTPRequestSize.WithLabelValues(method, endpoint).Observe(float64(requestSize))
	m.HTTPResponseSize.WithLabelValues(method, endpoint).Observe(float64(responseSize))
}

// IncHTTPRequestsInFlight increments the in-flight requests counter
func (m *Metrics) IncHTTPRequestsInFlight() {
	m.HTTPRequestsInFlight.Inc()
}

// DecHTTPRequestsInFlight decrements the in-flight requests counter
func (m *Metrics) DecHTTPRequestsInFlight() {
	m.HTTPRequestsInFlight.Dec()
}

// RecordSessionCreated records a new session creation
func (m *Metrics) RecordSessionCreated() {
	m.MCPSessionsActive.Inc()
	m.MCPSessionsTotal.WithLabelValues("created").Inc()
}

// RecordSessionDeleted records a session deletion
func (m *Metrics) RecordSessionDeleted(duration time.Duration) {
	m.MCPSessionsActive.Dec()
	m.MCPSessionsTotal.WithLabelValues("deleted").Inc()
	m.MCPSessionDuration.WithLabelValues("deleted").Observe(duration.Seconds())
}

// RecordSessionExpired records a session expiration
func (m *Metrics) RecordSessionExpired(duration time.Duration) {
	m.MCPSessionsActive.Dec()
	m.MCPSessionsTotal.WithLabelValues("expired").Inc()
	m.MCPSessionDuration.WithLabelValues("expired").Observe(duration.Seconds())
}

// RecordToolExecution records a tool execution
func (m *Metrics) RecordToolExecution(toolName, status string, duration time.Duration) {
	m.MCPToolExecutions.WithLabelValues(toolName, status).Inc()
	m.MCPToolDuration.WithLabelValues(toolName).Observe(duration.Seconds())
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics(goroutines int, memoryBytes uint64) {
	m.GoRoutines.Set(float64(goroutines))
	m.MemoryUsage.Set(float64(memoryBytes))
}