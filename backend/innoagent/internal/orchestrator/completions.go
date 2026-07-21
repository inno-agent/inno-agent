package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"

	"innoagent/internal/llm"
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
// decoded. Every message element produces exactly one output message: objects
// decode to Role and Content (with non-string content degrading to empty);
// non-objects degrade to a zero-value message. This ensures the request is
// never silently truncated — the router only needs something to classify.
func (r *completionsRequest) routerMessages() []llm.Message {
	out := make([]llm.Message, 0, len(r.messages))
	for _, raw := range r.messages {
		msg := llm.Message{}
		var m struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(raw, &m); err == nil {
			var content string
			if len(m.Content) > 0 {
				_ = json.Unmarshal(m.Content, &content)
			}
			msg = llm.Message{Role: m.Role, Content: content}
		}
		out = append(out, msg)
	}
	return out
}
