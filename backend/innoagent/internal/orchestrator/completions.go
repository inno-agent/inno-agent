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
	encoded, err := json.Marshal(model)
	if err != nil {
		// A Go string always marshals; this branch is unreachable in practice.
		return
	}
	r.raw["model"] = encoded
	r.model = model
}

func (r *completionsRequest) marshal() ([]byte, error) {
	return json.Marshal(r.raw)
}

// routerMessages decodes roles and text for the router only. It is called
// solely when model == "auto"; on an explicit model the message array is never
// decoded. Messages whose content is not a plain string (multimodal parts,
// tool_calls with null content) degrade to empty text rather than failing the
// request — the router only needs something to classify.
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
