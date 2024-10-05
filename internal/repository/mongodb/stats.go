package mongodb

import (
	"context"
	"github.com/xloki21/alias/internal/domain"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"net/url"
	"time"
)

type eventDocument struct {
	OccurredAt time.Time `bson:"occurred_at"` // time when event occurred
	Key        string    `bson:"key"`
	URL        *url.URL  `bson:"url"`
}

type StatisticsRepository struct {
	collection *mongo.Collection
}

// NewStatisticsRepository creates a new StatisticsRepository
func NewStatisticsRepository(collection *mongo.Collection) *StatisticsRepository {
	return &StatisticsRepository{
		collection: collection,
	}
}

func (r *StatisticsRepository) Name() string {
	return "mongodb::StatisticsRepository"
}

// PushStats pushes data with statistics into collection
func (r *StatisticsRepository) PushStats(ctx context.Context, event domain.Event) error {
	const fn = "PushEvent"
	zap.S().Infow("repo",
		zap.String("name", r.Name()),
		zap.String("fn", fn),
		zap.String("event", event.String()),
		zap.String("alias key", event.Key))
	newEventDoc := eventDocument{
		OccurredAt: event.OccurredAt,
		Key:        event.Key,
		URL:        event.URL,
	}

	if _, err := r.collection.InsertOne(ctx, newEventDoc); err != nil {
		return domain.ErrStatsCollectingFailed
	}
	return nil
}
