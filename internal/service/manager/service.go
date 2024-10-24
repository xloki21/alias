package manager

import (
	"context"
	"github.com/segmentio/kafka-go"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/gen/go/pbuf/aliasapi"
	"github.com/xloki21/alias/pkg/kafker"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type aliasRepository interface {
	DecreaseTTLCounter(ctx context.Context, key string) error
}

type eventConsumer interface {
	Consume(ctx context.Context, fn func(context.Context, any) error) error
	Close()
}

type Manager struct {
	consumer  eventConsumer
	aliasRepo aliasRepository
}

func NewManager(aliasRepo aliasRepository, consumer eventConsumer) *Manager {
	return &Manager{
		consumer:  consumer,
		aliasRepo: aliasRepo,
	}
}

func (m *Manager) Name() string {
	return "Manager"
}

func (m *Manager) Process(ctx context.Context) {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				m.consumer.Close()
				return ctx.Err()
			default:
				err := m.consumer.Consume(ctx, m.consumerFn)
				if err != nil {
					return err
				}
			}
		}
	})
}

func (m *Manager) consumerFn(ctx context.Context, received any) error {
	const fn = "consumerFn"

	event := new(domain.Event)

	switch received.(type) {
	case kafka.Message:
		raw := received.(kafka.Message)
		message := kafker.NewFromRaw(raw)
		protoEvent := new(aliasapi.Event)

		err := message.AsProto(protoEvent)
		if err != nil {
			return domain.ErrInternal
		}

		event, err = domain.NewEventFromProto(protoEvent)
		if err != nil {
			return domain.ErrInternal
		}
	default:
		return domain.ErrUnknownBrokerMessageType
	}

	zap.S().Infow("service",
		zap.String("name", m.Name()),
		zap.String("fn", fn),
		zap.String("received", event.String()),
		zap.String("alias key", event.Key),
	)

	if !event.Params.IsPermanent {
		if err := m.aliasRepo.DecreaseTTLCounter(ctx, event.Key); err != nil {
			zap.S().Errorw("service",
				zap.String("name", m.Name()),
				zap.String("fn", fn),
				zap.String("error", err.Error()))
			return err
		}
	}
	return nil

}
