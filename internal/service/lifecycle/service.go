package lifecycle

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
)

const maxQueueSize = 1024

type aliasRepository interface {
	DecreaseTTLCounter(ctx context.Context, alias domain.Alias) error
}

type statsRepository interface {
	PushStats(ctx context.Context, event domain.URLExpired) error
}

type AliasLifeCycleService struct {
	eventQueue chan interface{}
	aliasRepo  aliasRepository
	statsRepo  statsRepository
}

func (s *AliasLifeCycleService) Name() string {
	return "AliasLifeCycleService"
}

func (s *AliasLifeCycleService) Publish(ctx context.Context, event interface{}) error {
	s.eventQueue <- event
	return nil
}

func (s *AliasLifeCycleService) ProcessEvents(ctx context.Context) error {
	const fn = "AliasLifeCycleService::ProcessEvents"

	for {
		select {
		case message := <-s.eventQueue:

			switch message.(type) {

			case domain.URLExpired:
				event := message.(domain.URLExpired)
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

			case domain.AliasLinkRedirected:
				event := message.(domain.AliasLinkRedirected)

				zap.S().Infow("service",
					zap.String("name", s.Name()),
					zap.String("fn", fn),
					zap.String("received", event.String()),
				)

				if event.IsPermanent {
					break
				} else {
					if err := s.aliasRepo.DecreaseTTLCounter(ctx, event.Alias); err != nil {
						zap.S().Errorw("service",
							zap.String("name", s.Name()),
							zap.String("fn", fn),
							zap.String("error", err.Error()))
					}
				}
			default:
			}

		case <-ctx.Done():
			zap.S().Infow("service",
				zap.String("name", s.Name()),
				zap.String("fn", fn),
				zap.String("exiting reason", ctx.Err().Error()))
			return nil
		default:
		}
	}
}

func NewAliasLifeCycleService(aliasRepo aliasRepository, statsRepo statsRepository) *AliasLifeCycleService {
	return &AliasLifeCycleService{
		eventQueue: make(chan interface{}, maxQueueSize),
		aliasRepo:  aliasRepo,
		statsRepo:  statsRepo,
	}
}
