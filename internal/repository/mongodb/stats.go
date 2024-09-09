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
	URL        *url.URL  `bson:"url"`
	Origin     *url.URL  `bson:"origin"`
}

type AliasStatsRepository struct {
	collection *mongo.Collection
}

// NewAliasStatsRepository creates a new AliasStatsRepository
func NewAliasStatsRepository(collection *mongo.Collection) *AliasStatsRepository {
	return &AliasStatsRepository{
		collection: collection,
	}
}

// PushStats pushes data with statistics into collection
func (r *AliasStatsRepository) PushStats(ctx context.Context, event domain.AliasExpired) error {
	const fn = "mongodb::PushEvent"
	zap.S().Infow("repo",
		zap.String("name", "NewAliasStatsRepository"),
		zap.String("fn", fn),
		zap.String("event", event.String()),
	)
	newEventDoc := eventDocument{
		OccurredAt: event.OccurredAt,
		URL:        event.URL,
		Origin:     event.Origin,
	}

	if _, err := r.collection.InsertOne(ctx, newEventDoc); err != nil {
		return domain.ErrStatsCollectingFailed
	}
	return nil
}
