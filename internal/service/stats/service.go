package stats

import (
	"context"
	"errors"
	"github.com/segmentio/kafka-go"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/gen/go/pbuf/aliasapi"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
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

func (s *Statistics) consumerFn(ctx context.Context, msg any) error {
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
