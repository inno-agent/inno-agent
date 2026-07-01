package llm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
)

// ConcurrencyLimiter bounds concurrent LLM requests to prevent
// goroutine explosion and Ollama overload.
type ConcurrencyLimiter struct {
	sem chan struct{}
}

// NewLimiter creates a limiter allowing at most max concurrent requests.
func NewLimiter(max int) *ConcurrencyLimiter {
	if max <= 0 {
		max = 16
	}
	return &ConcurrencyLimiter{sem: make(chan struct{}, max)}
}

func (l *ConcurrencyLimiter) Acquire() { l.sem <- struct{}{} }
func (l *ConcurrencyLimiter) Release() { <-l.sem }

// RequestCoalescer deduplicates identical in-flight requests.
// If two callers submit the same prompt simultaneously, the second
// waits for the first result instead of sending a duplicate to Ollama.
type RequestCoalescer struct {
	inflight sync.Map
}

type coalescedResult struct {
	content string
	err     error
}

// HashMessages produces a deterministic key for a request.
func HashMessages(messages []Message, model string) string {
	type entry struct {
		Role    string `json:"r"`
		Content string `json:"c"`
	}
	entries := make([]entry, len(messages))
	for i, m := range messages {
		entries[i] = entry{Role: m.Role, Content: m.Content}
	}
	payload, _ := json.Marshal(entries)
	h := sha256.Sum256(append(payload, []byte(model)...))
	return fmt.Sprintf("%x", h[:16])
}

// Do executes fn unless an identical request is already in flight,
// in which case it waits for that result.
func (c *RequestCoalescer) Do(key string, fn func() (string, error)) (string, error) {
	ch := make(chan coalescedResult, 1)
	if actual, loaded := c.inflight.LoadOrStore(key, ch); loaded {
		result := <-actual.(chan coalescedResult)
		return result.content, result.err
	}
	content, err := fn()
	ch <- coalescedResult{content: content, err: err}
	c.inflight.Delete(key)
	return content, err
}
