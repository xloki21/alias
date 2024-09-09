package stats

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/msgbroker"
	"go.uber.org/zap"
)

type statsRepository interface {
	PushStats(ctx context.Context, event domain.AliasExpired) error
}

type eventConsumer interface {
	Consume() chan any
}

type AliasStatisticsService struct {
	consumer  eventConsumer
	statsRepo statsRepository
}

func (s *AliasStatisticsService) Name() string {
	return "AliasStatisticsService"
}

func (s *AliasStatisticsService) Process(ctx context.Context) {
	go func() {
		for event := range s.consumer.Consume() {
			s.processEvent(ctx, event)
		}
	}()
}

func (s *AliasStatisticsService) processEvent(ctx context.Context, msg any) {
	const fn = "AliasStatisticsService::processEvent"
	event := msg.(domain.AliasExpired)
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("received", event.String()),
	)

	err := s.statsRepo.PushStats(ctx, event)
	if err != nil {
		zap.S().Errorw("service",
			zap.String("name", s.Name()),
			zap.String("fn", fn),
			zap.String("error", err.Error()))
	}
}

func NewAliasStatisticsService(statsRepo statsRepository, eventQueue msgbroker.Queue) *AliasStatisticsService {
	return &AliasStatisticsService{
		consumer:  eventQueue,
		statsRepo: statsRepo,
	}
}
