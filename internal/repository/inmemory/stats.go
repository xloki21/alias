package inmemory

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"go.uber.org/zap"
	"net/url"
	"sync"
	"time"
)

type eventStat struct {
	OccurredAt time.Time
	Key        string
	Origin     *url.URL
}

type AliasStatsRepository struct {
	db map[string]eventStat
	mu sync.RWMutex
}

// NewAliasStatsRepository creates a new AliasStatsRepository
func NewAliasStatsRepository() *AliasStatsRepository {
	return &AliasStatsRepository{
		db: make(map[string]eventStat),
	}
}

// PushStats pushes data with statistics into collection
func (r *AliasStatsRepository) PushStats(ctx context.Context, event domain.AliasExpired) error {
	const fn = "in-memory::PushStats"
	zap.S().Infow("repo",
		zap.String("name", "ExpiredURLStatsRepository"),
		zap.String("fn", fn),
		zap.String("event", event.String()),
	)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.db[event.Key] = eventStat{
		OccurredAt: event.OccurredAt,
		Key:        event.Key,
		Origin:     event.Origin,
	}
	return nil
}
