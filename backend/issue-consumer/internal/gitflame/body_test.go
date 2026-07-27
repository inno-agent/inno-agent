package gitflame

import (
	"testing"
)

// Bug #3: Test ParseIssueBody handles object-shaped JSON (single object with type/content/text)
func TestParseIssueBody_ObjectShape(t *testing.T) {
	raw := []byte(`{"type":"doc","content":"hello world"}`)
	got := ParseIssueBody(raw)
	if got != "hello world" {
		t.Fatalf("got %q, want %q", got, "hello world")
	}
}

func TestParseIssueBody_ObjectShape_WithText(t *testing.T) {
	raw := []byte(`{"type":"paragraph","text":"extracted text"}`)
	got := ParseIssueBody(raw)
	if got != "extracted text" {
		t.Fatalf("got %q, want %q", got, "extracted text")
	}
}

func TestParseIssueBody_ObjectShape_EmptyFallsBack(t *testing.T) {
	// If object doesn't have content or text, fall through to raw JSON
	raw := []byte(`{"type":"doc","other":"value"}`)
	got := ParseIssueBody(raw)
	// Should return raw JSON (not parse it as object)
	if got != `{"type":"doc","other":"value"}` {
		t.Fatalf("got %q, want raw JSON", got)
	}
}
