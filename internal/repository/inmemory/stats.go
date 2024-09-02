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
	URL        *url.URL
	Origin     *url.URL
}

type ExpiredURLStatsRepository struct {
	db map[string]eventStat
	mu sync.RWMutex
}

// NewExpiredURLStatsRepository creates a new ExpiredURLStatsRepository
func NewExpiredURLStatsRepository() *ExpiredURLStatsRepository {
	return &ExpiredURLStatsRepository{
		db: make(map[string]eventStat),
	}
}

// PushStats pushes data with statistics into collection
func (r *ExpiredURLStatsRepository) PushStats(ctx context.Context, event domain.URLExpired) error {
	const fn = "in-memory::PushStats"
	zap.S().Infow("repo",
		zap.String("name", "ExpiredURLStatsRepository"),
		zap.String("fn", fn),
		zap.String("event", event.String()),
	)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.db[event.URL.String()] = eventStat{
		OccurredAt: event.OccurredAt,
		URL:        event.URL,
		Origin:     event.Origin,
	}
	return nil
}
