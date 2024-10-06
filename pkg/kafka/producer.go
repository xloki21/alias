package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	kafkago "github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"log"
)

type Producer interface {
	WriteMessage(ctx context.Context, msg any) error
	Close()
}

// producer структура для работы с Kafka
type producer struct {
	writer *kafkago.Writer
}

// NewProducer создает новый Kafka продьюсер с конфигурацией
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
func (p *producer) WriteMessage(ctx context.Context, msg any) error {
	content, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	kmsg := kafkago.Message{
		Value: content,
	}
	if err := p.writer.WriteMessages(ctx, kmsg); err != nil {
		log.Printf("Failed to write message: %v", err)
		return err
	}

	log.Printf("Message sent with key %s", string(kmsg.Key))
	return nil
}

func (p *producer) Close() {
	if err := p.writer.Close(); err != nil {
		log.Printf("Failed to close producer: %v", err)
		return
	}
}
