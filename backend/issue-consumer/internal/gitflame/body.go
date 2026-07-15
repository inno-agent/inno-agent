package gitflame

import (
	"encoding/json"
	"strings"
)

// ParseIssueBody converts a GitFlame issue body field to plain text.
// The API and webhooks may return a plain string or rich-text JSON blocks.
func ParseIssueBody(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var blocks []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			switch {
			case b.Content != "":
				parts = append(parts, b.Content)
			case b.Text != "":
				parts = append(parts, b.Text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}

	return string(raw)
}
