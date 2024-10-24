package kafker

import (
	"context"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde/protobuf"
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
func NewConsumer(groupID, topic string, brokers []string, startOffset *int64) (Consumer, error) {
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

	c, err := schemaregistry.NewClient(schemaregistry.NewConfig("http://localhost:8085"))
	if err != nil {
		return nil, err
	}

	d, err := protobuf.NewDeserializer(c, serde.ValueSerde, protobuf.NewDeserializerConfig())
	if err != nil {
		return nil, err
	}

	zap.S().Infow("core",
		zap.String("state", "created Kafka consumer"),
		zap.String("topic", topic))

	return &consumer{
		Reader:       reader,
		topic:        topic,
		deserializer: d,
	}, nil
}

type consumer struct {
	*kafkago.Reader
	topic        string
	deserializer serde.Deserializer
}

func (c *consumer) Consume(ctx context.Context, fn func(context.Context, any) error) error {
	message, err := c.Reader.ReadMessage(ctx)
	if err != nil {
		return err
	}

	// todo: sr-deserialization
	//tmp, err := c.deserializer.Deserialize("expired-value", message.Value)
	//if err != nil {
	//	return err
	//}
	//fmt.Println(tmp)

	if err := fn(ctx, message); err != nil {
		return err
	}
	// Коммит сообщения после успешной обработки
	if err := c.Reader.CommitMessages(ctx, message); err != nil {
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
