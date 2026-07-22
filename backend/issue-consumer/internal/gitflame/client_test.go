package gitflame

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
)

func TestCreatePullRequestRequestJSON(t *testing.T) {
	payload, err := json.Marshal(createPullRequestRequest{
		Title: "meow",
		From:  "innoagent-issue-10",
		To:    "main",
		Body:  buildPRBody("ehehhehehe"),
	})
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["title"] != "meow" {
		t.Fatalf("title = %v", got["title"])
	}
	if got["from"] != "innoagent-issue-10" {
		t.Fatalf("from = %v", got["from"])
	}
	if got["to"] != "main" {
		t.Fatalf("to = %v", got["to"])
	}
	if _, ok := got["head"]; ok {
		t.Fatal("head should not be sent on create")
	}
	if _, ok := got["base"]; ok {
		t.Fatal("base should not be sent on create")
	}
	if _, ok := got["reviewers"]; ok {
		t.Fatal("reviewers should not be sent on create")
	}

	body, ok := got["body"].([]any)
	if !ok || len(body) != 1 {
		t.Fatalf("body = %v", got["body"])
	}
	block, ok := body[0].(map[string]any)
	if !ok {
		t.Fatalf("body block = %T", body[0])
	}
	if block["mime"] != "text" {
		t.Fatalf("mime = %v", block["mime"])
	}
	if block["size"] != float64(1) {
		t.Fatalf("size = %v", block["size"])
	}
	if block["name"] != "text" {
		t.Fatalf("name = %v", block["name"])
	}
	if block["type"] != "text" {
		t.Fatalf("type = %v", block["type"])
	}
	if block["body"] != "<p>ehehhehehe</p>" {
		t.Fatalf("body = %v", block["body"])
	}
}

func TestBuildPRBody_Empty(t *testing.T) {
	if got := buildPRBody(""); got != nil {
		t.Fatalf("got %v", got)
	}
}

func TestBuildPRBody_Multiline(t *testing.T) {
	got := buildPRBody("line one\nline two")
	if len(got) != 1 {
		t.Fatalf("got %d blocks", len(got))
	}
	if got[0].Body != "<p>line one<br>line two</p>" {
		t.Fatalf("body = %q", got[0].Body)
	}
}

func TestParseIssueBody_String(t *testing.T) {
	got := ParseIssueBody([]byte(`"hello world"`))
	if got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestParseIssueBody_ArrayBlocks(t *testing.T) {
	raw := []byte(`[{"type":"paragraph","content":"line one"},{"type":"paragraph","content":"line two"}]`)
	got := ParseIssueBody(raw)
	want := "line one\nline two"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseIssueBody_Null(t *testing.T) {
	if got := ParseIssueBody([]byte("null")); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestParsePullRequestNumber(t *testing.T) {
	if got := parsePullRequestNumber([]byte(`{"number":15}`)); got != 15 {
		t.Fatalf("number = %d", got)
	}
	if got := parsePullRequestNumber([]byte(`{"index":9}`)); got != 9 {
		t.Fatalf("index = %d", got)
	}
	if got := parsePullRequestNumber(nil); got != 0 {
		t.Fatalf("empty = %d", got)
	}
}

func TestParseExistingPRNumber(t *testing.T) {
	raw := []byte(`{"message":"GetUnmergedPullRequest: pull request already exists for these targets [id: 1084, issue_id: 15, head_branch: innoagent-issue-14, base_branch: main]","code":"AlreadyExistNameError"}`)
	if got := parseExistingPRNumber(raw); got != 15 {
		t.Fatalf("issue_id = %d", got)
	}
	if got := parseExistingPRNumber([]byte(`{"message":"nope"}`)); got != 0 {
		t.Fatalf("missing = %d", got)
	}
}

func TestGetIssue_NotConfigured(t *testing.T) {
	c := NewClient("", "")
	_, _, err := c.GetIssue(context.Background(), domain.IssueRef{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("error %v is not domain.ErrPermanent", err)
	}
}

func TestCreatePullRequest_NotConfigured(t *testing.T) {
	c := NewClient("", "")
	_, err := c.CreatePullRequest(context.Background(), domain.IssueRef{}, "", "", "", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("error %v is not domain.ErrPermanent", err)
	}
}

// linkPullRequestToIssue's PATCH is a full-replace, not a merge (confirmed
// from a real GitFlame network trace): it must round-trip every editable
// field it read via GET, or a "harmless" dependency link silently wipes the
// issue's title/body/assignees/labels/milestone. This is the load-bearing
// assertion — every field below must come back unchanged except dependencies.
func TestLinkPullRequestToIssue_PreservesFieldsAndMergesDependencies(t *testing.T) {
	var gotPatch map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"title": "Add DELETE /tasks/{id} and a completion toggle",
				"body": [{"id": 2208, "body": "<p>desc</p>", "mime": "text", "size": 1, "name": "", "type": "text"}],
				"assignees": [{"login": "innoagent"}],
				"labels": [{"id": 3}],
				"milestone": null,
				"dependencies": [{"id": 5700, "number": 5, "title": "old dep", "state": "open"}]
			}`))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &gotPatch); err != nil {
				t.Fatalf("PATCH body not JSON: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-token")
	err := c.linkPullRequestToIssue(context.Background(), domain.IssueRef{Owner: "askarr", Repo: "pyfile", Index: 6}, 7)
	if err != nil {
		t.Fatalf("linkPullRequestToIssue: %v", err)
	}

	if gotPatch["title"] != "Add DELETE /tasks/{id} and a completion toggle" {
		t.Errorf("title changed: %v", gotPatch["title"])
	}
	if gotPatch["milestone"] != float64(0) {
		t.Errorf("milestone = %v, want 0 (null milestone round-trips to 0)", gotPatch["milestone"])
	}
	assignees, _ := gotPatch["assignees"].([]any)
	if len(assignees) != 1 || assignees[0] != "innoagent" {
		t.Errorf("assignees = %v, want [innoagent]", gotPatch["assignees"])
	}
	labels, _ := gotPatch["labels"].([]any)
	if len(labels) != 1 || labels[0] != float64(3) {
		t.Errorf("labels = %v, want [3]", gotPatch["labels"])
	}
	body, _ := gotPatch["body"].([]any)
	if len(body) != 1 {
		t.Fatalf("body = %v, want 1 block", gotPatch["body"])
	}
	block, _ := body[0].(map[string]any)
	if _, hasID := block["id"]; hasID {
		t.Error("body block still has response-only id field")
	}
	if block["body"] != "<p>desc</p>" {
		t.Errorf("body text changed: %v", block["body"])
	}

	deps, _ := gotPatch["dependencies"].([]any)
	got := map[float64]bool{}
	for _, d := range deps {
		got[d.(float64)] = true
	}
	if !got[5] || !got[7] || len(got) != 2 {
		t.Errorf("dependencies = %v, want [5, 7]", deps)
	}
}

// Re-linking an already-linked PR (issue reassignment retries the whole run)
// must not duplicate the dependency entry.
func TestLinkPullRequestToIssue_AlreadyLinked_NoDuplicate(t *testing.T) {
	var gotPatch map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"title": "t", "body": [], "assignees": [], "labels": [], "milestone": null,
				"dependencies": [{"id": 5737, "number": 7, "title": "already linked", "state": "open"}]
			}`))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotPatch)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-token")
	if err := c.linkPullRequestToIssue(context.Background(), domain.IssueRef{Owner: "o", Repo: "r", Index: 6}, 7); err != nil {
		t.Fatalf("linkPullRequestToIssue: %v", err)
	}

	deps, _ := gotPatch["dependencies"].([]any)
	if len(deps) != 1 || deps[0] != float64(7) {
		t.Errorf("dependencies = %v, want exactly [7] (no duplicate)", deps)
	}
}
