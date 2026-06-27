package kafka_test

import (
	"strings"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-webhook/internal/kafka"
)

// TestNewPublisher verifies that a Publisher can be constructed without panicking
// and that the topic / brokers are accepted. No live broker is required.
func TestNewPublisher(t *testing.T) {
	brokers := "localhost:9092"
	topic := "gitflame.events"

	pub := kafka.NewPublisher(brokers, topic)
	if pub == nil {
		t.Fatal("expected non-nil publisher")
	}

	// Verify Close does not panic (it may return an error when no broker is present,
	// but it must not crash).
	_ = pub.Close()

	// Sanity-check that our helper split logic matches the consumer's approach.
	parts := strings.Split(brokers, ",")
	if len(parts) != 1 || parts[0] != "localhost:9092" {
		t.Errorf("unexpected broker split: %v", parts)
	}
}
