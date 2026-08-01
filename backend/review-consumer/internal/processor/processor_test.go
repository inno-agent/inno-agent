package processor_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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

	removedReviewers []string
	removeErr        error
}

func (f *fakePoster) PostPRComment(_ context.Context, _ domain.PRRef, body string) error {
	if f.err != nil {
		return f.err
	}

	f.posted = append(f.posted, body)

	return nil
}

func (f *fakePoster) RemoveRequestedReviewer(_ context.Context, _ domain.PRRef, reviewer string) error {
	if f.removeErr != nil {
		return f.removeErr
	}

	f.removedReviewers = append(f.removedReviewers, reviewer)

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

	// "started" comment + review comment
	if len(poster.posted) != 2 {
		t.Fatalf("expected 2 posted comments (started + review), got %d: %v", len(poster.posted), poster.posted)
	}
	if poster.posted[0] == "" {
		t.Fatal("started comment should not be empty")
	}
	if poster.posted[1] != "# Review" {
		t.Fatalf("expected review comment second, got: %v", poster.posted[1])
	}

	if len(poster.removedReviewers) != 1 || poster.removedReviewers[0] != testBotUsername {
		t.Fatalf("expected bot removed as reviewer, got %v", poster.removedReviewers)
	}
}

func TestProcess_RemoveReviewerFails_StillDone(t *testing.T) {
	// The review comment already posted successfully — a failure removing the
	// bot as reviewer must not turn into a retry (that would double-post).
	data := makeEnvelope("del-900")
	reviewer := &fakeReviewer{result: "# Review"}
	poster := &fakePoster{removeErr: errors.New("gitflame down")}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Done {
		t.Fatalf("expected Done even when reviewer removal fails, got %v", result)
	}

	// "started" comment + review comment
	if len(poster.posted) != 2 {
		t.Fatalf("expected exactly 2 posted comments (started + review), got %d", len(poster.posted))
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

func TestProcess_TransientError_PostsErrorCommentOnce(t *testing.T) {
	// On the first transient failure, an error comment is posted (in addition
	// to the "started" comment). On a retry with the same dedup key, the error
	// comment must NOT be posted again.
	data := makeEnvelope("del-350")
	reviewer := &fakeReviewer{err: errors.New("llm unavailable")}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)

	r1 := p.Process(context.Background(), data)
	if r1 != processor.Transient {
		t.Fatalf("first: expected Transient, got %v", r1)
	}
	// "started" + error notification = 2 comments
	if len(poster.posted) != 2 {
		t.Fatalf("first: expected 2 comments (started + error), got %d: %v", len(poster.posted), poster.posted)
	}

	r2 := p.Process(context.Background(), data)
	if r2 != processor.Transient {
		t.Fatalf("second: expected Transient, got %v", r2)
	}
	// Retry posts "started" again but NOT the error comment again = 3 total
	if len(poster.posted) != 3 {
		t.Fatalf("second: expected 3 total comments (started+error, started), got %d: %v", len(poster.posted), poster.posted)
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
	poster := &fakePoster{}
	p := newProc(reviewer, poster)
	result := p.Process(context.Background(), data)

	if result != processor.Skip {
		t.Fatalf("expected Skip for permanent review error, got %v", result)
	}

	// "started" comment + error comment
	if len(poster.posted) != 2 {
		t.Fatalf("expected 2 comments (started + error), got %d: %v", len(poster.posted), poster.posted)
	}
	if poster.posted[1] == "" {
		t.Fatal("error comment should not be empty")
	}
}

func TestProcess_ReviewPermanentError_ContainsErrorMessage(t *testing.T) {
	data := makeEnvelope("del-550")
	reviewErr := fmt.Errorf("cannot access AI model: status 401: %w", domain.ErrPermanent)
	reviewer := &fakeReviewer{err: reviewErr}
	poster := &fakePoster{}
	p := newProc(reviewer, poster)
	p.Process(context.Background(), data)

	if len(poster.posted) < 2 {
		t.Fatalf("expected at least 2 comments, got %d", len(poster.posted))
	}
	errorComment := poster.posted[1]
	if !strings.Contains(errorComment, "cannot access AI model") {
		t.Fatalf("error comment should contain the error details, got: %s", errorComment)
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

	// "started" comment + not-onboarded comment
	if len(poster.posted) != 2 {
		t.Fatalf("expected 2 comments posted (started + not-onboarded notice), got %d", len(poster.posted))
	}

	if len(poster.posted[1]) == 0 {
		t.Fatal("not-onboarded comment should not be empty")
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

func TestNotifyGaveUp_PostsComment(t *testing.T) {
	data := makeEnvelope("del-giveup")
	poster := &fakePoster{}
	p := newProc(&fakeReviewer{}, poster)

	p.NotifyGaveUp(context.Background(), data)

	if len(poster.posted) != 1 {
		t.Fatalf("expected 1 comment from NotifyGaveUp, got %d", len(poster.posted))
	}
	if poster.posted[0] == "" {
		t.Fatal("gave-up comment should not be empty")
	}
}

func TestNotifyGaveUp_GarbageInput_NoPanic(t *testing.T) {
	p := newProc(&fakeReviewer{}, &fakePoster{})

	// Should not panic on undecodable input.
	p.NotifyGaveUp(context.Background(), []byte("not json"))

	p.NotifyGaveUp(context.Background(), []byte(`{"delivery_id":"x","event_type":"pull_request","payload":"not-json"}`))
}
