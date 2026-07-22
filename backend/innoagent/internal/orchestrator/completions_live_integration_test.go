//go:build integration

package orchestrator_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLiveToolCallingRoundTrip drives a real tool-calling exchange through a
// running orchestrator.
//
// Unit tests elsewhere talk to a stub that agrees to anything by construction,
// so they cannot show that the runtime accepts what we forward. If tool calling
// does not survive the pass, this endpoint is pointless — and only this test
// can discover that.
//
// Run with:
//
//	ORCHESTRATOR_URL=http://localhost:8080 \
//	ORCHESTRATOR_TOKEN=<valid identity token> \
//	CODEGEN_MODEL=qwen2.5-coder:1.5b \
//	go test -tags=integration ./internal/orchestrator/ -run TestLiveToolCalling -v
func TestLiveToolCallingRoundTrip(t *testing.T) {
	base := strings.TrimRight(os.Getenv("ORCHESTRATOR_URL"), "/")
	token := os.Getenv("ORCHESTRATOR_TOKEN")
	if base == "" || token == "" {
		t.Skip("ORCHESTRATOR_URL and ORCHESTRATOR_TOKEN required")
	}

	model := os.Getenv("CODEGEN_MODEL")
	if model == "" {
		model = "qwen2.5-coder:1.5b"
	}

	client := &http.Client{Timeout: 180 * time.Second}

	post := func(t *testing.T, payload map[string]any) []byte {
		t.Helper()
		body, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req, err := http.NewRequest(http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("build request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		raw := new(bytes.Buffer)
		if _, err := raw.ReadFrom(resp.Body); err != nil {
			t.Fatalf("read body: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d: %s", resp.StatusCode, raw.String())
		}
		return raw.Bytes()
	}

	tools := []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name":        "get_weather",
			"description": "Get the current weather for a city",
			"parameters": map[string]any{
				"type":       "object",
				"properties": map[string]any{"city": map[string]any{"type": "string"}},
				"required":   []string{"city"},
			},
		},
	}}

	userTurn := map[string]any{"role": "user", "content": "What is the weather in Innopolis? Use the tool."}

	// Round 1: the model should ask for the tool.
	first := post(t, map[string]any{
		"model":    model,
		"messages": []map[string]any{userTurn},
		"tools":    tools,
	})

	if !strings.Contains(string(first), "tool_calls") {
		t.Fatalf("no tool_calls in response — tool calling did not survive the pass: %s", first)
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					ID       string `json:"id"`
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(first, &parsed); err != nil {
		t.Fatalf("unmarshal choices: %v", err)
	}
	if len(parsed.Choices) == 0 || len(parsed.Choices[0].Message.ToolCalls) == 0 {
		t.Fatalf("expected at least one tool call, got: %s", first)
	}

	call := parsed.Choices[0].Message.ToolCalls[0]
	if call.Function.Name != "get_weather" {
		t.Errorf("tool name = %q, want get_weather", call.Function.Name)
	}

	// Round 2: feed the tool result back and expect a normal completion. This
	// exercises the `tool` role and tool_call_id on the request side, which the
	// old text-only /v1/chat path could not represent at all.
	second := post(t, map[string]any{
		"model": model,
		"messages": []map[string]any{
			userTurn,
			{"role": "assistant", "tool_calls": parsed.Choices[0].Message.ToolCalls},
			{"role": "tool", "tool_call_id": call.ID, "content": `{"temp_c":-7,"sky":"snow"}`},
		},
		"tools": tools,
	})

	if !strings.Contains(string(second), "choices") {
		t.Fatalf("malformed second response: %s", second)
	}
	t.Logf("final response: %s", second)
}
