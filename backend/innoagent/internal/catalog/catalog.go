package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed models.json
var metadataJSON []byte

// AutoID is the canonical ID of the synthetic "auto" routing option. It is the
// single source of truth shared with the orchestrator, which treats a request
// for this model as a trigger to run the router.
const AutoID = "auto"

type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Catalog struct {
	Models  []Model `json:"models"`
	Default string  `json:"default"`

	byID map[string]Model
}

// Load builds the catalog for the given model IDs (order preserved), enriching
// each from the embedded metadata registry. LLM_MODELS is the single source of
// truth for which models exist; models.json only supplies display metadata.
// "auto" is always prepended as the first (default) option — it triggers the
// router to select the best model automatically. The router model itself is
// never exposed in the catalog.
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

	// "auto" is always present. models.json may override its display metadata;
	// otherwise we synthesize a default so the option never depends on the
	// registry entry existing.
	autoEntry, ok := meta[AutoID]
	if !ok {
		autoEntry = Model{
			ID:          AutoID,
			Name:        "Auto",
			Description: "Automatically selects the best model for your query",
		}
	}

	c := &Catalog{}
	c.Models = append(c.Models, autoEntry)
	for _, id := range ids {
		if id == AutoID {
			continue
		}
		if m, ok := meta[id]; ok {
			c.Models = append(c.Models, m)
		} else {
			c.Models = append(c.Models, Model{ID: id, Name: id})
		}
	}
	if len(c.Models) > 0 {
		c.Default = c.Models[0].ID
	}
	c.byID = make(map[string]Model, len(c.Models))
	for _, m := range c.Models {
		c.byID[m.ID] = m
	}
	return c, nil
}

// Description returns the display description for the given model ID, or the ID
// itself when the model is unknown.
func (c *Catalog) Description(id string) string {
	if m, ok := c.byID[id]; ok {
		return m.Description
	}
	return id
}
