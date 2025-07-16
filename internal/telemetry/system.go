package telemetry

import (
	"context"
	"runtime"
	"time"

	"github.com/rs/zerolog"
)

// SystemMetricsCollector collects system-level metrics periodically
type SystemMetricsCollector struct {
	metrics  *Metrics
	logger   zerolog.Logger
	interval time.Duration
	done     chan struct{}
}

// NewSystemMetricsCollector creates a new system metrics collector
func NewSystemMetricsCollector(metrics *Metrics, logger zerolog.Logger, interval time.Duration) *SystemMetricsCollector {
	return &SystemMetricsCollector{
		metrics:  metrics,
		logger:   logger,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins collecting system metrics
func (c *SystemMetricsCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Info().
		Dur("interval", c.interval).
		Msg("Starting system metrics collection")

	for {
		select {
		case <-ctx.Done():
			c.logger.Info().Msg("Stopping system metrics collection due to context cancellation")
			return
		case <-c.done:
			c.logger.Info().Msg("Stopping system metrics collection")
			return
		case <-ticker.C:
			c.collectMetrics()
		}
	}
}

// Stop stops the metrics collection
func (c *SystemMetricsCollector) Stop() {
	close(c.done)
}

// collectMetrics collects and updates system metrics
func (c *SystemMetricsCollector) collectMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Update metrics
	c.metrics.UpdateSystemMetrics(
		runtime.NumGoroutine(),
		m.Alloc,
	)

	c.logger.Debug().
		Int("goroutines", runtime.NumGoroutine()).
		Uint64("memory_bytes", m.Alloc).
		Msg("Updated system metrics")
}