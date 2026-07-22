package gitflame

import (
	"context"
	"encoding/json"
	"errors"
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
