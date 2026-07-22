package gitflame

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
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

// Bug #1: Test PushFiles can update existing files
func TestPushFiles_UpdatesExistingFile(t *testing.T) {
	var (
		getFileCalled  bool
		putFileCalled  bool
		postFileCalled bool
	)

	server := newMockGiteaServer(t, func(method, path string, body []byte) (statusCode int, respBody []byte) {
		if strings.Contains(path, "/contents/") {
			if method == "GET" {
				getFileCalled = true
				// File exists, return sha
				return 200, []byte(`{"sha":"abc123def456"}`)
			}
			if method == "PUT" {
				putFileCalled = true
				// Verify the request body contains the sha
				var req updateFileRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal PUT body: %v", err)
				}
				if req.Sha != "abc123def456" {
					t.Fatalf("expected sha abc123def456, got %s", req.Sha)
				}
				// Update succeeds
				return 200, []byte(`{"sha":"newsha789"}`)
			}
			if method == "POST" {
				postFileCalled = true
				// Create succeeds
				return 201, []byte(`{"sha":"newsha"}`)
			}
		}
		if strings.Contains(path, "/branches") {
			// Branch creation/existence check
			return 201, []byte(`{}`)
		}
		return 500, []byte("unexpected request")
	})
	defer server.Close()

	c := NewClient(server.URL, "test-token")
	ref := domain.IssueRef{Owner: "test-owner", Repo: "test-repo", DefaultBranch: "main"}
	files := []domain.GeneratedFile{{Path: "test.txt", Content: "hello"}}

	err := c.PushFiles(context.Background(), ref, "innoagent-test", files, "test commit")
	if err != nil {
		t.Fatalf("PushFiles failed: %v", err)
	}

	if !getFileCalled {
		t.Fatal("GET /contents/{path} was not called")
	}
	if !putFileCalled {
		t.Fatal("PUT /contents/{path} was not called for existing file")
	}
	if postFileCalled {
		t.Fatal("POST /contents/{path} should not be called when file exists")
	}
}

// A create racing another writer to the same path (issue reassignment
// retries, redelivered webhooks) gets 409 AlreadyExistNameError even though
// the preceding getFileSha saw 404. PushFiles must retry as an update instead
// of surfacing the race as a permanent failure that drops the whole push.
func TestPushFiles_CreateConflict_RetriesAsUpdate(t *testing.T) {
	var getCalls, postCalls, putCalls int

	server := newMockGiteaServer(t, func(method, path string, body []byte) (statusCode int, respBody []byte) {
		if strings.Contains(path, "/contents/") {
			switch method {
			case "GET":
				getCalls++
				if getCalls == 1 {
					return 404, []byte(`{"message":"not found"}`)
				}
				return 200, []byte(`{"sha":"racedsha"}`)
			case "POST":
				postCalls++
				return 409, []byte(`{"message":"[go.mod]","code":"AlreadyExistNameError"}`)
			case "PUT":
				putCalls++
				var req updateFileRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal PUT body: %v", err)
				}
				if req.Sha != "racedsha" {
					t.Fatalf("expected sha racedsha, got %s", req.Sha)
				}
				return 200, []byte(`{"sha":"finalsha"}`)
			}
		}
		if strings.Contains(path, "/branches") {
			return 201, []byte(`{}`)
		}
		return 500, []byte("unexpected request")
	})
	defer server.Close()

	c := NewClient(server.URL, "test-token")
	ref := domain.IssueRef{Owner: "test-owner", Repo: "test-repo", DefaultBranch: "main"}
	files := []domain.GeneratedFile{{Path: "go.mod", Content: "module test"}}

	if err := c.PushFiles(context.Background(), ref, "innoagent-test", files, "test commit"); err != nil {
		t.Fatalf("PushFiles failed: %v", err)
	}
	if postCalls != 1 {
		t.Fatalf("expected 1 POST attempt, got %d", postCalls)
	}
	if putCalls != 1 {
		t.Fatalf("expected 1 retry PUT, got %d", putCalls)
	}
	if getCalls != 2 {
		t.Fatalf("expected initial GET + retry GET, got %d", getCalls)
	}
}

// Two concurrent Process() runs for the same reassigned issue (each
// redelivery gets a fresh delivery ID, so dedup doesn't stop this) can both
// fetch the same file's sha, then race to PUT — the loser's sha is stale by
// the time its request lands and GitFlame answers 409/422. PushFiles must
// re-fetch the now-current sha and retry once rather than dropping the push.
func TestPushFiles_UpdateConflict_RetriesWithFreshSha(t *testing.T) {
	var getCalls, putCalls int

	server := newMockGiteaServer(t, func(method, path string, body []byte) (statusCode int, respBody []byte) {
		if strings.Contains(path, "/contents/") {
			switch method {
			case "GET":
				getCalls++
				if getCalls == 1 {
					return 200, []byte(`{"sha":"stalesha"}`)
				}
				return 200, []byte(`{"sha":"freshsha"}`)
			case "PUT":
				putCalls++
				var req updateFileRequest
				if err := json.Unmarshal(body, &req); err != nil {
					t.Fatalf("failed to unmarshal PUT body: %v", err)
				}
				if putCalls == 1 {
					if req.Sha != "stalesha" {
						t.Fatalf("expected first PUT to use stalesha, got %s", req.Sha)
					}
					return 409, []byte(`{"message":"[go.mod]","code":"AlreadyExistNameError"}`)
				}
				if req.Sha != "freshsha" {
					t.Fatalf("expected retry PUT to use freshsha, got %s", req.Sha)
				}
				return 200, []byte(`{"sha":"finalsha"}`)
			}
		}
		if strings.Contains(path, "/branches") {
			return 201, []byte(`{}`)
		}
		return 500, []byte("unexpected request")
	})
	defer server.Close()

	c := NewClient(server.URL, "test-token")
	ref := domain.IssueRef{Owner: "test-owner", Repo: "test-repo", DefaultBranch: "main"}
	files := []domain.GeneratedFile{{Path: "go.mod", Content: "module test"}}

	if err := c.PushFiles(context.Background(), ref, "innoagent-test", files, "test commit"); err != nil {
		t.Fatalf("PushFiles failed: %v", err)
	}
	if putCalls != 2 {
		t.Fatalf("expected initial PUT + retry PUT, got %d", putCalls)
	}
	if getCalls != 2 {
		t.Fatalf("expected initial GET + retry GET, got %d", getCalls)
	}
}

// Bug #2: Test "not configured" errors are tagged as permanent
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

// Helper function to create a mock Gitea API server for testing
func newMockGiteaServer(t *testing.T, handler func(method, path string, body []byte) (statusCode int, respBody []byte)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 0)
		if r.Body != nil {
			b := make([]byte, 8192)
			n, _ := r.Body.Read(b)
			body = b[:n]
			r.Body.Close()
		}

		statusCode, respBody := handler(r.Method, r.URL.Path, body)
		w.WriteHeader(statusCode)
		_, _ = w.Write(respBody)
	}))
}
