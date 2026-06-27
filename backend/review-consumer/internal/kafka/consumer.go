package kafka

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	kafka "github.com/segmentio/kafka-go"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
)

const (
	retryInitial = time.Second
	retryCap     = 30 * time.Second
	fetchErrWait = time.Second
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

		// Retry the SAME message payload in place on Transient results with
		// capped exponential backoff. This ensures the failed offset is never
		// leapfrogged by a later commit (at-least-once guarantee).
		backoff := retryInitial
		for {
			result := c.processor.Process(ctx, msg.Value)
			if result != processor.Transient {
				// Done or Skip: commit this offset and move on.
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					c.logger.Error("commit failed", zap.Error(err))
				}
				break
			}

			// Transient: sleep then retry the same message.
			c.logger.Info(
				"transient result; retrying message",
				zap.Duration("backoff", backoff),
				zap.Int64("offset", msg.Offset),
			)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > retryCap {
				backoff = retryCap
			}
		}
	}
}
