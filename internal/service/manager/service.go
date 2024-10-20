package manager

import (
	"context"
	"github.com/segmentio/kafka-go"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/gen/go/pbuf/aliasapi"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
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

func (m *Manager) consumerFn(ctx context.Context, msg any) error {
	const fn = "consumerFn"

	message := msg.(kafka.Message)
	newEvt := new(aliasapi.Event)
	err := proto.Unmarshal(message.Value, newEvt)
	if err != nil {
		return err // todo: fix
	}

	event, err := domain.NewEventFromProto(newEvt)
	if err != nil {
		return err // todo: fix
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
