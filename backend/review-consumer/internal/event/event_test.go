package event_test

import (
	"encoding/json"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/event"
)

func TestPullRequestEvent_Decode(t *testing.T) {
	raw := `{
		"action": "opened",
		"number": 7,
		"pull_request": {"head": {"ref": "feat/foo", "sha": "cafebabe"}},
		"repository": {"name": "myrepo", "full_name": "myorg/myrepo", "owner": {"login": "myorg"}}
	}`

	var pr event.PullRequestEvent
	if err := json.Unmarshal([]byte(raw), &pr); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if pr.Action != "opened" {
		t.Errorf("action: got %q", pr.Action)
	}
	if pr.Number != 7 {
		t.Errorf("number: got %d", pr.Number)
	}
	if pr.PullRequest.Head.SHA != "cafebabe" {
		t.Errorf("head.sha: got %q", pr.PullRequest.Head.SHA)
	}
	if pr.PullRequest.Head.Ref != "feat/foo" {
		t.Errorf("head.ref: got %q", pr.PullRequest.Head.Ref)
	}
	if pr.Repository.Owner.Login != "myorg" {
		t.Errorf("owner.login: got %q", pr.Repository.Owner.Login)
	}
	if pr.Repository.Name != "myrepo" {
		t.Errorf("repo name: got %q", pr.Repository.Name)
	}
}

func TestPullRequestEvent_Index_FallsBackToTopLevel(t *testing.T) {
	// No pull_request.number -> fall back to top-level number.
	raw := `{
		"action": "opened",
		"number": 42,
		"pull_request": {"head": {"ref": "feat/bar", "sha": "aabbcc"}},
		"repository": {"name": "r", "owner": {"login": "o"}}
	}`
	var pr event.PullRequestEvent
	if err := json.Unmarshal([]byte(raw), &pr); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if pr.Index() != 42 {
		t.Errorf("expected 42, got %d", pr.Index())
	}
}

func TestPullRequestEvent_Index_PrefersNested(t *testing.T) {
	// pull_request.number set -> prefer it over top-level number.
	raw := `{
		"action": "opened",
		"number": 1,
		"pull_request": {"number": 99, "head": {"ref": "feat/bar", "sha": "aabbcc"}},
		"repository": {"name": "r", "owner": {"login": "o"}}
	}`
	var pr event.PullRequestEvent
	if err := json.Unmarshal([]byte(raw), &pr); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if pr.Index() != 99 {
		t.Errorf("expected 99, got %d", pr.Index())
	}
}

func TestEnvelope_Decode(t *testing.T) {
	raw := `{"delivery_id":"abc","event_type":"pull_request","payload":{"action":"opened"}}`

	var env event.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if env.DeliveryID != "abc" {
		t.Errorf("delivery_id: got %q", env.DeliveryID)
	}
	if env.EventType != "pull_request" {
		t.Errorf("event_type: got %q", env.EventType)
	}
	if string(env.Payload) != `{"action":"opened"}` {
		t.Errorf("payload: got %q", string(env.Payload))
	}
}
