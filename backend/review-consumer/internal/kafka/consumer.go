package kafka

import (
	"context"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	kafka "github.com/segmentio/kafka-go"

	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
)

// Tunables (vars, not consts, so tests can shrink the backoffs).
var (
	retryInitial = time.Second
	retryCap     = 30 * time.Second
	fetchErrWait = time.Second
	// maxTransientRetries caps in-place retries of one message. Beyond it the
	// message is treated as poison: logged loudly and skipped (committed) so it
	// can't wedge the partition forever. Trades at-least-once for liveness.
	maxTransientRetries = 10
)

type Consumer struct {
	reader    *kafka.Reader
	processor *processor.Processor
	logger    *zap.Logger
}

func NewConsumer(brokers, topic, group string, proc *processor.Processor, logger *zap.Logger) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        strings.Split(brokers, ","),
		Topic:          topic,
		GroupID:        group,
		MinBytes:       1,
		MaxBytes:       10 * 1024 * 1024,
		MaxWait:        500 * time.Millisecond,
		CommitInterval: 0, // manual commit
	})

	return &Consumer{
		reader:    reader,
		processor: proc,
		logger:    logger.With(zap.String("layer", "kafka")),
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	c.logger.Info("consumer started")
	defer func() {
		_ = c.reader.Close()
		c.logger.Info("consumer stopped")
	}()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("fetch message failed", zap.Error(err))
			// Small cancellable backoff to avoid busy-spinning on a broken reader.
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(fetchErrWait):
			}
			continue
		}

		// Retry the SAME message in place on Transient results (capped backoff,
		// capped attempts) so the failed offset is never leapfrogged — until it
		// looks like poison, at which point we skip to keep the partition live.
		if c.processWithRetry(ctx, msg.Offset, msg.Partition,
			func() processor.Result {
				msgCtx := tracing.ContextFromKafkaHeaders(ctx, kafkaHeaders(msg.Headers))
				msgCtx, span := tracing.StartSpan(msgCtx, "review-consumer", "kafka.process")
				defer span.End()
				span.SetAttributes(
					attribute.Int64("kafka.offset", msg.Offset),
					attribute.Int("kafka.partition", msg.Partition),
				)

				result := c.processor.Process(msgCtx, msg.Value)
				if result == processor.Transient {
					span.SetStatus(codes.Error, "transient")
				}
				return result
			},
			func() bool { return c.commit(ctx, msg) },
		) {
			return nil
		}
	}
}

// commit commits the message's offset. Returns true if the context was
// cancelled (caller should stop).
func (c *Consumer) commit(ctx context.Context, msg kafka.Message) (cancelled bool) {
	if err := c.reader.CommitMessages(ctx, msg); err != nil {
		if ctx.Err() != nil {
			return true
		}
		c.logger.Error("commit failed", zap.Error(err))
	}
	return false
}

// processWithRetry runs process(), retrying Transient results with capped
// exponential backoff. On a non-transient result, or after maxTransientRetries
// (poison message), it commits via commit(). Returns true if the context was
// cancelled. process/commit are injected so this is unit-testable without Kafka.
func (c *Consumer) processWithRetry(
	ctx context.Context,
	offset int64,
	partition int,
	process func() processor.Result,
	commit func() bool,
) (cancelled bool) {
	backoff := retryInitial
	attempts := 0
	for {
		if process() != processor.Transient {
			return commit()
		}

		attempts++
		if attempts >= maxTransientRetries {
			c.logger.Error(
				"poison message: giving up after max transient retries; skipping to unblock partition",
				zap.Int("attempts", attempts),
				zap.Int("max", maxTransientRetries),
				zap.Int64("offset", offset),
				zap.Int("partition", partition),
			)
			return commit()
		}

		c.logger.Info(
			"transient result; retrying message",
			zap.Int("attempt", attempts),
			zap.Int("max", maxTransientRetries),
			zap.Duration("backoff", backoff),
			zap.Int64("offset", offset),
		)
		select {
		case <-ctx.Done():
			return true
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > retryCap {
			backoff = retryCap
		}
	}
}

func kafkaHeaders(headers []kafka.Header) []tracing.KafkaHeader {
	out := make([]tracing.KafkaHeader, len(headers))
	for i, h := range headers {
		out[i] = tracing.KafkaHeader{Key: h.Key, Value: string(h.Value)}
	}
	return out
}
