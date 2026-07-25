//go:build integration

package tracing_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
)

// TestLiveOrchestratorMultiHop hits the running orchestrator container and
// verifies Jaeger shows linked chat-api → orchestrator spans.
func TestLiveOrchestratorMultiHop(t *testing.T) {
	otelEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if otelEndpoint == "" {
		t.Skip("OTEL_EXPORTER_OTLP_ENDPOINT not set")
	}
	orchURL := strings.TrimSpace(os.Getenv("ORCHESTRATOR_URL"))
	if orchURL == "" {
		t.Skip("ORCHESTRATOR_URL not set")
	}
	jaegerAPI := strings.TrimSpace(os.Getenv("JAEGER_QUERY_URL"))
	if jaegerAPI == "" {
		jaegerAPI = "http://127.0.0.1:16686"
	}

	ctx := context.Background()
	cleanup, err := tracing.Setup(ctx, "chat-api-smoke")
	if err != nil {
		t.Fatalf("tracing setup: %v", err)
	}
	defer cleanup()

	parentCtx, span := tracing.StartSpan(ctx, "chat-api", "smoke.trigger")
	defer span.End()

	req, err := http.NewRequestWithContext(parentCtx, http.MethodGet, strings.TrimRight(orchURL, "/")+"/health", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	tracing.PropagateOutbound(parentCtx, req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("orchestrator request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("orchestrator status: %d", resp.StatusCode)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		found, traceID, err := findMultiHopTrace(jaegerAPI, "chat-api-smoke", "orchestrator")
		if err != nil {
			t.Fatalf("jaeger query: %v", err)
		}
		if found {
			t.Logf("live multi-hop trace verified trace_id=%s", traceID)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatal("timed out waiting for live chat-api → orchestrator trace in Jaeger")
}
