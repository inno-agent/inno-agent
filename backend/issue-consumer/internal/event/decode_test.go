package event

import (
	"encoding/json"
	"testing"
)

func TestDecodeIssuePayload_Object(t *testing.T) {
	payload, _ := json.Marshal(IssueEvent{
		Action: "assigned",
		Number: 2,
	})
	issueEv, err := DecodeIssuePayload(payload)
	if err != nil {
		t.Fatal(err)
	}
	if issueEv.Action != "assigned" || issueEv.Number != 2 {
		t.Fatalf("unexpected issue: %+v", issueEv)
	}
}

func TestDecodeIssuePayload_StringEncoded(t *testing.T) {
	inner, _ := json.Marshal(IssueEvent{
		Action: "assigned",
		Number: 6,
	})
	encoded, _ := json.Marshal(string(inner))

	issueEv, err := DecodeIssuePayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if issueEv.Action != "assigned" || issueEv.Number != 6 {
		t.Fatalf("unexpected issue: %+v", issueEv)
	}
}

func TestDecodeIssuePayload_NestedEnvelope(t *testing.T) {
	inner, _ := json.Marshal(IssueEvent{
		Action: "opened",
		Number: 3,
	})
	env := Envelope{
		EventType: "issues",
		Payload:   inner,
	}
	raw, _ := json.Marshal(env)

	issueEv, err := DecodeIssuePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if issueEv.Action != "opened" || issueEv.Number != 3 {
		t.Fatalf("unexpected issue: %+v", issueEv)
	}
}

func TestDecodeIssuePayload_RichTextBody(t *testing.T) {
	raw := []byte(`{
		"action": "assigned",
		"number": 5,
		"issue": {
			"number": 5,
			"title": "Fix bug",
			"body": [{"type":"paragraph","content":"Do the thing"}]
		},
		"repository": {"name": "repo", "full_name": "org/repo"},
		"assignee": {"login": "bot"}
	}`)

	issueEv, err := DecodeIssuePayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if issueEv.IssueBody() != "Do the thing" {
		t.Fatalf("body = %q", issueEv.IssueBody())
	}
}

func TestIssueEvent_RepoOwnerFromFullName(t *testing.T) {
	raw := []byte(`{
		"action": "assigned",
		"number": 9,
		"issue": {"number": 9, "title": "Task", "body": "details"},
		"repository": {"name": "myrepo", "full_name": "myorg/myrepo"},
		"assignee": {"login": "bot"}
	}`)

	var issueEv IssueEvent
	if err := json.Unmarshal(raw, &issueEv); err != nil {
		t.Fatal(err)
	}
	if issueEv.RepoOwner() != "myorg" {
		t.Fatalf("owner = %q", issueEv.RepoOwner())
	}
	if issueEv.RepoName() != "myrepo" {
		t.Fatalf("repo = %q", issueEv.RepoName())
	}
}

func TestIssueEvent_IssueIndexUsesNestedIndexField(t *testing.T) {
	raw := []byte(`{
		"action": "assigned",
		"issue": {"index": 11, "title": "Task", "body": "details"},
		"repository": {"full_name": "org/repo"},
		"assignee": {"login": "bot"}
	}`)

	var issueEv IssueEvent
	if err := json.Unmarshal(raw, &issueEv); err != nil {
		t.Fatal(err)
	}
	if issueEv.IssueIndex() != 11 {
		t.Fatalf("index = %d", issueEv.IssueIndex())
	}
}

func TestIsIssueEventType(t *testing.T) {
	for _, eventType := range []string{"", "issues", "issue", "issue_assign"} {
		if !IsIssueEventType(eventType) {
			t.Fatalf("expected %q to match", eventType)
		}
	}
	if IsIssueEventType("pull_request") {
		t.Fatal("pull_request should not match")
	}
}
