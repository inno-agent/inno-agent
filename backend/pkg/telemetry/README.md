# pkg/telemetry

Shared observability library.

## Purpose

Prometheus metrics and HTTP middleware for all Go services.

## Features

- Prometheus metric registration
- HTTP middleware for Chi, Gin, net/http
- Standalone metrics server for headless workers
- Per-service metric aliases

## Usage

```go
import "github.com/inno-agent/inno-agent/backend/pkg/telemetry"

// Initialize
telemetry.Init("my-service")

// Chi middleware
r.Use(telemetry.ChiMiddleware("my-service"))

// Metrics endpoint
telemetry.Handler() // returns http.Handler for /metrics

// Standalone server (for workers without HTTP)
telemetry.ListenAndServe(":9090", "my-service")
```

## Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `service_http_requests_total` | Counter | Total HTTP requests |
| `service_http_request_duration_seconds` | Histogram | Request latency |
| `service_http_requests_in_flight` | Gauge | Current in-flight requests |
| `service_errors_total` | Counter | Total errors |
| `service_up` | Gauge | Service health (1=up, 0=down) |
| `service_healthcheck_total` | Counter | Healthcheck probes |

## Used By

- chat-api
- identity
- orchestrator (innoagent)
- review-api
- review-consumer
- review-webhook
