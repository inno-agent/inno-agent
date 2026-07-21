package llm

import (
	"testing"
	"time"
)

// The constructor's timeout guard is invisible from outside the package: it
// ends up in an unexported field of an unexported struct field. An in-package
// test reaches it without exporting anything or restructuring the type.
//
// This matters because the two fallbacks must agree with the config defaults.
// A guard falling back to qwen.go's 120s while config supplies 180s would give
// the misuse path a TIGHTER bound than production — long tool-calling
// generations would work when wired from config and time out when the client
// is constructed directly.
func TestNewCompletionsClientGuardsMatchConfigDefaults(t *testing.T) {
	tests := []struct {
		name         string
		timeout      time.Duration
		maxRespBytes int64
	}{
		{"zero", 0, 0},
		{"negative", -1 * time.Second, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCompletionsClient("http://example/v1", "", tt.timeout, tt.maxRespBytes)

			if c.httpClient.Timeout != defaultCompletionsTimeout {
				t.Errorf("timeout = %v, want %v", c.httpClient.Timeout, defaultCompletionsTimeout)
			}
			if c.maxRespBytes != defaultMaxResponseBytes {
				t.Errorf("maxRespBytes = %d, want %d", c.maxRespBytes, defaultMaxResponseBytes)
			}
		})
	}
}

// defaultCompletionsTimeout must equal the config package's default for
// LLM_COMPLETIONS_TIMEOUT. They live in different packages with no compile-time
// link, so this pins the number itself.
func TestDefaultCompletionsTimeoutIs180s(t *testing.T) {
	if defaultCompletionsTimeout != 180*time.Second {
		t.Errorf("defaultCompletionsTimeout = %v, want 180s (must match config.Load)", defaultCompletionsTimeout)
	}
}
