package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"

	kafka "github.com/segmentio/kafka-go"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/processor"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
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
			telemetry.IncConsumerKafkaFetchError()
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(fetchErrWait):
			}
			continue
		}

		backoff := retryInitial
		for {
			c.logIncomingMessage(msg)

			result := c.processor.Process(ctx, msg.Value)
			if result != processor.Transient {
				if err := c.reader.CommitMessages(ctx, msg); err != nil {
					if ctx.Err() != nil {
						return nil
					}
					c.logger.Error("commit failed", zap.Error(err))
					telemetry.IncConsumerKafkaCommitError()
				}
				break
			}

			telemetry.IncConsumerKafkaRetry()
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

func (c *Consumer) logIncomingMessage(msg kafka.Message) {
	var envelope struct {
		EventType  string `json:"event_type"`
		DeliveryID string `json:"delivery_id"`
	}
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		c.logger.Info(
			"message received",
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
		return
	}
	c.logger.Info(
		"message received",
		zap.String("event_type", envelope.EventType),
		zap.String("delivery_id", envelope.DeliveryID),
		zap.Int64("offset", msg.Offset),
	)
}
