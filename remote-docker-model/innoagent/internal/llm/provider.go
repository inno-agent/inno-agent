// Package llm defines the core abstractions for LLM provider integration
// in the InnoAgent AI Orchestrator.
//
// The package follows dependency inversion: high-level orchestration code
// depends on the Provider interface, not on concrete implementations.
// This enables easy swapping of providers (Qwen, OpenAI, Mistral, etc.)
// without touching orchestrator logic.
package llm

import "context"

// Provider is the top-level abstraction for any LLM backend.
// All provider implementations must satisfy this interface.
type Provider interface {
	// Chat sends a user message and returns the model's text response.
	// The caller is responsible for supplying a context that carries
	// deadlines, cancellation signals, and request-scoped values.
	//
	// Returns a non-nil error on network failure, non-2xx HTTP status,
	// or malformed response from the upstream API.
	Chat(ctx context.Context, message string) (string, error)
}
