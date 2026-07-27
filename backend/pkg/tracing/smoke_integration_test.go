//go:build integration

package tracing_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
)

// TestMultiHopExport verifies parent→child spans across two HTTP services,
// using the same middleware and PropagateOutbound as production chat-api → orchestrator.
func TestMultiHopExport(t *testing.T) {
	otelEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if otelEndpoint == "" {
		t.Skip("OTEL_EXPORTER_OTLP_ENDPOINT not set")
	}
	jaegerAPI := strings.TrimSpace(os.Getenv("JAEGER_QUERY_URL"))
	if jaegerAPI == "" {
		jaegerAPI = "http://127.0.0.1:16686"
	}

	ctx := context.Background()
	cleanup, err := tracing.Setup(ctx, "chat-api")
	if err != nil {
		t.Fatalf("tracing setup: %v", err)
	}
	defer cleanup()

	orchestrator := httptest.NewServer(tracing.HTTPMiddleware("orchestrator", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})))
	defer orchestrator.Close()

	router := chi.NewRouter()
	router.Use(tracing.ChiMiddleware("chat-api"))
	router.Get("/trigger", func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, orchestrator.URL+"/health", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tracing.PropagateOutbound(r.Context(), req)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		w.WriteHeader(resp.StatusCode)
	})

	chatAPI := httptest.NewServer(router)
	defer chatAPI.Close()

	correlationID := fmt.Sprintf("smoke-%d", time.Now().UnixNano())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, chatAPI.URL+"/trigger", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Correlation-ID", correlationID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("trigger request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("trigger status %d: %s", resp.StatusCode, body)
	}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		found, traceID, err := findMultiHopTrace(jaegerAPI, "chat-api", "orchestrator")
		if err != nil {
			t.Fatalf("jaeger query: %v", err)
		}
		if found {
			t.Logf("multi-hop trace verified trace_id=%s services=[chat-api orchestrator]", traceID)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("timed out waiting for parent→child trace in Jaeger (chat-api → orchestrator)")
}

type jaegerTracesResponse struct {
	Data []struct {
		TraceID string `json:"traceID"`
		Spans   []struct {
			TraceID       string `json:"traceID"`
			OperationName string `json:"operationName"`
			ProcessID     string `json:"processID"`
		} `json:"spans"`
		Processes map[string]struct {
			ServiceName string `json:"serviceName"`
		} `json:"processes"`
	} `json:"data"`
}

func findMultiHopTrace(jaegerAPI, parentService, childService string) (bool, string, error) {
	url := fmt.Sprintf("%s/api/traces?service=%s&limit=20", strings.TrimRight(jaegerAPI, "/"), parentService)
	resp, err := http.Get(url)
	if err != nil {
		return false, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", err
	}
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("jaeger status %d: %s", resp.StatusCode, body)
	}

	var parsed jaegerTracesResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false, "", err
	}

	for _, trace := range parsed.Data {
		services := map[string]struct{}{}
		ops := map[string]struct{}{}
		for _, span := range trace.Spans {
			ops[span.OperationName] = struct{}{}
			if proc, ok := trace.Processes[span.ProcessID]; ok && proc.ServiceName != "" {
				services[proc.ServiceName] = struct{}{}
			}
		}
		parentOK := false
		childOK := false
		if _, ok := services[parentService]; ok {
			parentOK = true
		} else if _, ok := ops[parentService]; ok {
			parentOK = true
		}
		if _, ok := services[childService]; ok {
			childOK = true
		} else if _, ok := ops[childService]; ok {
			childOK = true
		}
		if parentOK && childOK {
			return true, trace.TraceID, nil
		}
	}
	return false, "", nil
}
