package kafka

import (
	"context"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"log"
	"time"
)

type Consumer interface {
	Consume(ctx context.Context, fn func(context.Context, any) error) error
	Close()
}

// NewConsumer создает простой Kafka-консьюмер
func NewConsumer(groupID, topic string, brokers []string, startOffset *int64) Consumer {
	if startOffset == nil {
		startOffset = new(int64)
		*startOffset = kafkago.FirstOffset
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     brokers,
		GroupID:     groupID,
		Topic:       topic,
		MinBytes:    10e3, // 10KB
		MaxBytes:    10e6, // 10MB
		StartOffset: *startOffset,
		MaxWait:     10 * time.Millisecond,
	})

	zap.S().Infow("core",
		zap.String("state", "created Kafka consumer"),
		zap.String("topic", topic))

	return &consumer{
		Reader: reader,
		topic:  topic,
	}
}

type consumer struct {
	*kafkago.Reader
	topic string
}

func (c *consumer) Consume(ctx context.Context, fn func(context.Context, any) error) error {
	msg, err := c.Reader.ReadMessage(ctx)
	if err != nil {
		return err
	}

	if err := fn(ctx, msg); err != nil {
		return err
	}
	// Коммит сообщения после успешной обработки
	if err := c.Reader.CommitMessages(ctx, msg); err != nil {
		return err
	}

	return nil
}

func (c *consumer) TopicName() string {
	return c.topic
}

func (c *consumer) Close() {
	if err := c.Reader.Close(); err != nil {
		log.Printf("error closing consumer: %v", err)
	}
}
