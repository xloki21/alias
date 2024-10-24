package kafker

import (
	"context"
	"crypto/tls"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde/protobuf"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"log"
)

type Producer interface {
	WriteMessage(ctx context.Context, message Message) error
	WriteMessage2(ctx context.Context, message proto.Message) error
	Close()
}

type producer struct {
	writer     *kafkago.Writer
	serializer serde.Serializer
}

func NewProducer(brokers []string, topic string, tlsCfg *tls.Config) (Producer, error) {
	writer := &kafkago.Writer{
		Addr:  kafkago.TCP(brokers...),
		Topic: topic,
		Balancer: &kafkago.RoundRobin{
			ChunkSize: 1,
		},
	}

	//todo: schema-registry client
	c, err := schemaregistry.NewClient(schemaregistry.NewConfig("http://localhost:8085"))
	if err != nil {
		return nil, err
	}

	s, err := protobuf.NewSerializer(c, serde.ValueSerde, protobuf.NewSerializerConfig())
	if err != nil {
		return nil, err
	}

	if tlsCfg != nil {
		writer.Transport = &kafkago.Transport{
			TLS: tlsCfg,
		}
	}

	zap.S().Infow("core",
		zap.String("state", "created Kafka producer"),
		zap.String("topic", topic))

	return &producer{
		writer:     writer,
		serializer: s,
	}, nil
}

// WriteMessage отправляет сообщение в Kafka
func (p *producer) WriteMessage(ctx context.Context, message Message) error {

	if err := p.writer.WriteMessages(ctx, message.Message); err != nil {
		log.Printf("Failed to write message: %v", err)
		return err
	}

	return nil
}

func (p *producer) WriteMessage2(ctx context.Context, message proto.Message) error {

	payload, err := p.serializer.Serialize(p.writer.Topic, message)
	if err != nil {
		return err
	}

	if err := p.writer.WriteMessages(ctx, kafkago.Message{Value: payload}); err != nil {
		log.Printf("Failed to write message: %v", err)
		return err
	}

	return nil

}

func (p *producer) Close() {
	if err := p.writer.Close(); err != nil {
		log.Printf("Failed to close producer: %v", err)
		return
	}
}
