package telemetry

import (
	"context"
	"encoding/json"
	"time"

	"mcp-sse-go/internal/tools"
)

// ToolRegistryWrapper wraps a tool registry to add telemetry
type ToolRegistryWrapper struct {
	*tools.Registry
	metrics *Metrics
}

// NewToolRegistryWrapper creates a new telemetry-aware tool registry wrapper
func NewToolRegistryWrapper(registry *tools.Registry, metrics *Metrics) *ToolRegistryWrapper {
	return &ToolRegistryWrapper{
		Registry: registry,
		metrics:  metrics,
	}
}

// Call wraps the original Call to add telemetry
func (w *ToolRegistryWrapper) Call(ctx context.Context, name string, args json.RawMessage) (interface{}, error) {
	start := time.Now()
	
	result, err := w.Registry.Call(ctx, name, args)
	
	duration := time.Since(start)
	status := "success"
	if err != nil {
		status = "error"
	}
	
	w.metrics.RecordToolExecution(name, status, duration)
	
	return result, err
}