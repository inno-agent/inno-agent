package orchestrator

import (
	"encoding/json"
	"errors"
	"testing"
)

// The body below deliberately carries: a tools array, tool_choice, a message
// with tool_calls, a multimodal content array, and two invented fields. None
// of it may change on the way through.
const richBody = `{"model":"qwen2.5-coder:1.5b","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]},{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"read","arguments":"{}"}}]},{"role":"tool","tool_call_id":"call_1","content":"ok"}],"tools":[{"type":"function","function":{"name":"read","parameters":{"type":"object"}}}],"tool_choice":"auto","parallel_tool_calls":true,"response_format":{"type":"json_object"},"future_field":{"nested":[1,2,3]},"another_unknown":"keep me"}`

func TestParseCompletionsPreservesEverythingButModel(t *testing.T) {
	req, err := parseCompletionsRequest([]byte(richBody))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	req.setModel("replacement-model")

	out, err := req.marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got, want map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if err := json.Unmarshal([]byte(richBody), &want); err != nil {
		t.Fatalf("unmarshal source: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("key count = %d, want %d (got %v)", len(got), len(want), keysOf(got))
	}

	for k, wantVal := range want {
		gotVal, ok := got[k]
		if !ok {
			t.Errorf("key %q dropped", k)
			continue
		}
		if k == "model" {
			if string(gotVal) != `"replacement-model"` {
				t.Errorf("model = %s, want \"replacement-model\"", gotVal)
			}
			continue
		}
		if string(gotVal) != string(wantVal) {
			t.Errorf("key %q changed:\n got %s\nwant %s", k, gotVal, wantVal)
		}
	}
}

func keysOf(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestParseCompletionsReadsModelAndStream(t *testing.T) {
	req, err := parseCompletionsRequest([]byte(`{"model":"m1","stream":true,"messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if req.model != "m1" {
		t.Errorf("model = %q, want m1", req.model)
	}
	if !req.stream {
		t.Error("stream = false, want true")
	}
}

func TestParseCompletionsRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		body string
		want error
	}{
		{"invalid json", `{not json`, ErrInvalidBody},
		{"messages missing", `{"model":"m"}`, ErrEmptyMessages},
		{"messages empty", `{"model":"m","messages":[]}`, ErrEmptyMessages},
		{"messages not an array", `{"model":"m","messages":{"role":"user"}}`, ErrInvalidBody},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseCompletionsRequest([]byte(tt.body))
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRouterMessagesDecodesRolesAndText(t *testing.T) {
	req, err := parseCompletionsRequest([]byte(`{"model":"auto","messages":[{"role":"user","content":"hello"},{"role":"assistant","content":[{"type":"text","text":"ignored"}]}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	msgs := req.routerMessages()
	if len(msgs) != 2 {
		t.Fatalf("len = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hello" {
		t.Errorf("msgs[0] = %+v", msgs[0])
	}
	// Non-string content is not decodable into a plain string; it must degrade
	// to empty rather than fail the whole request.
	if msgs[1].Role != "assistant" || msgs[1].Content != "" {
		t.Errorf("msgs[1] = %+v", msgs[1])
	}
}
