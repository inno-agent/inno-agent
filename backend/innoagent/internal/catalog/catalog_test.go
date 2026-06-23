package catalog

import "testing"

func TestLoad_AutoPrependedAsDefault(t *testing.T) {
	c, err := Load([]string{"llama3.2:1b", "qwen2.5:0.5b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// "auto" is always prepended + 2 explicit IDs = 3 models.
	if len(c.Models) != 3 {
		t.Fatalf("want 3 models, got %d", len(c.Models))
	}
	if c.Models[0].ID != "auto" {
		t.Fatalf("first model must be auto, got %q", c.Models[0].ID)
	}
	if c.Default != "auto" {
		t.Fatalf("default must be auto, got %q", c.Default)
	}
}

func TestLoad_OrderPreservedAfterAuto(t *testing.T) {
	c, err := Load([]string{"llama3.2:1b", "qwen2.5:0.5b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Models[1].ID != "llama3.2:1b" {
		t.Fatalf("order not preserved after auto: %+v", c.Models)
	}
	if c.Models[2].ID != "qwen2.5:0.5b" {
		t.Fatalf("order not preserved after auto: %+v", c.Models)
	}
}

func TestLoad_EnrichesFromRegistry(t *testing.T) {
	c, err := Load([]string{"qwen2.5:0.5b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// First model is "auto", second is "qwen2.5:0.5b" enriched as "Fast".
	if c.Models[1].Name != "Fast" {
		t.Fatalf("want registry name Fast, got %q", c.Models[1].Name)
	}
}

func TestLoad_UnknownIDFallsBackToID(t *testing.T) {
	c, err := Load([]string{"mystery:7b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// "mystery:7b" is after "auto", so it's c.Models[1].
	if c.Models[1].Name != "mystery:7b" {
		t.Fatalf("unknown id should use id as name, got %q", c.Models[1].Name)
	}
}

func TestLoad_AutoNotDuplicated(t *testing.T) {
	// Even if "auto" appears in the IDs list, it must not be duplicated.
	c, err := Load([]string{"auto", "qwen2.5:0.5b"})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	autoCount := 0
	for _, m := range c.Models {
		if m.ID == "auto" {
			autoCount++
		}
	}
	if autoCount != 1 {
		t.Fatalf("auto should appear exactly once, got %d", autoCount)
	}
}
