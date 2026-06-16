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
	if c.Default != "llama3.2:3b" {
		t.Fatalf("want default llama3.2:3b, got %q", c.Default)
	}
}

func TestFilter(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	f := c.Filter([]string{"qwen2.5-coder:7b"})
	if len(f.Models) != 1 || f.Models[0].ID != "qwen2.5-coder:7b" {
		t.Fatalf("filter failed: %+v", f.Models)
	}
	if f.Default != "qwen2.5-coder:7b" {
		t.Fatalf("filtered default should fall back to first allowed, got %q", f.Default)
	}
}

func TestFilterNilAllowsAll(t *testing.T) {
	c, _ := Load()
	f := c.Filter(nil)
	if len(f.Models) != len(c.Models) {
		t.Fatalf("nil allowlist must return full catalog")
	}
}
