package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed models.json
var metadataJSON []byte

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Catalog struct {
	Models  []Model `json:"models"`
	Default string  `json:"default"`
}

// Load builds the catalog for the given model IDs (order preserved), enriching
// each from the embedded metadata registry. LLM_MODELS is the single source of
// truth for which models exist; models.json only supplies display metadata.
// The first ID is the default. An ID without a registry entry falls back to its
// own ID as the display name.
func Load(ids []string) (*Catalog, error) {
	var reg struct {
		Models []Model `json:"models"`
	}
	if err := json.Unmarshal(metadataJSON, &reg); err != nil {
		return nil, fmt.Errorf("catalog: unmarshal metadata: %w", err)
	}
	meta := make(map[string]Model, len(reg.Models))
	for _, m := range reg.Models {
		meta[m.ID] = m
	}

	c := &Catalog{}
	for _, id := range ids {
		if m, ok := meta[id]; ok {
			c.Models = append(c.Models, m)
		} else {
			c.Models = append(c.Models, Model{ID: id, Name: id})
		}
	}
	if len(c.Models) > 0 {
		c.Default = c.Models[0].ID
	}
	return c, nil
}
