package statssvc

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
)

type statsRepository interface {
	PushStats(ctx context.Context, event domain.AliasExpired) error
}

type eventConsumer interface {
	Consume() chan any
}

type Statistics struct {
	consumer  eventConsumer
	statsRepo statsRepository
}

func (s *Statistics) Name() string {
	return "Statistics"
}

func (s *Statistics) Process(ctx context.Context) {
	go func() {
		for event := range s.consumer.Consume() {
			s.processEvent(ctx, event)
		}
	}()
}

func (s *Statistics) processEvent(ctx context.Context, msg any) {
	const fn = "processEvent"
	event := msg.(domain.AliasExpired)
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("received", event.String()),
		zap.String("alias key", event.Key),
	)

	err := s.statsRepo.PushStats(ctx, event)
	if err != nil {
		zap.S().Errorw("service",
			zap.String("name", s.Name()),
			zap.String("fn", fn),
			zap.String("error", err.Error()))
	}
}

func NewStatistics(statsRepo statsRepository, consumer eventConsumer) *Statistics {
	return &Statistics{
		consumer:  consumer,
		statsRepo: statsRepo,
	}
}
