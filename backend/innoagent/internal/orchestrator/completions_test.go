package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"innoagent/internal/catalog"
	"innoagent/internal/llm"

	"go.uber.org/zap"
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

// newTestOrchestrator builds an orchestrator whose allowlist is exactly
// {"m1","m2"} and whose router always answers "m2".
func newTestOrchestrator() *AIOrchestrator {
	router := &mockProvider{chatResponses: map[string]string{"": `{"route":"m2"}`}}
	return New(&mockProvider{}, router, defaultRoutes(), []string{"m1", "m2"}, zap.NewNop())
}

func TestResolveCompletionsModelRejectsUnknownModel(t *testing.T) {
	o := newTestOrchestrator()
	req, err := parseCompletionsRequest([]byte(`{"model":"nope","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := o.prepareCompletions(context.Background(), req); !errors.Is(err, ErrModelNotAllowed) {
		t.Fatalf("err = %v, want ErrModelNotAllowed", err)
	}
}

// The router model is deliberately absent from LLM_MODELS. A client naming it
// must be refused, or it can drive the internal routing model directly.
func TestResolveCompletionsModelRejectsRouterModel(t *testing.T) {
	o := newTestOrchestrator()
	req, err := parseCompletionsRequest([]byte(`{"model":"arch-router:1.5b","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := o.prepareCompletions(context.Background(), req); !errors.Is(err, ErrModelNotAllowed) {
		t.Fatalf("err = %v, want ErrModelNotAllowed", err)
	}
}

func TestResolveCompletionsRejectsStreaming(t *testing.T) {
	o := newTestOrchestrator()
	req, err := parseCompletionsRequest([]byte(`{"model":"m1","stream":true,"messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := o.prepareCompletions(context.Background(), req); !errors.Is(err, ErrStreamUnsupported) {
		t.Fatalf("err = %v, want ErrStreamUnsupported", err)
	}
}

func TestResolveCompletionsAutoSubstitutesRoutedModel(t *testing.T) {
	o := newTestOrchestrator()
	req, err := parseCompletionsRequest([]byte(`{"model":"` + catalog.AutoID + `","messages":[{"role":"user","content":"write go code"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := o.prepareCompletions(context.Background(), req); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if req.model != "m2" {
		t.Errorf("model = %q, want m2 (router choice)", req.model)
	}

	out, err := req.marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Model != "m2" {
		t.Errorf("outgoing model = %q, want m2", decoded.Model)
	}
}

func TestResolveCompletionsEmptyModelUsesDefault(t *testing.T) {
	o := newTestOrchestrator()
	req, err := parseCompletionsRequest([]byte(`{"messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := o.prepareCompletions(context.Background(), req); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if req.model != "m1" {
		t.Errorf("model = %q, want m1 (first of LLM_MODELS)", req.model)
	}
}

// The shared-context seam is a no-op today. This test pins that: P3 will change
// the expectation deliberately, and until then nothing may silently start
// rewriting client messages.
func TestInjectSharedContextIsCurrentlyANoOp(t *testing.T) {
	o := newTestOrchestrator()
	req, err := parseCompletionsRequest([]byte(richBody))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	before, err := req.marshal()
	if err != nil {
		t.Fatalf("marshal before: %v", err)
	}

	if err := o.injectSharedContext(context.Background(), req); err != nil {
		t.Fatalf("inject: %v", err)
	}

	after, err := req.marshal()
	if err != nil {
		t.Fatalf("marshal after: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("request mutated:\nbefore %s\nafter  %s", before, after)
	}
	if len(req.messages) != 3 {
		t.Errorf("message count = %d, want 3", len(req.messages))
	}
}

// stubCompleter stands in for the llm.CompletionsClient.
type stubCompleter struct {
	result   *llm.CompletionsResult
	err      error
	lastBody []byte
}

func (s *stubCompleter) Complete(ctx context.Context, body []byte) (*llm.CompletionsResult, error) {
	s.lastBody = body
	return s.result, s.err
}

func TestCompleteForwardsAndReturnsUpstreamSuccess(t *testing.T) {
	// richBody names a real model, so this orchestrator's allowlist has to
	// contain it — newTestOrchestrator only allows m1/m2.
	o := New(&mockProvider{}, &mockProvider{}, defaultRoutes(), []string{"qwen2.5-coder:1.5b"}, zap.NewNop())
	upstreamBody := []byte(`{"choices":[{"message":{"tool_calls":[{"id":"c1"}]}}]}`)
	stub := &stubCompleter{result: &llm.CompletionsResult{
		Status:      200,
		Body:        upstreamBody,
		ContentType: "application/json",
	}}
	o.SetCompleter(stub)

	res, err := o.Complete(context.Background(), []byte(richBody))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != 200 {
		t.Errorf("status = %d, want 200", res.Status)
	}
	if string(res.Body) != string(upstreamBody) {
		t.Errorf("body = %s, want %s", res.Body, upstreamBody)
	}

	// The tools array must reach the runtime untouched; that is the whole point.
	var sent map[string]json.RawMessage
	if err := json.Unmarshal(stub.lastBody, &sent); err != nil {
		t.Fatalf("unmarshal forwarded body: %v", err)
	}
	var want map[string]json.RawMessage
	if err := json.Unmarshal([]byte(richBody), &want); err != nil {
		t.Fatalf("unmarshal source: %v", err)
	}
	if string(sent["tools"]) != string(want["tools"]) {
		t.Errorf("tools changed:\n got %s\nwant %s", sent["tools"], want["tools"])
	}
	if string(sent["messages"]) != string(want["messages"]) {
		t.Errorf("messages changed:\n got %s\nwant %s", sent["messages"], want["messages"])
	}
}

func TestCompletePassesUpstream4xxThrough(t *testing.T) {
	o := newTestOrchestrator()
	upstreamBody := []byte(`{"error":{"message":"bad tools schema"}}`)
	o.SetCompleter(&stubCompleter{result: &llm.CompletionsResult{
		Status:      400,
		Body:        upstreamBody,
		ContentType: "application/json",
	}})

	res, err := o.Complete(context.Background(), []byte(`{"model":"m1","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != 400 {
		t.Errorf("status = %d, want 400", res.Status)
	}
	// 4xx bodies pass through verbatim: they are almost always a complaint
	// about the tools schema and are undebuggable without the original text.
	if string(res.Body) != string(upstreamBody) {
		t.Errorf("body = %s, want %s", res.Body, upstreamBody)
	}
}

func TestCompleteCollapsesUpstream5xx(t *testing.T) {
	o := newTestOrchestrator()
	o.SetCompleter(&stubCompleter{result: &llm.CompletionsResult{
		Status:      500,
		Body:        []byte(`{"error":"dial tcp 10.0.0.5:11434 refused"}`),
		ContentType: "application/json",
	}})

	res, err := o.Complete(context.Background(), []byte(`{"model":"m1","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != 502 {
		t.Errorf("status = %d, want 502", res.Status)
	}
	if strings.Contains(string(res.Body), "10.0.0.5") {
		t.Errorf("upstream internals leaked to client: %s", res.Body)
	}
}

func TestCompleteMapsTimeoutTo504(t *testing.T) {
	o := newTestOrchestrator()
	o.SetCompleter(&stubCompleter{err: context.DeadlineExceeded})

	res, err := o.Complete(context.Background(), []byte(`{"model":"m1","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != 504 {
		t.Errorf("status = %d, want 504", res.Status)
	}
}

func TestCompleteMapsOversizedResponseTo502(t *testing.T) {
	o := newTestOrchestrator()
	o.SetCompleter(&stubCompleter{err: llm.ErrResponseTooLarge})

	res, err := o.Complete(context.Background(), []byte(`{"model":"m1","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != 502 {
		t.Errorf("status = %d, want 502", res.Status)
	}
}

func TestCompletePropagatesPreflightErrors(t *testing.T) {
	o := newTestOrchestrator()
	o.SetCompleter(&stubCompleter{})

	if _, err := o.Complete(context.Background(), []byte(`{"model":"nope","messages":[{"role":"user","content":"x"}]}`)); !errors.Is(err, ErrModelNotAllowed) {
		t.Fatalf("err = %v, want ErrModelNotAllowed", err)
	}
}

func TestCompleteReportsRequestedAndResolvedModel(t *testing.T) {
	o := newTestOrchestrator()
	o.SetCompleter(&stubCompleter{result: &llm.CompletionsResult{
		Status: 200, Body: []byte(`{}`), ContentType: "application/json",
	}})

	res, err := o.Complete(context.Background(), []byte(`{"model":"`+catalog.AutoID+`","messages":[{"role":"user","content":"x"}]}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// Attribution needs both: "auto" is what the client asked for, "m2" is what
	// the router actually picked.
	if res.RequestedModel != catalog.AutoID {
		t.Errorf("RequestedModel = %q, want %q", res.RequestedModel, catalog.AutoID)
	}
	if res.ResolvedModel != "m2" {
		t.Errorf("ResolvedModel = %q, want m2", res.ResolvedModel)
	}
}
