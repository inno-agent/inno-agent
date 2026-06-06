package llm

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyResponse is returned when the model returns a valid HTTP 200
	// but the choices array is empty.
	ErrEmptyResponse = errors.New("llm: model returned an empty response")

	// ErrEmptyMessage is returned when the caller passes an empty message string.
	ErrEmptyMessage = errors.New("llm: message must not be empty")
)

// ProviderError wraps an upstream HTTP error with status code and body detail.
// Callers can use errors.As to inspect the code and message.
type ProviderError struct {
	// StatusCode is the HTTP status code returned by the upstream API.
	StatusCode int

	// Message is the human-readable error text extracted from the response body,
	// or a raw excerpt of the body if JSON decoding failed.
	Message string
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	return fmt.Sprintf("llm: upstream API error (HTTP %d %s): %s",
		e.StatusCode, http.StatusText(e.StatusCode), e.Message)
}

// newProviderError constructs a ProviderError from raw HTTP status and message.
func newProviderError(statusCode int, message string) *ProviderError {
	return &ProviderError{
		StatusCode: statusCode,
		Message:    message,
	}
}
