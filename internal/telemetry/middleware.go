package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// responseWriter wraps http.ResponseWriter to capture response size and status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	size       int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	size, err := rw.ResponseWriter.Write(b)
	rw.size += int64(size)
	return size, err
}

// HTTPMetricsMiddleware creates middleware that records HTTP metrics
func HTTPMetricsMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// Increment in-flight requests
			metrics.IncHTTPRequestsInFlight()
			defer metrics.DecHTTPRequestsInFlight()

			// Wrap response writer to capture metrics
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     200, // default status code
			}

			// Get request size
			requestSize := r.ContentLength
			if requestSize < 0 {
				requestSize = 0
			}

			// Process request
			next.ServeHTTP(rw, r)

			// Calculate duration
			duration := time.Since(start)

			// Get route pattern for better grouping
			endpoint := r.URL.Path
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				if pattern := routeCtx.RoutePattern(); pattern != "" {
					endpoint = pattern
				}
			}

			// Record metrics
			metrics.RecordHTTPRequest(
				r.Method,
				endpoint,
				strconv.Itoa(rw.statusCode),
				duration,
				requestSize,
				rw.size,
			)
		})
	}
}