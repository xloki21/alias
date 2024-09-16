package managersvc

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
)

type aliasRepository interface {
	DecreaseTTLCounter(ctx context.Context, key string) error
}

type eventConsumer interface {
	Consume() chan any
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
	go func() {
		for event := range m.consumer.Consume() {
			m.processEvent(ctx, event)
		}
	}()
}

func (m *Manager) processEvent(ctx context.Context, msg any) {
	const fn = "processEvent"
	event := msg.(domain.AliasUsed)
	zap.S().Infow("service",
		zap.String("name", m.Name()),
		zap.String("fn", fn),
		zap.String("received", event.String()),
		zap.String("alias key", event.Key),
	)

	if !event.Params.IsPermanent {
		if err := m.aliasRepo.DecreaseTTLCounter(ctx, event.Alias.Key); err != nil {
			zap.S().Errorw("service",
				zap.String("name", m.Name()),
				zap.String("fn", fn),
				zap.String("error", err.Error()))
		}
	}

}
