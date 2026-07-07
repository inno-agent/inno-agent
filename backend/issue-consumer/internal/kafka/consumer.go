package kafka

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"

	kafka "github.com/segmentio/kafka-go"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/processor"
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
		CommitInterval: 0,
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
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(fetchErrWait):
			}
			continue
		}

		backoff := retryInitial
		for {
			result := c.processor.Process(ctx, msg.Value)
			if result != processor.Transient {
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					c.logger.Error("commit failed", zap.Error(err))
				}
				break
			}

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
