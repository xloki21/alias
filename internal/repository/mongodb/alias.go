package mongodb

import (
	"context"
	"errors"
	"fmt"
	"github.com/xloki21/alias/internal/domain"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"net/url"
)

const (
	AliasCollectionName = "aliases"
	StatsCollectionName = "stats"
)

// AliasDTO is DTO for AliasCollectionName collection
type AliasDTO struct {
	ID          string   `bson:"_id"`
	URL         *url.URL `bson:"url"`
	Key         string   `bson:"key"`
	IsActive    bool     `bson:"is_active"`
	IsPermanent bool     `bson:"is_permanent"`
	TriesLeft   int      `bson:"tries_left,omitempty"`
}

type AliasRepository struct {
	collection *mongo.Collection
}

// NewAliasRepository creates a new AliasRepository
func NewAliasRepository(collection *mongo.Collection) *AliasRepository {
	return &AliasRepository{
		collection: collection,
	}
}

func (r *AliasRepository) Name() string {
	return "mongodb::AliasRepository"
}

// Save saves aliases in storage
func (a *AliasRepository) Save(ctx context.Context, aliases []domain.Alias) error {
	const fn = "Save"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.Int("aliases count", len(aliases)))

	documents := make([]interface{}, len(aliases))
	for index, alias := range aliases {
		documents[index] = bson.D{
			{"key", alias.Key},
			{"url", alias.URL},
			{"is_active", alias.IsActive},
			{"is_permanent", alias.Params.IsPermanent},
			{"tries_left", alias.Params.TriesLeft},
		}
	}
	opStatus, err := a.collection.InsertMany(ctx, documents)
	if err != nil {
		return err
	}
	for index, insertedID := range opStatus.InsertedIDs {
		aliases[index].ID = insertedID.(primitive.ObjectID).Hex()
	}
	return nil
}

// Find gets the alias by key
func (a *AliasRepository) Find(ctx context.Context, key string) (*domain.Alias, error) {
	const fn = "Find"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.String("key", key))

	filter := bson.M{
		"$and": []bson.M{
			{"is_active": true},
			{"key": key},
		},
	}

	result := a.collection.FindOne(ctx, filter)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return nil, domain.ErrAliasNotFound
		}
		return nil, domain.ErrAliasSearchEngineFailure
	}
	doc := new(AliasDTO)
	if err := result.Decode(doc); err != nil {
		return nil, domain.ErrAliasDecodeFailed
	}

	alias := &domain.Alias{
		ID:       doc.ID,
		Key:      doc.Key,
		URL:      doc.URL,
		IsActive: doc.IsActive,
		Params:   domain.TTLParams{TriesLeft: uint64(doc.TriesLeft), IsPermanent: doc.IsPermanent},
	}
	return alias, nil
}

// DecreaseTTLCounter decreases the alias redirect counter
func (a *AliasRepository) DecreaseTTLCounter(ctx context.Context, key string) error {
	const fn = "DecreaseTTLCounter"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.String("key", key))

	filter := bson.M{"key": key, "is_active": true}

	pipeline := bson.A{
		bson.M{
			"$set": bson.M{"tries_left": bson.M{"$add": bson.A{"$tries_left", -1}}},
		},
		bson.M{
			"$set": bson.M{"tries_left": bson.M{"$cond": bson.A{bson.M{"$lt": bson.A{"$tries_left", 0}}, 0, "$tries_left"}}},
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.Before)

	result := a.collection.FindOneAndUpdate(ctx, filter, pipeline, opts)

	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return domain.ErrAliasNotFound
		}
		return fmt.Errorf("%s: %w", fn, result.Err())
	}

	doc := new(AliasDTO)

	if err := result.Decode(doc); err != nil {
		return fmt.Errorf("%s: %w", fn, err)
	}

	if doc.TriesLeft == 0 {
		return domain.ErrAliasExpired
	}

	return nil
}

// Remove deletes a shortened link
func (a *AliasRepository) Remove(ctx context.Context, key string) error {
	const fn = "Remove"
	zap.S().Infow("repo",
		zap.String("name", a.Name()),
		zap.String("fn", fn),
		zap.String("key", key))

	filter := bson.M{"key": key, "is_active": true}
	update := bson.M{"$set": bson.M{"is_active": false}}

	result := a.collection.FindOneAndUpdate(ctx, filter, update)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return domain.ErrAliasNotFound
		}
		return result.Err()
	}
	return nil
}
