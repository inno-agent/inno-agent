package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
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
