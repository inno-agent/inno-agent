package gitflame

import (
	"encoding/json"
	"testing"
)

func TestCreateBranchRequestJSON(t *testing.T) {
	payload, err := json.Marshal(createBranchRequest{
		NewBranchName: "innoagent-issue-2",
		OldBranchName: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["new_branch_name"] != "innoagent-issue-2" {
		t.Fatalf("new_branch_name = %q", got["new_branch_name"])
	}
	if got["old_branch_name"] != "main" {
		t.Fatalf("old_branch_name = %q", got["old_branch_name"])
	}
}

func TestCreateFileRequestJSON(t *testing.T) {
	payload, err := json.Marshal(createFileRequest{
		Message:   "init",
		Content:   "Zm9v",
		Branch:    "main",
		NewBranch: "innoagent-issue-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["new_branch"] != "innoagent-issue-2" {
		t.Fatalf("new_branch = %q", got["new_branch"])
	}
	if _, ok := got["new_branch_name"]; ok {
		t.Fatal("should not use new_branch_name for contents API")
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
