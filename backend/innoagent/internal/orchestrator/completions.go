package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"innoagent/internal/catalog"
	"innoagent/internal/llm"

	"go.uber.org/zap"
)

// Sentinel errors for pre-flight rejections. The HTTP handler maps these to
// status codes; see the error table in the design spec.
var (
	ErrInvalidBody       = errors.New("orchestrator: invalid request body")
	ErrEmptyMessages     = errors.New("orchestrator: messages field is required")
	ErrStreamUnsupported = errors.New("orchestrator: streaming is not supported")
	ErrModelNotAllowed   = errors.New("orchestrator: model not allowed")
)

// completionsRequest holds an OpenAI-shaped chat-completions body with only
// the fields the orchestrator governs decoded.
//
// The opacity guarantee lives in this type: `raw` keeps every top-level key as
// undecoded bytes and `messages` keeps every element as undecoded bytes, so
// tools, tool_choice, tool_calls, multimodal content and any field added to the
// schema later cannot be lost — they are never parsed in the first place.
type completionsRequest struct {
	raw      map[string]json.RawMessage
	messages []json.RawMessage
	model    string
	stream   bool
}

func parseCompletionsRequest(body []byte) (*completionsRequest, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBody, err)
	}

	rawMessages, ok := raw["messages"]
	if !ok {
		return nil, ErrEmptyMessages
	}

	var messages []json.RawMessage
	if err := json.Unmarshal(rawMessages, &messages); err != nil {
		return nil, fmt.Errorf("%w: messages must be an array: %v", ErrInvalidBody, err)
	}
	if len(messages) == 0 {
		return nil, ErrEmptyMessages
	}

	req := &completionsRequest{raw: raw, messages: messages}

	if rawModel, ok := raw["model"]; ok {
		if err := json.Unmarshal(rawModel, &req.model); err != nil {
			return nil, fmt.Errorf("%w: model must be a string: %v", ErrInvalidBody, err)
		}
	}

	if rawStream, ok := raw["stream"]; ok {
		if err := json.Unmarshal(rawStream, &req.stream); err != nil {
			return nil, fmt.Errorf("%w: stream must be a boolean: %v", ErrInvalidBody, err)
		}
	}

	return req, nil
}

func (r *completionsRequest) setModel(model string) {
	if r.raw == nil {
		r.raw = make(map[string]json.RawMessage, 1)
	}
	encoded, _ := json.Marshal(model) // a Go string always marshals
	r.raw["model"] = encoded
	r.model = model
}

// marshal re-encodes the request body. Note: top-level keys are sorted
// alphabetically and any duplicate keys collapse with last-wins. However,
// individual message elements and unknown field values are byte-exact copies,
// which preserves the opacity guarantee. The output is not byte-identical to
// the input as a whole, so it should not be assumed for hashing, signing, or
// caching the raw body.
func (r *completionsRequest) marshal() ([]byte, error) {
	return json.Marshal(r.raw)
}

// routerMessages decodes roles and text for the router only. It is called
// solely when model == "auto"; on an explicit model the message array is never
// decoded. Messages whose content is not a plain string (multimodal parts,
// tool_calls with null content) degrade to empty text rather than failing the
// request. Elements that are not JSON objects are skipped — this affects only
// the model selection decision. The request forwarded upstream comes from r.raw
// (untouched bytes), so no part of the original message array is ever lost; the
// opacity guarantee is unaffected by this filtering.
func (r *completionsRequest) routerMessages() []llm.Message {
	out := make([]llm.Message, 0, len(r.messages))
	for _, raw := range r.messages {
		var m struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		var content string
		if len(m.Content) > 0 {
			_ = json.Unmarshal(m.Content, &content)
		}
		out = append(out, llm.Message{Role: m.Role, Content: content})
	}
	return out
}

// modelAllowed reports whether the client may request this model.
//
// The allowlist is o.models, which comes from LLM_MODELS. ROUTER_MODEL is a
// separate setting and is deliberately not in that list, so naming the routing
// model is refused here. Checking against the catalog instead would be a bug:
// the catalog carries display metadata, not permission.
func (o *AIOrchestrator) modelAllowed(model string) bool {
	if model == "" || model == catalog.AutoID {
		return true
	}
	for _, m := range o.models {
		if m == model {
			return true
		}
	}
	return false
}

// prepareCompletions validates and rewrites the request in place, leaving it
// ready to forward. It rejects streaming and disallowed models, and resolves
// "auto" (and the empty model) to a concrete one.
func (o *AIOrchestrator) prepareCompletions(ctx context.Context, req *completionsRequest) error {
	if req.stream {
		return ErrStreamUnsupported
	}
	if !o.modelAllowed(req.model) {
		return fmt.Errorf("%w: %q", ErrModelNotAllowed, req.model)
	}

	var resolved string
	switch req.model {
	case catalog.AutoID:
		resolved = o.route(ctx, req.routerMessages())
	case "":
		resolved = o.models[0]
	default:
		resolved = req.model
	}

	req.setModel(resolved)
	return nil
}

// injectSharedContext is the extension point for cross-service user context
// (P3): platform instructions and remembered user preferences, prepended as a
// system message.
//
// It is a deliberate no-op today, and it is the ONLY place allowed to modify
// the message array. Note what it must do when implemented: prepend a newly
// built raw element, never re-encode an existing one. Round-tripping a client
// message through a Go struct would silently drop tool_calls and multimodal
// content, which is exactly what the opacity design exists to prevent.
func (o *AIOrchestrator) injectSharedContext(ctx context.Context, req *completionsRequest) error {
	return nil
}

// Completer is the transport the orchestrator forwards prepared bodies to.
// llm.CompletionsClient satisfies it; tests substitute a stub.
type Completer interface {
	Complete(ctx context.Context, body []byte) (*llm.CompletionsResult, error)
}

// SetCompleter installs the transport. Kept separate from New so the existing
// constructor signature — and every caller of it — stays untouched.
func (o *AIOrchestrator) SetCompleter(c Completer) {
	o.completer = c
}

// CompleteResult is what the HTTP handler writes back. RequestedModel and
// ResolvedModel differ when the client asked for "auto" or omitted the field;
// the handler logs both for attribution.
type CompleteResult struct {
	Status         int
	Body           []byte
	ContentType    string
	RequestedModel string
	ResolvedModel  string
}

// genericUpstreamError is returned in place of an upstream 5xx body, which may
// carry internal hostnames and paths.
var genericUpstreamError = []byte(`{"error":{"message":"upstream model runtime error","type":"upstream_error"}}`)

// Complete parses, prepares and forwards a chat-completions request.
//
// Pre-flight rejections come back as sentinel errors for the handler to map.
// Once the request reaches the runtime, every outcome is expressed as a
// CompleteResult so the client always receives a body.
func (o *AIOrchestrator) Complete(ctx context.Context, body []byte) (*CompleteResult, error) {
	req, err := parseCompletionsRequest(body)
	if err != nil {
		return nil, err
	}
	requested := req.model

	if err := o.prepareCompletions(ctx, req); err != nil {
		return nil, err
	}
	if err := o.injectSharedContext(ctx, req); err != nil {
		return nil, fmt.Errorf("orchestrator: inject shared context: %w", err)
	}

	outbound, err := req.marshal()
	if err != nil {
		return nil, fmt.Errorf("orchestrator: marshal completions request: %w", err)
	}

	res, err := o.completer.Complete(ctx, outbound)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		o.logger.Error("completions upstream call failed",
			zap.String("model", req.model),
			zap.Error(err))
		return &CompleteResult{
			Status:         status,
			Body:           genericUpstreamError,
			ContentType:    "application/json",
			RequestedModel: requested,
			ResolvedModel:  req.model,
		}, nil
	}

	// Client errors from the runtime are almost always a complaint about the
	// tools schema and are undebuggable without the original text, so they pass
	// through. Server errors may carry internal addresses, so the client gets a
	// generic body and the detail goes to the log.
	if res.Status >= 500 {
		o.logger.Error("completions upstream returned server error",
			zap.String("model", req.model),
			zap.Int("upstream_status", res.Status),
			zap.ByteString("upstream_body", res.Body))
		return &CompleteResult{
			Status:         http.StatusBadGateway,
			Body:           genericUpstreamError,
			ContentType:    "application/json",
			RequestedModel: requested,
			ResolvedModel:  req.model,
		}, nil
	}

	return &CompleteResult{
		Status:         res.Status,
		Body:           res.Body,
		ContentType:    res.ContentType,
		RequestedModel: requested,
		ResolvedModel:  req.model,
	}, nil
}
