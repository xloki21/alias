package stats

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/gen/go/pbuf/aliasapi"
	"github.com/xloki21/alias/pkg/kafker"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewStatistics(statsRepo statsRepository, consumer eventConsumer) *Statistics {
	return &Statistics{
		consumer:  consumer,
		statsRepo: statsRepo,
	}
}

type statsRepository interface {
	PushStats(ctx context.Context, event domain.Event) error
}

type eventConsumer interface {
	Consume(ctx context.Context, fn func(context.Context, any) error) error
	Close()
}

type Statistics struct {
	consumer  eventConsumer
	statsRepo statsRepository
}

func (s *Statistics) Name() string {
	return "Statistics"
}

func (s *Statistics) Process(ctx context.Context) {
	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				s.consumer.Close()
				return ctx.Err()
			default:
				err := s.consumer.Consume(ctx, s.consumerFn)
				if err != nil {
					return err
				}
			}
		}
	})
}

func (s *Statistics) consumerFn(ctx context.Context, received any) error {
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
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("received", event.String()),
		zap.String("alias key", event.Key),
	)

	if err := s.statsRepo.PushStats(ctx, *event); err != nil {
		zap.S().Errorw("service",
			zap.String("name", s.Name()),
			zap.String("fn", fn),
			zap.String("error", err.Error()))
		return errors.New("failed to push stat info")
	}
	return nil
}
