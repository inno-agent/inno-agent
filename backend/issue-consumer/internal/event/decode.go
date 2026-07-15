package event

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func DecodeIssuePayload(raw json.RawMessage) (IssueEvent, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return IssueEvent{}, fmt.Errorf("empty payload")
	}

	if raw[0] == '"' {
		var encoded string
		if err := json.Unmarshal(raw, &encoded); err != nil {
			return IssueEvent{}, err
		}
		return DecodeIssuePayload(json.RawMessage(encoded))
	}

	var nested struct {
		EventType string          `json:"event_type"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &nested); err != nil {
		return IssueEvent{}, err
	}
	if len(nested.Payload) > 0 && nested.EventType != "" && IsIssueEventType(nested.EventType) {
		if inner, err := DecodeIssuePayload(nested.Payload); err == nil && inner.IssueIndex() != 0 {
			return inner, nil
		}
	}

	var issueEv IssueEvent
	if err := json.Unmarshal(raw, &issueEv); err != nil {
		return IssueEvent{}, err
	}
	return issueEv, nil
}

func IsIssueEventType(eventType string) bool {
	switch eventType {
	case "", "issues", "issue", "issue_assign":
		return true
	default:
		return false
	}
}
