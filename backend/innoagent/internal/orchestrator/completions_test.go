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

	if req.model != "replacement-model" {
		t.Errorf("req.model = %q, want replacement-model", req.model)
	}

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

func TestRouterMessagesSkipsNonObjects(t *testing.T) {
	// Non-object messages (e.g. integers, nulls) are skipped when decoding for
	// routing. This is safe because routerMessages() output affects only model
	// selection; the request forwarded upstream comes from r.raw (untouched
	// bytes), so no content is lost from the perspective of the actual LLM call.
	//
	// This matters for the route() fallback: route() filters to user messages,
	// and if none remain, uses messages[len(messages)-1]. If non-objects were
	// appended as zero-values, the fallback would see empty instead of real
	// content (e.g., a system message), breaking routing decisions.

	req, err := parseCompletionsRequest([]byte(`{"model":"auto","messages":[{"role":"user","content":"hi"},42]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	msgs := req.routerMessages()
	if len(msgs) != 1 {
		t.Fatalf("len = %d, want 1 (non-object messages must be skipped)", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "hi" {
		t.Errorf("msgs[0] = %+v", msgs[0])
	}
}

// TestRouterMessagesPreservesLastRealMessageForRouterFallback pins the specific
// regression that motivated the skip: route() falls back to the LAST message
// when no user-role message is present, so a trailing non-object must not
// become the last element or the router classifies against empty content.
//
// This lives in its own function on purpose. Folded into the test above as a
// second case, a t.Fatalf on the earlier assertion aborts before this one runs,
// so it could never fail under the very mutation it exists to catch.
func TestRouterMessagesPreservesLastRealMessageForRouterFallback(t *testing.T) {
	req, err := parseCompletionsRequest([]byte(`{"model":"auto","messages":[{"role":"system","content":"you are a Go expert, answer about goroutines"},42]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	msgs := req.routerMessages()
	last := msgs[len(msgs)-1]
	if last.Role != "system" || last.Content == "" {
		t.Errorf("last message = %+v, want the system message with its content", last)
	}
}
