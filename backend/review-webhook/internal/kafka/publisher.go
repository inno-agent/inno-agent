package kafka

import (
	"context"
	"strings"

	kafka "github.com/segmentio/kafka-go"
)

// Publisher writes messages to a single Kafka topic using synchronous acks.
// A successful Publish means the message is durably enqueued (RequiredAcks = RequireAll).
type Publisher struct {
	writer *kafka.Writer
}

// NewPublisher creates a Publisher that writes to the given topic on the given brokers.
func NewPublisher(brokers, topic string) *Publisher {
	w := &kafka.Writer{
		Addr:         kafka.TCP(strings.Split(brokers, ",")...),
		Topic:        topic,
		RequiredAcks: kafka.RequireAll,
		Balancer:     &kafka.Hash{},
	}
	return &Publisher{writer: w}
}

// Publish writes a single message with the given key and value.
func (p *Publisher) Publish(ctx context.Context, key string, value []byte) error {
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: value,
	})
}

// Close flushes and closes the underlying writer.
func (p *Publisher) Close() error {
	return p.writer.Close()
}
