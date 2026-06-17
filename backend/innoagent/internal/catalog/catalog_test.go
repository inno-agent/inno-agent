package catalog

import "testing"

func TestLoad_OrderAndDefaultFromIDs(t *testing.T) {
	c, err := Load([]string{"llama3.2:1b", "qwen2.5:0.5b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Models) != 2 {
		t.Fatalf("want 2 models, got %d", len(c.Models))
	}
	if c.Models[0].ID != "llama3.2:1b" {
		t.Fatalf("order not preserved: %+v", c.Models)
	}
	if c.Default != "llama3.2:1b" {
		t.Fatalf("default must be first ID, got %q", c.Default)
	}
}

func TestLoad_EnrichesFromRegistry(t *testing.T) {
	c, err := Load([]string{"qwen2.5:0.5b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Models[0].Name != "Fast" {
		t.Fatalf("want registry name Fast, got %q", c.Models[0].Name)
	}
}

func TestLoad_UnknownIDFallsBackToID(t *testing.T) {
	c, err := Load([]string{"mystery:7b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Models[0].Name != "mystery:7b" {
		t.Fatalf("unknown id should use id as name, got %q", c.Models[0].Name)
	}
}
