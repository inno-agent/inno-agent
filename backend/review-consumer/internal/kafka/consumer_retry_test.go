package kafka

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
)

// shrinkBackoff makes retries instant for tests and restores the originals.
func shrinkBackoff(t *testing.T) {
	t.Helper()
	oi, oc := retryInitial, retryCap
	retryInitial, retryCap = time.Microsecond, time.Microsecond
	t.Cleanup(func() { retryInitial, retryCap = oi, oc })
}

func newTestConsumer() *Consumer {
	return &Consumer{logger: zap.NewNop()}
}

func TestProcessWithRetry_CommitsImmediatelyOnDone(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits := 0, 0
	c.processWithRetry(context.Background(), 1, 0,
		func() processor.Result { processCalls++; return processor.Done },
		func() bool { commits++; return false },
	)
	if processCalls != 1 || commits != 1 {
		t.Fatalf("Done: processCalls=%d commits=%d, want 1/1", processCalls, commits)
	}
}

func TestProcessWithRetry_GivesUpAndCommitsAfterMaxRetries(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits := 0, 0
	cancelled := c.processWithRetry(context.Background(), 7, 3,
		func() processor.Result { processCalls++; return processor.Transient }, // always transient
		func() bool { commits++; return false },
	)
	if cancelled {
		t.Fatal("should not report cancelled on poison give-up")
	}
	// Loop calls process each iteration; gives up when attempts == max.
	if processCalls != maxTransientRetries {
		t.Fatalf("processCalls=%d, want %d", processCalls, maxTransientRetries)
	}
	if commits != 1 {
		t.Fatalf("poison message must be committed exactly once to unblock partition, got %d", commits)
	}
}

func TestProcessWithRetry_RecoversBeforeCap(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits := 0, 0
	c.processWithRetry(context.Background(), 2, 0,
		func() processor.Result {
			processCalls++
			if processCalls < 3 {
				return processor.Transient
			}
			return processor.Done // recovers on 3rd attempt
		},
		func() bool { commits++; return false },
	)
	if processCalls != 3 || commits != 1 {
		t.Fatalf("recover: processCalls=%d commits=%d, want 3/1", processCalls, commits)
	}
}

func TestProcessWithRetry_StopsOnContextCancel(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	commits := 0
	cancelled := c.processWithRetry(ctx, 1, 0,
		func() processor.Result { return processor.Transient },
		func() bool { commits++; return false },
	)
	if !cancelled {
		t.Fatal("expected cancelled=true when context is done")
	}
	if commits != 0 {
		t.Fatalf("should not commit on cancel, got %d", commits)
	}
}
