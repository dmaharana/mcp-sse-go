# Telemetry Implementation

This document describes the telemetry implementation for the MCP Server, including Prometheus metrics collection and Grafana visualization.

## Architecture

```
MCP Server → Prometheus Metrics → Prometheus Server → Grafana Dashboard
```

## Metrics Collected

### HTTP Metrics
- `http_requests_total` - Total number of HTTP requests (by method, endpoint, status code)
- `http_request_duration_seconds` - Duration of HTTP requests (by method, endpoint)
- `http_requests_in_flight` - Number of HTTP requests currently being processed
- `http_request_size_bytes` - Size of HTTP requests (by method, endpoint)
- `http_response_size_bytes` - Size of HTTP responses (by method, endpoint)

### MCP-Specific Metrics
- `mcp_sessions_active` - Number of active MCP sessions
- `mcp_sessions_total` - Total number of MCP sessions (by action: created, deleted, expired)
- `mcp_session_duration_seconds` - Duration of MCP sessions (by reason: expired, deleted)
- `mcp_tool_executions_total` - Total number of MCP tool executions (by tool name, status)
- `mcp_tool_execution_duration_seconds` - Duration of MCP tool executions (by tool name)

### System Metrics
- `go_goroutines_current` - Number of goroutines currently running
- `memory_usage_bytes` - Current memory usage in bytes

## Endpoints

- `/metrics` - Prometheus metrics endpoint
- `/health` - Health check endpoint

## Setup and Usage

### 1. Local Development

Build and run the server with telemetry:

```bash
make build
make run
```

The metrics will be available at `http://localhost:8080/metrics`.

### 2. Docker Compose Setup

Start the complete monitoring stack:

```bash
# Build the application
make docker-build

# Start all services (MCP Server, Prometheus, Grafana)
make docker-up
```

This will start:
- MCP Server on `http://localhost:8080`
- Prometheus on `http://localhost:9090`
- Grafana on `http://localhost:3000`

### 3. Monitoring Only

To start only Prometheus and Grafana (if you're running the MCP server locally):

```bash
make monitoring-up
```

## Accessing the Dashboard

1. Open Grafana at `http://localhost:3000`
2. Login with:
   - Username: `admin`
   - Password: `admin`
3. Navigate to the "MCP Server Dashboard"

## Dashboard Panels

The Grafana dashboard includes:

1. **HTTP Request Rate** - Shows the rate of incoming HTTP requests
2. **Active MCP Sessions** - Gauge showing current active sessions
3. **HTTP Request Duration** - 95th and 50th percentile response times
4. **Tool Execution Rate** - Rate of tool executions by tool name and status
5. **Memory Usage** - Current memory consumption
6. **Goroutines** - Number of active goroutines

## Configuration

### Prometheus Configuration

The Prometheus configuration (`monitoring/prometheus.yml`) scrapes metrics from the MCP server every 5 seconds:

```yaml
scrape_configs:
  - job_name: 'mcp-server'
    static_configs:
      - targets: ['mcp-server:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s
```

### Grafana Configuration

Grafana is automatically configured with:
- Prometheus as the default datasource
- Pre-built MCP Server dashboard
- Auto-refresh every 5 seconds

## Alerting (Optional)

You can extend the setup with alerting rules in Prometheus. Example alert for high error rate:

```yaml
groups:
  - name: mcp-server
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status_code=~"5.."}[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors per second"
```

## Troubleshooting

### Metrics Not Appearing

1. Check if the MCP server is running: `curl http://localhost:8080/health`
2. Check if metrics endpoint is accessible: `curl http://localhost:8080/metrics`
3. Verify Prometheus is scraping: Check `http://localhost:9090/targets`

### Dashboard Not Loading

1. Verify Grafana is running: `docker-compose ps grafana`
2. Check Grafana logs: `docker-compose logs grafana`
3. Ensure Prometheus datasource is configured correctly

### Performance Impact

The telemetry implementation is designed to be lightweight:
- Metrics collection adds minimal overhead (~1-2% CPU)
- Memory usage increases by ~10-20MB for metric storage
- HTTP middleware adds ~0.1ms latency per request

## Extending Metrics

To add custom metrics:

1. Define new metrics in `internal/telemetry/metrics.go`
2. Add recording methods to the `Metrics` struct
3. Call the recording methods from your application code
4. Update the Grafana dashboard to visualize new metrics

Example:

```go
// In metrics.go
CustomCounter: promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "custom_events_total",
        Help: "Total number of custom events",
    },
    []string{"event_type"},
)

// In your code
metrics.CustomCounter.WithLabelValues("user_action").Inc()
```