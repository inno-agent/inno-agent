package catalog

import "testing"

func TestLoad(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Models) != 3 {
		t.Fatalf("want 3 models, got %d", len(c.Models))
	}
	if c.Default != "qwen2.5:0.5b" {
		t.Fatalf("want default qwen2.5:0.5b, got %q", c.Default)
	}
}
