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
	URL        *url.URL
}

type StatisticsRepository struct {
	db map[string]eventStat
	mu sync.RWMutex
}

// NewStatisticsRepository creates a new StatisticsRepository
func NewStatisticsRepository() *StatisticsRepository {
	return &StatisticsRepository{
		db: make(map[string]eventStat),
	}
}

func (r *StatisticsRepository) Name() string {
	return "in-memory::StatisticsRepository"
}

// PushStats pushes data with statistics into collection
func (r *StatisticsRepository) PushStats(ctx context.Context, event domain.Event) error {
	const fn = "PushStats"
	zap.S().Infow("repo",
		zap.String("name", r.Name()),
		zap.String("fn", fn),
		zap.String("event", event.String()),
	)
	r.mu.Lock()
	defer r.mu.Unlock()

	r.db[event.Key] = eventStat{
		OccurredAt: event.OccurredAt,
		Key:        event.Key,
	}
	return nil
}
