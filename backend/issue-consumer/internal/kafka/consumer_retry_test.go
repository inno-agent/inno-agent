package kafka

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/segmentio/kafka-go"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/processor"
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

func testMsg() kafka.Message {
	return kafka.Message{Offset: 1, Partition: 0}
}

func TestProcessWithRetry_CommitsImmediatelyOnDone(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits, giveUps := 0, 0, 0
	cancelled := c.processWithRetry(
		context.Background(), testMsg(),
		func() processor.Result { processCalls++; return processor.Done },
		func() bool { commits++; return false },
		func() { giveUps++ },
	)
	if cancelled {
		t.Fatal("should not report cancelled on Done")
	}
	if processCalls != 1 || commits != 1 || giveUps != 0 {
		t.Fatalf("Done: processCalls=%d commits=%d giveUps=%d, want 1/1/0", processCalls, commits, giveUps)
	}
}

func TestProcessWithRetry_GivesUpAndCommitsAfterMaxRetries(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits, giveUps := 0, 0, 0
	cancelled := c.processWithRetry(
		context.Background(), testMsg(),
		func() processor.Result { processCalls++; return processor.Transient }, // always transient
		func() bool { commits++; return false },
		func() { giveUps++ },
	)
	if cancelled {
		t.Fatal("should not report cancelled on poison give-up")
	}
	// Loop calls process each iteration; gives up when attempts == max.
	if processCalls != maxAttempts {
		t.Fatalf("processCalls=%d, want %d", processCalls, maxAttempts)
	}
	if commits != 1 {
		t.Fatalf("poison message must be committed exactly once to unblock partition, got %d", commits)
	}
	if giveUps != 1 {
		t.Fatalf("giveUp must be called exactly once on poison, got %d", giveUps)
	}
}

func TestProcessWithRetry_RecoversBeforeCap(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits, giveUps := 0, 0, 0
	cancelled := c.processWithRetry(
		context.Background(), testMsg(),
		func() processor.Result {
			processCalls++
			if processCalls < 3 {
				return processor.Transient
			}
			return processor.Done // recovers on 3rd attempt
		},
		func() bool { commits++; return false },
		func() { giveUps++ },
	)
	if cancelled {
		t.Fatal("should not report cancelled on recovery")
	}
	if processCalls != 3 || commits != 1 || giveUps != 0 {
		t.Fatalf("recover: processCalls=%d commits=%d giveUps=%d, want 3/1/0", processCalls, commits, giveUps)
	}
}

func TestProcessWithRetry_StopsOnContextCancel(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	commits, giveUps := 0, 0
	cancelled := c.processWithRetry(
		ctx, testMsg(),
		func() processor.Result { return processor.Transient },
		func() bool { commits++; return false },
		func() { giveUps++ },
	)
	if !cancelled {
		t.Fatal("expected cancelled=true when context is done")
	}
	if commits != 0 {
		t.Fatalf("should not commit on cancel, got %d", commits)
	}
	if giveUps != 0 {
		t.Fatalf("should not call giveUp on cancel, got %d", giveUps)
	}
}

func TestProcessWithRetry_SkipCommitsWithoutGiveUp(t *testing.T) {
	shrinkBackoff(t)
	c := newTestConsumer()
	processCalls, commits, giveUps := 0, 0, 0
	cancelled := c.processWithRetry(
		context.Background(), testMsg(),
		func() processor.Result { processCalls++; return processor.Skip },
		func() bool { commits++; return false },
		func() { giveUps++ },
	)
	if cancelled {
		t.Fatal("should not report cancelled on Skip")
	}
	if processCalls != 1 || commits != 1 || giveUps != 0 {
		t.Fatalf("Skip: processCalls=%d commits=%d giveUps=%d, want 1/1/0", processCalls, commits, giveUps)
	}
}
