package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

//go:embed models.json
var modelsJSON []byte

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Catalog struct {
	Models  []Model `json:"models"`
	Default string  `json:"default"`
}

// Load parses the embedded catalog.
func Load() (*Catalog, error) {
	var c Catalog
	if err := json.Unmarshal(modelsJSON, &c); err != nil {
		return nil, fmt.Errorf("catalog: unmarshal: %w", err)
	}
	if c.Default == "" && len(c.Models) > 0 {
		c.Default = c.Models[0].ID
	}
	return &c, nil
}

// Filter returns a catalog containing only models whose ID is in allowed.
// A nil allowed slice means "no policy restriction" and returns the full catalog.
func (c *Catalog) Filter(allowed []string) *Catalog {
	if allowed == nil {
		return c
	}
	out := &Catalog{}
	for _, m := range c.Models {
		if slices.Contains(allowed, m.ID) {
			out.Models = append(out.Models, m)
		}
	}
	out.Default = c.Default
	if !slices.ContainsFunc(out.Models, func(m Model) bool { return m.ID == out.Default }) && len(out.Models) > 0 {
		out.Default = out.Models[0].ID
	}
	return out
}
