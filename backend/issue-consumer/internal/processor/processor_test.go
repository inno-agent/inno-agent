package processor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/event"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/processor"
)

const (
	testBotUsername   = "innoagent"
	testOnboardingURL = "https://review.example.com/onboard"
)

type fakeGenerator struct {
	result *domain.GenerationResult
	err    error
	calls  int
}

func (f *fakeGenerator) Generate(_ context.Context, _ domain.IssueRef) (*domain.GenerationResult, error) {
	f.calls++
	return f.result, f.err
}

type fakePusher struct {
	branches  []string
	reviewers []string
	prIndex   int64
	err       error
	prErr     error
}

func (f *fakePusher) PushFiles(_ context.Context, _ domain.IssueRef, branch string, _ []domain.GeneratedFile, _ string) error {
	if f.err != nil {
		return f.err
	}
	f.branches = append(f.branches, branch)
	return nil
}

func (f *fakePusher) CreatePullRequest(_ context.Context, _ domain.IssueRef, _ string, _, _ string, reviewers []string) (int64, error) {
	if f.prErr != nil {
		return 0, f.prErr
	}
	f.reviewers = reviewers
	if f.prIndex != 0 {
		return f.prIndex, nil
	}
	return 1, nil
}

type fakePoster struct {
	posted []string
	err    error
}

func (f *fakePoster) PostIssueComment(_ context.Context, _ domain.IssueRef, body string) error {
	if f.err != nil {
		return f.err
	}
	f.posted = append(f.posted, body)
	return nil
}

func newProc(gen domain.Generator, publisher *fakePusher, poster domain.CommentPoster) *processor.Processor {
	return processor.New(gen, publisher, publisher, poster, zap.NewNop(), testBotUsername, testOnboardingURL)
}

func makeEnvelope(action, deliveryID string) []byte {
	issue := event.IssueEvent{
		Action: action,
		Number: 7,
	}
	issue.Issue.Number = 7
	issue.Issue.Title = "Add health endpoint"
	issue.Issue.Body = json.RawMessage(`"Please add GET /health"`)
	issue.Issue.Assignee.Login = testBotUsername
	issue.Issue.User.Login = "alice"
	issue.Repository.Name = "myrepo"
	issue.Repository.Owner.Login = "myorg"
	issue.Repository.DefaultBranch = "main"
	issue.Sender.Login = "alice"

	payload, _ := json.Marshal(issue)
	env := event.Envelope{
		DeliveryID: deliveryID,
		EventType:  "issues",
		Payload:    payload,
	}
	data, _ := json.Marshal(env)
	return data
}

func TestProcess_WrongAction_Skip(t *testing.T) {
	issue := event.IssueEvent{Action: "closed", Number: 1}
	issue.Issue.Assignee.Login = testBotUsername
	payload, _ := json.Marshal(issue)
	env := event.Envelope{EventType: "issues", Payload: payload}
	data, _ := json.Marshal(env)

	gen := &fakeGenerator{}
	p := newProc(gen, &fakePusher{}, &fakePoster{})
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}
	if gen.calls != 0 {
		t.Fatal("generator should not be called")
	}
}

func TestProcess_NotAssignedToBot_Skip(t *testing.T) {
	issue := event.IssueEvent{Action: "assigned", Number: 1}
	issue.Issue.Assignee.Login = "other-user"
	payload, _ := json.Marshal(issue)
	env := event.Envelope{EventType: "issues", Payload: payload}
	data, _ := json.Marshal(env)

	gen := &fakeGenerator{}
	p := newProc(gen, &fakePusher{}, &fakePoster{})
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}
	if gen.calls != 0 {
		t.Fatal("generator should not be called")
	}
}

func TestProcess_Assigned_Done(t *testing.T) {
	data := makeEnvelope("assigned", "del-1")
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Summary: "added endpoint",
		Files:   []domain.GeneratedFile{{Path: "health.go", Content: "package main"}},
	}}
	pusher := &fakePusher{}
	poster := &fakePoster{}
	p := newProc(gen, pusher, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Done {
		t.Fatalf("expected Done, got %v", result)
	}
	if len(pusher.branches) != 1 || pusher.branches[0] != "innoagent-issue-7" {
		t.Fatalf("unexpected branches: %v", pusher.branches)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(poster.posted))
	}
	if len(pusher.reviewers) != 1 || pusher.reviewers[0] != "alice" {
		t.Fatalf("expected alice as reviewer, got %v", pusher.reviewers)
	}
}

// PR creation failing permanently must not go quiet: the branch was already
// pushed with working code, and the user needs the comment to know they must
// open the PR themselves.
func TestProcess_PRCreationPermanentError_DoneWithFailureComment(t *testing.T) {
	data := makeEnvelope("assigned", "del-pr-fail")
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "a.go", Content: "x"}},
	}}
	pusher := &fakePusher{prErr: fmt.Errorf("500: %w", domain.ErrPermanent)}
	poster := &fakePoster{}
	p := newProc(gen, pusher, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Done {
		t.Fatalf("expected Done, got %v", result)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(poster.posted))
	}
	if !strings.Contains(poster.posted[0], "Pull request creation failed") {
		t.Fatalf("comment does not mention the PR failure:\n%s", poster.posted[0])
	}
}

func TestProcess_Assigned_ReviewerIsIssueCreatorNotSender(t *testing.T) {
	issue := event.IssueEvent{Action: "assigned", Number: 8}
	issue.Issue.Number = 8
	issue.Issue.Title = "Task"
	issue.Issue.Body = json.RawMessage(`"body"`)
	issue.Issue.User.Login = "creator"
	issue.Issue.Assignee.Login = testBotUsername
	issue.Repository.Owner.Login = "myorg"
	issue.Repository.Name = "myrepo"
	issue.Repository.DefaultBranch = "main"
	issue.Sender.Login = "someone-else"

	payload, _ := json.Marshal(issue)
	env := event.Envelope{DeliveryID: "del-creator", EventType: "issues", Payload: payload}
	data, _ := json.Marshal(env)

	pusher := &fakePusher{}
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "a.go", Content: "x"}},
	}}
	p := newProc(gen, pusher, &fakePoster{})

	if result := p.Process(context.Background(), data); result != processor.Done {
		t.Fatalf("expected Done, got %v", result)
	}
	if len(pusher.reviewers) != 1 || pusher.reviewers[0] != "creator" {
		t.Fatalf("expected creator as reviewer, got %v", pusher.reviewers)
	}
}

func TestProcess_Dedup_ByDeliveryID(t *testing.T) {
	data := makeEnvelope("assigned", "del-2")
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "a.go", Content: "x"}},
	}}
	p := newProc(gen, &fakePusher{}, &fakePoster{})

	r1 := p.Process(context.Background(), data)
	r2 := p.Process(context.Background(), data)

	if r1 != processor.Done || r2 != processor.Skip {
		t.Fatalf("expected Done then Skip, got %v and %v", r1, r2)
	}
	if gen.calls != 1 {
		t.Fatalf("generator should be called once, got %d", gen.calls)
	}
}

func TestProcess_NotOnboarded_PostsCommentAndSkips(t *testing.T) {
	data := makeEnvelope("assigned", "del-3")
	gen := &fakeGenerator{err: domain.ErrNotOnboarded}
	poster := &fakePoster{}
	p := newProc(gen, &fakePusher{}, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(poster.posted))
	}
}

func TestProcess_GenerationTransient_Retries(t *testing.T) {
	data := makeEnvelope("assigned", "del-4")
	gen := &fakeGenerator{err: fmt.Errorf("llm down")}
	p := newProc(gen, &fakePusher{}, &fakePoster{})

	result := p.Process(context.Background(), data)
	if result != processor.Transient {
		t.Fatalf("expected Transient, got %v", result)
	}
}

func TestProcess_PushPermanentError_Skip(t *testing.T) {
	data := makeEnvelope("assigned", "del-5")
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "a.go", Content: "x"}},
	}}
	pusher := &fakePusher{err: fmt.Errorf("403: %w", domain.ErrPermanent)}
	poster := &fakePoster{}
	p := newProc(gen, pusher, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}
	// A permanent failure that isn't going to be retried must still tell the
	// user — silently skipping leaves them with no idea anything happened.
	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 failure comment posted, got %d", len(poster.posted))
	}
}

func TestProcess_OpenedWithAssignee_Done(t *testing.T) {
	data := makeEnvelope("opened", "del-6")
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "main.go", Content: "package main"}},
	}}
	p := newProc(gen, &fakePusher{}, &fakePoster{})

	result := p.Process(context.Background(), data)
	if result != processor.Done {
		t.Fatalf("expected Done, got %v", result)
	}
}

func TestProcess_GitFlameUIPayload_Done(t *testing.T) {
	issue := event.IssueEvent{
		Action: "assigned",
		Number: 12,
	}
	issue.Issue.Index = 12
	issue.Issue.Title = "Add endpoint"
	issue.Issue.Body = json.RawMessage(`[{"type":"paragraph","content":"Implement GET /health"}]`)
	issue.Assignee.Login = testBotUsername
	issue.Repository.FullName = "myorg/myrepo"
	issue.Repository.DefaultBranch = "main"
	issue.Sender.Login = "alice"

	payload, _ := json.Marshal(issue)
	env := event.Envelope{
		DeliveryID: "del-ui",
		EventType:  "issue_assign",
		Payload:    payload,
	}
	data, _ := json.Marshal(env)

	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "health.go", Content: "package main"}},
	}}
	p := newProc(gen, &fakePusher{}, &fakePoster{})

	result := p.Process(context.Background(), data)
	if result != processor.Done {
		t.Fatalf("expected Done, got %v", result)
	}
}

func TestProcess_NotOnboarded_PostCommentTransientFail_Transient(t *testing.T) {
	data := makeEnvelope("assigned", "del-7")
	gen := &fakeGenerator{err: domain.ErrNotOnboarded}
	poster := &fakePoster{err: fmt.Errorf("network: %w", domain.ErrTransient)}
	p := newProc(gen, &fakePusher{}, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Transient {
		t.Fatalf("expected Transient, got %v", result)
	}
}

func TestProcess_UndecodablePayload_Skip(t *testing.T) {
	p := newProc(&fakeGenerator{}, &fakePusher{}, &fakePoster{})
	result := p.Process(context.Background(), []byte("not json"))
	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}
}

func TestProcess_GenerationPermanentError_Skip(t *testing.T) {
	data := makeEnvelope("assigned", "del-8")
	gen := &fakeGenerator{err: fmt.Errorf("401: %w", domain.ErrPermanent)}
	poster := &fakePoster{}
	p := newProc(gen, &fakePusher{}, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 failure comment posted, got %d", len(poster.posted))
	}
}

func TestProcess_PostCommentError_Transient(t *testing.T) {
	data := makeEnvelope("assigned", "del-9")
	gen := &fakeGenerator{result: &domain.GenerationResult{
		Files: []domain.GeneratedFile{{Path: "a.go", Content: "x"}},
	}}
	poster := &fakePoster{err: errors.New("network error")}
	p := newProc(gen, &fakePusher{}, poster)

	result := p.Process(context.Background(), data)
	if result != processor.Transient {
		t.Fatalf("expected Transient, got %v", result)
	}
}
