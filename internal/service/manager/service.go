package manager

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/msgbroker"
	"go.uber.org/zap"
)

type aliasRepository interface {
	DecreaseTTLCounter(ctx context.Context, alias domain.Alias) error
}

type eventConsumer interface {
	Consume() chan any
}

type AliasManagerService struct {
	consumer  eventConsumer
	aliasRepo aliasRepository
}

func (s *AliasManagerService) Name() string {
	return "AliasManagerService"
}

func (s *AliasManagerService) Process(ctx context.Context) {
	go func() {
		for event := range s.consumer.Consume() {
			s.processEvent(ctx, event)
		}
	}()
}

func (s *AliasManagerService) processEvent(ctx context.Context, msg any) {
	const fn = "AliasManagerService::processEvent"
	event := msg.(domain.AliasUsed)
	zap.S().Infow("service",
		zap.String("name", s.Name()),
		zap.String("fn", fn),
		zap.String("received", event.String()),
	)

	if !event.IsPermanent {
		if err := s.aliasRepo.DecreaseTTLCounter(ctx, event.Alias); err != nil {
			zap.S().Errorw("service",
				zap.String("name", s.Name()),
				zap.String("fn", fn),
				zap.String("error", err.Error()))
		}
	}

}

func NewAliasManagerService(aliasRepo aliasRepository, eventQueue msgbroker.Queue) *AliasManagerService {
	return &AliasManagerService{
		consumer:  eventQueue,
		aliasRepo: aliasRepo,
	}
}
