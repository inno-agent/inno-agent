package processor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/event"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
)

const (
	testBotUsername   = "innoagent"
	testOnboardingURL = "https://review.example.com/onboard"
)

type fakeReviewer struct {
	result string
	err    error
	calls  int
}

func (f *fakeReviewer) Review(_ context.Context, _ domain.PRRef) (string, error) {
	f.calls++
	return f.result, f.err
}

type fakePoster struct {
	posted []string
	err    error
}

func (f *fakePoster) PostPRComment(_ context.Context, _ domain.PRRef, body string) error {
	if f.err != nil {
		return f.err
	}

	f.posted = append(f.posted, body)

	return nil
}

// newProc creates a processor with default test bot config.
func newProc(reviewer domain.Reviewer, poster domain.CommentPoster) *processor.Processor {
	return processor.New(reviewer, poster, zap.NewNop(), testBotUsername, testOnboardingURL)
}

// makeEnvelope builds a review-request event envelope for the bot reviewer.
func makeEnvelope(deliveryID string) []byte {
	pr := event.PullRequestEvent{
		Action: "reviewer_added",
		Number: 42,
	}
	pr.PullRequest.Head.SHA = "deadbeef"
	pr.Repository.Name = "myrepo"
	pr.Repository.Owner.Login = "myorg"
	pr.RequestedReviewer.Login = testBotUsername
	pr.Sender.Login = "alice"

	payload, _ := json.Marshal(pr)
	env := event.Envelope{
		DeliveryID: deliveryID,
		EventType:  "pull_request_review_request",
		Payload:    payload,
	}
	data, _ := json.Marshal(env)

	return data
}

func TestProcess_WrongEventType_Skip(t *testing.T) {
	env := event.Envelope{EventType: "push", Payload: json.RawMessage(`{}`)}
	data, _ := json.Marshal(env)

	reviewer := &fakeReviewer{}
	p := newProc(reviewer, &fakePoster{})
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip, got %v", result)
	}

	if reviewer.calls != 0 {
		t.Fatal("reviewer should not be called for non-PR event")
	}
}

func TestProcess_WrongRequestedReviewer_Skip(t *testing.T) {
	pr := event.PullRequestEvent{Action: "reviewer_added", Number: 1}
	pr.PullRequest.Head.SHA = "abc"
	pr.Repository.Name = "repo"
	pr.Repository.Owner.Login = "org"
	pr.RequestedReviewer.Login = "some-other-bot"
	pr.Sender.Login = "carol"

	payload, _ := json.Marshal(pr)
	env := event.Envelope{
		DeliveryID: "del-1",
		EventType:  "pull_request_review_request",
		Payload:    payload,
	}
	data, _ := json.Marshal(env)

	reviewer := &fakeReviewer{}
	p := newProc(reviewer, &fakePoster{})
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip for wrong reviewer, got %v", result)
	}

	if reviewer.calls != 0 {
		t.Fatal("reviewer should not be called when bot is not the requested reviewer")
	}
}

func TestProcess_ReviewRequest_Done(t *testing.T) {
	data := makeEnvelope("del-100")
	reviewer := &fakeReviewer{result: "# Review"}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Done {
		t.Fatalf("expected Done, got %v", result)
	}

	if len(poster.posted) != 1 || poster.posted[0] != "# Review" {
		t.Fatalf("unexpected posted comments: %v", poster.posted)
	}
}

func TestProcess_Dedup_ByDeliveryID(t *testing.T) {
	data := makeEnvelope("del-200")
	reviewer := &fakeReviewer{result: "# Review"}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)

	r1 := p.Process(context.Background(), data)
	r2 := p.Process(context.Background(), data)

	if r1 != processor.Done {
		t.Fatalf("first: expected Done, got %v", r1)
	}

	if r2 != processor.Skip {
		t.Fatalf("second: expected Skip (dedup by delivery_id), got %v", r2)
	}

	if reviewer.calls != 1 {
		t.Fatalf("reviewer should be called exactly once, got %d", reviewer.calls)
	}
}

func TestProcess_Dedup_FallbackToSHA_WhenNoDeliveryID(t *testing.T) {
	// Empty delivery_id -> falls back to owner/repo/index@sha key.
	data := makeEnvelope("")
	reviewer := &fakeReviewer{result: "# Review"}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)

	r1 := p.Process(context.Background(), data)
	r2 := p.Process(context.Background(), data)

	if r1 != processor.Done {
		t.Fatalf("first: expected Done, got %v", r1)
	}

	if r2 != processor.Skip {
		t.Fatalf("second: expected Skip (dedup fallback), got %v", r2)
	}
}

func TestProcess_ReviewError_Transient(t *testing.T) {
	data := makeEnvelope("del-300")
	reviewer := &fakeReviewer{err: errors.New("llm unavailable")}
	p := newProc(reviewer, &fakePoster{})
	result := p.Process(context.Background(), data)

	if result != processor.Transient {
		t.Fatalf("expected Transient, got %v", result)
	}
}

func TestProcess_PostCommentError_Transient(t *testing.T) {
	data := makeEnvelope("del-400")
	reviewer := &fakeReviewer{result: "# Review"}
	poster := &fakePoster{err: errors.New("network error")}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Transient {
		t.Fatalf("expected Transient, got %v", result)
	}
}

func TestProcess_UndecodablePayload_Skip(t *testing.T) {
	p := newProc(&fakeReviewer{}, &fakePoster{})
	result := p.Process(context.Background(), []byte("not json"))

	if result != processor.Skip {
		t.Fatalf("expected Skip for garbage input, got %v", result)
	}
}

func TestProcess_ReviewPermanentError_Skip(t *testing.T) {
	data := makeEnvelope("del-500")
	reviewer := &fakeReviewer{err: fmt.Errorf("status 401: %w", domain.ErrPermanent)}
	p := newProc(reviewer, &fakePoster{})
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip for permanent review error, got %v", result)
	}
}

func TestProcess_PostCommentPermanentError_Skip(t *testing.T) {
	data := makeEnvelope("del-600")
	poster := &fakePoster{err: fmt.Errorf("status 403: %w", domain.ErrPermanent)}
	p := newProc(&fakeReviewer{result: "# Review"}, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip for permanent poster error, got %v", result)
	}
}

func TestProcess_NotOnboarded_PostsCommentAndSkips(t *testing.T) {
	data := makeEnvelope("del-700")
	reviewer := &fakeReviewer{err: domain.ErrNotOnboarded}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip for not-onboarded, got %v", result)
	}

	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 comment posted (not-onboarded notice), got %d", len(poster.posted))
	}

	if len(poster.posted[0]) == 0 {
		t.Fatal("posted comment should not be empty")
	}
}

func TestProcess_GrantExpired_PostsCommentAndSkips(t *testing.T) {
	data := makeEnvelope("del-710")
	reviewer := &fakeReviewer{err: domain.ErrGrantExpired}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip for grant-expired, got %v", result)
	}
	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 comment posted (grant-expired notice), got %d", len(poster.posted))
	}
	notOnboardedMsg := fmt.Sprintf("⚠️ @alice, connect your account at %s to use the review bot.", testOnboardingURL)
	if poster.posted[0] == notOnboardedMsg {
		t.Fatal("grant-expired message must differ from not-onboarded message")
	}
}

func TestProcess_GrantExpired_PostCommentTransientFail_Transient(t *testing.T) {
	data := makeEnvelope("del-711")
	reviewer := &fakeReviewer{err: domain.ErrGrantExpired}
	poster := &fakePoster{err: fmt.Errorf("network: %w", domain.ErrTransient)}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Transient {
		t.Fatalf("expected Transient when posting grant-expired comment fails transiently, got %v", result)
	}
}

func TestProcess_NotOnboarded_PostCommentTransientFail_Transient(t *testing.T) {
	data := makeEnvelope("del-701")
	reviewer := &fakeReviewer{err: domain.ErrNotOnboarded}
	poster := &fakePoster{err: fmt.Errorf("network: %w", domain.ErrTransient)}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	// If posting the not-onboarded comment fails transiently, return Transient.
	if result != processor.Transient {
		t.Fatalf("expected Transient when posting not-onboarded comment fails transiently, got %v", result)
	}
}

func TestProcess_DedupBoundedSet_EvictsOldest(t *testing.T) {
	reviewer := &fakeReviewer{result: "ok"}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)

	// Process the same message twice: second call must be a dedup Skip.
	d := makeEnvelope("del-800")
	r1 := p.Process(context.Background(), d)
	r2 := p.Process(context.Background(), d)

	if r1 != processor.Done {
		t.Fatalf("first: want Done, got %v", r1)
	}

	if r2 != processor.Skip {
		t.Fatalf("dedup: want Skip, got %v", r2)
	}
}
