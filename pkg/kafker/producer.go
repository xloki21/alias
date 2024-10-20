package kafker

import (
	"context"
	"crypto/tls"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"log"
)

type Producer interface {
	WriteMessage(ctx context.Context, message Message) error
	Close()
}

type producer struct {
	writer *kafkago.Writer
}

func NewProducer(brokers []string, topic string, tlsCfg *tls.Config) Producer {
	// Конфигурация writer
	writer := &kafkago.Writer{
		Addr:  kafkago.TCP(brokers...),
		Topic: topic,
		Balancer: &kafkago.RoundRobin{
			ChunkSize: 1,
		},
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
		writer: writer,
	}
}

// WriteMessage отправляет сообщение в Kafka
func (p *producer) WriteMessage(ctx context.Context, message Message) error {

	if err := p.writer.WriteMessages(ctx, message.Message); err != nil {
		log.Printf("Failed to write message: %v", err)
		return err
	}

	log.Printf("Message sent with key %s", string(message.Key))
	return nil
}

func (p *producer) Close() {
	if err := p.writer.Close(); err != nil {
		log.Printf("Failed to close producer: %v", err)
		return
	}
}
