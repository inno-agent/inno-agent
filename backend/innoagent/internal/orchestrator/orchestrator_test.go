package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"innoagent/internal/llm"

	"go.uber.org/zap"
)

// mockProvider records calls and returns configurable responses.
type mockProvider struct {
	chatResponses map[string]string // modelName → response
	streamCh      chan string
	lastChatModel string
	lastMessages  []llm.Message
}

func (m *mockProvider) Chat(ctx context.Context, messages []llm.Message, modelName string) (string, error) {
	m.lastChatModel = modelName
	m.lastMessages = messages

	if modelName == "" && len(m.chatResponses) > 0 {
		// Use the first key as default response.
		for k := range m.chatResponses {
			modelName = k
			break
		}
	}

	resp, ok := m.chatResponses[modelName]
	if !ok {
		return "", fmt.Errorf("no mock response for model %q", modelName)
	}
	return resp, nil
}

func (m *mockProvider) Stream(ctx context.Context, messages []llm.Message, modelName string) (<-chan string, error) {
	m.lastChatModel = modelName
	ch := make(chan string, 1)
	go func() {
		ch <- "stream-chunk"
		close(ch)
	}()
	return ch, nil
}

func defaultRoutes() []RouteInfo {
	return []RouteInfo{
		{Name: "qwen2.5:0.5b", Description: "Tiny model, fastest responses"},
		{Name: "llama3.2:1b", Description: "General Q&A and conversation"},
		{Name: "qwen2.5-coder:1.5b", Description: "Programming, debugging, code generation"},
		{Name: "qwen2.5-coder-32b", Description: "Advanced code review and generation"},
	}
}

func defaultModels() []string {
	return []string{"qwen2.5:0.5b", "llama3.2:1b", "qwen2.5-coder:1.5b", "qwen2.5-coder-32b"}
}

func defaultVLLMModels() map[string]bool {
	return map[string]bool{
		"qwen2.5-coder-32b": true,
	}
}

func testLogger() *zap.Logger {
	return zap.NewNop()
}

func TestResolveModel_AutoRoutesSuccessfully(t *testing.T) {
	// Route to the middle model — clearly NOT models[0] fallback.
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": `{"route": "llama3.2:1b"}`,
		},
	}
	// Responses for ALL models so the test never fails on "no mock response".
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b":       "fast answer",
			"llama3.2:1b":        "general answer",
			"qwen2.5-coder:1.5b": "code answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "tell me about history"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "general answer" {
		t.Fatalf("want 'general answer' (from llama3.2:1b), got %q", answer)
	}
	if mainProvider.lastChatModel != "llama3.2:1b" {
		t.Fatalf("want llama3.2:1b (not fallback), got %q", mainProvider.lastChatModel)
	}
}

func TestResolveModel_AutoSendsArchRouterFormat(t *testing.T) {
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": `{"route": "llama3.2:1b"}`,
		},
	}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"llama3.2:1b": "general answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	_, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "system", Content: "you are helpful"},
		{Role: "user", Content: "tell me a joke"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	prompt := routerProvider.lastMessages[0].Content

	// Verify the prompt contains <routes> and <conversation> tags.
	if !strings.Contains(prompt, "<routes>") {
		t.Error("router prompt missing <routes> tag")
	}
	if !strings.Contains(prompt, "</routes>") {
		t.Error("router prompt missing </routes> tag")
	}
	if !strings.Contains(prompt, "<conversation>") {
		t.Error("router prompt missing <conversation> tag")
	}
	if !strings.Contains(prompt, "</conversation>") {
		t.Error("router prompt missing </conversation> tag")
	}

	// Verify routes JSON contains model descriptions.
	if !strings.Contains(prompt, "Programming, debugging, code generation") {
		t.Error("router prompt missing model description for coder model")
	}

	// Verify conversation only contains user messages (system filtered).
	if strings.Contains(prompt, `"role":"system"`) {
		t.Error("router prompt should not contain system messages")
	}
	if !strings.Contains(prompt, `"role":"user"`) {
		t.Error("router prompt should contain user messages")
	}
}

func TestResolveModel_AutoFallbackOnInvalidJSON(t *testing.T) {
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": "I think llama3.2 would be best", // not JSON
		},
	}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fallback answer", // models[0] is the fallback
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "fallback answer" {
		t.Fatalf("want fallback answer from models[0], got %q", answer)
	}
	if mainProvider.lastChatModel != "qwen2.5:0.5b" {
		t.Fatalf("fallback should use models[0], got %q", mainProvider.lastChatModel)
	}
}

func TestResolveModel_AutoHandlesSingleQuotedJSON(t *testing.T) {
	// arch-router returns single-quoted JSON: {'route': 'model'}
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": "{'route': 'llama3.2:1b'}",
		},
	}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b":       "fast answer",
			"llama3.2:1b":        "general answer",
			"qwen2.5-coder:1.5b": "code answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "tell me a joke"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "general answer" {
		t.Fatalf("want 'general answer' (from llama3.2:1b), got %q", answer)
	}
	if mainProvider.lastChatModel != "llama3.2:1b" {
		t.Fatalf("want llama3.2:1b, got %q", mainProvider.lastChatModel)
	}
}

func TestResolveModel_AutoFallbackOnUnknownRoute(t *testing.T) {
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": `{"route": "gpt-4"}`, // not in our model list
		},
	}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fallback answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "fallback answer" {
		t.Fatalf("want fallback from models[0], got %q", answer)
	}
}

func TestResolveModel_AutoFallbackOnRouterError(t *testing.T) {
	routerProvider := &mockProvider{
		chatResponses: map[string]string{}, // empty = all calls return error
	}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fallback answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask should not fail on router error, got: %v", err)
	}
	if answer != "fallback answer" {
		t.Fatalf("want fallback from models[0], got %q", answer)
	}
}

func TestResolveModel_EmptyModelNameUsesDefault(t *testing.T) {
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "default answer",
		},
	}
	routerProvider := &mockProvider{}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "default answer" {
		t.Fatalf("want default answer, got %q", answer)
	}
	if mainProvider.lastChatModel != "qwen2.5:0.5b" {
		t.Fatalf("empty model should use models[0], got %q", mainProvider.lastChatModel)
	}
}

func TestResolveModel_ConcreteModelBypassesRouter(t *testing.T) {
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"llama3.2:1b": "general answer",
		},
	}
	routerProvider := &mockProvider{}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "llama3.2:1b")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "general answer" {
		t.Fatalf("want general answer, got %q", answer)
	}
	// Router should never have been called.
	if len(routerProvider.lastMessages) != 0 {
		t.Fatalf("router should not be called for explicit model, got %d messages", len(routerProvider.lastMessages))
	}
}

func TestResolveModel_NeverLoopsBackToAuto(t *testing.T) {
	// Simulate router returning "auto" as a route — must not create a loop.
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": `{"route": "auto"}`,
		},
	}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fallback answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	// "auto" is not in models list, so it falls back to models[0].
	if mainProvider.lastChatModel != "qwen2.5:0.5b" {
		t.Fatalf("router returned 'auto' which is not a concrete model, should fallback to models[0], got %q", mainProvider.lastChatModel)
	}
	if answer != "fallback answer" {
		t.Fatalf("want fallback answer, got %q", answer)
	}
}

func TestResolveModel_AutoEmptyMessagesFallsBack(t *testing.T) {
	// route() must not panic on an empty message slice — it falls back to
	// models[0] without ever calling the router.
	routerProvider := &mockProvider{}
	mainProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fallback answer",
		},
	}

	orch := New(mainProvider, mainProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "fallback answer" {
		t.Fatalf("want fallback answer, got %q", answer)
	}
	if mainProvider.lastChatModel != "qwen2.5:0.5b" {
		t.Fatalf("empty messages should fall back to models[0], got %q", mainProvider.lastChatModel)
	}
	if len(routerProvider.lastMessages) != 0 {
		t.Fatalf("router must not be called on empty messages, got %d messages", len(routerProvider.lastMessages))
	}
}

// ─── Integration tests for dual-provider routing ──────────────────────────────

func TestDualProvider_CodeModelRoutesToVLLM(t *testing.T) {
	ollamaProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fast answer",
		},
	}
	vllmProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5-coder-32b": "code answer",
		},
	}
	routerProvider := &mockProvider{}

	orch := New(ollamaProvider, vllmProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "review this PR"},
	}, "qwen2.5-coder-32b")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "code answer" {
		t.Fatalf("want 'code answer' from vLLM, got %q", answer)
	}
	// Verify vLLM provider was called, not Ollama
	if vllmProvider.lastChatModel != "qwen2.5-coder-32b" {
		t.Fatalf("vLLM provider should have been called, got model %q", vllmProvider.lastChatModel)
	}
	if ollamaProvider.lastChatModel != "" {
		t.Fatalf("Ollama provider should NOT have been called for code model, got %q", ollamaProvider.lastChatModel)
	}
}

func TestDualProvider_FastModelRoutesToOllama(t *testing.T) {
	ollamaProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b": "fast answer",
		},
	}
	vllmProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5-coder-32b": "code answer",
		},
	}
	routerProvider := &mockProvider{}

	orch := New(ollamaProvider, vllmProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "hello"},
	}, "qwen2.5:0.5b")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if answer != "fast answer" {
		t.Fatalf("want 'fast answer' from Ollama, got %q", answer)
	}
	// Verify Ollama provider was called, not vLLM
	if ollamaProvider.lastChatModel != "qwen2.5:0.5b" {
		t.Fatalf("Ollama provider should have been called, got model %q", ollamaProvider.lastChatModel)
	}
	if vllmProvider.lastChatModel != "" {
		t.Fatalf("vLLM provider should NOT have been called for fast model, got %q", vllmProvider.lastChatModel)
	}
}

func TestDualProvider_AutoRoutesCodeToVLLM(t *testing.T) {
	ollamaProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5:0.5b":       "fast answer",
			"qwen2.5-coder-32b":  "code answer from ollama (wrong)",
		},
	}
	vllmProvider := &mockProvider{
		chatResponses: map[string]string{
			"qwen2.5-coder-32b": "code answer from vLLM",
		},
	}
	routerProvider := &mockProvider{
		chatResponses: map[string]string{
			"fauxpaslife/arch-router:1.5b": `{"route": "qwen2.5-coder-32b"}`,
		},
	}

	orch := New(ollamaProvider, vllmProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	answer, err := orch.Ask(context.Background(), []llm.Message{
		{Role: "user", Content: "review this code for bugs"},
	}, "auto")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	// Router picks code model, orchestrator routes to vLLM
	if answer != "code answer from vLLM" {
		t.Fatalf("want 'code answer from vLLM', got %q", answer)
	}
	if vllmProvider.lastChatModel != "qwen2.5-coder-32b" {
		t.Fatalf("vLLM should handle code model, got %q", vllmProvider.lastChatModel)
	}
}

func TestDualProvider_StreamCodeModelViaVLLM(t *testing.T) {
	ollamaProvider := &mockProvider{
		chatResponses: map[string]string{},
	}
	vllmProvider := &mockProvider{
		chatResponses: map[string]string{},
	}
	routerProvider := &mockProvider{}

	orch := New(ollamaProvider, vllmProvider, routerProvider, defaultRoutes(), defaultModels(), defaultVLLMModels(), testLogger())

	ch, err := orch.AskStream(context.Background(), []llm.Message{
		{Role: "user", Content: "review this PR"},
	}, "qwen2.5-coder-32b")
	if err != nil {
		t.Fatalf("AskStream: %v", err)
	}

	// Consume the stream
	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk from stream")
	}

	// Verify vLLM was used for streaming, not Ollama
	if vllmProvider.lastChatModel != "qwen2.5-coder-32b" {
		t.Fatalf("vLLM should handle streaming for code model, got %q", vllmProvider.lastChatModel)
	}
	if ollamaProvider.lastChatModel != "" {
		t.Fatalf("Ollama provider should NOT have been called for streaming code model, got %q", ollamaProvider.lastChatModel)
	}
}
