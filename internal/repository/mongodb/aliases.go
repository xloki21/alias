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

// aliasDocument is DTO for alias collection
type aliasDocument struct {
	ID          string   `bson:"_id"`
	Origin      *url.URL `bson:"origin"`
	Alias       *url.URL `bson:"alias"`
	IsActive    bool     `bson:"is_active"`
	IsPermanent bool     `bson:"is_permanent"`
	TTL         int      `bson:"ttl"`
}

type AliasRepository struct {
	collection *mongo.Collection
}

// NewMongoDBAliasRepository creates a new AliasRepository
func NewMongoDBAliasRepository(collection *mongo.Collection) *AliasRepository {
	return &AliasRepository{
		collection: collection,
	}
}

// SaveOne saves a alias link
func (a *AliasRepository) SaveOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "mongodb::SaveOne"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()),
		zap.String("origin", alias.Origin.String()))

	document := bson.D{
		{"alias", alias.URL},
		{"origin", alias.Origin},
		{"is_active", alias.IsActive},
		{"is_permanent", alias.IsPermanent},
		{"TTL", alias.TTL},
	}
	opStatus, err := a.collection.InsertOne(ctx, document)
	if err != nil {
		return err
	}
	alias.ID = opStatus.InsertedID.(primitive.ObjectID).Hex()
	return nil
}

// SaveMany saves many aliases in bulk
func (a *AliasRepository) SaveMany(ctx context.Context, aliases []*domain.Alias) error {
	const fn = "mongodb::SaveMany"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.Int("aliases count", len(aliases)))

	documents := make([]interface{}, len(aliases))
	for index := range aliases {
		documents[index] = bson.D{
			{"alias", aliases[index].URL},
			{"origin", aliases[index].Origin},
			{"is_active", aliases[index].IsActive},
			{"is_permanent", aliases[index].IsPermanent},
			{"TTL", aliases[index].TTL},
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

// FindOne gets the target link from the shortened one
func (a *AliasRepository) FindOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "mongodb::FindOne"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))

	filter := bson.M{
		"$and": []bson.M{
			{"is_active": true},
			{"alias": alias.URL},
		},
	}

	result := a.collection.FindOne(ctx, filter)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return domain.ErrAliasNotFound
		}
		return result.Err()
	}
	document := aliasDocument{}
	if err := result.Decode(&document); err != nil {
		return err
	}

	alias.ID = document.ID
	alias.Origin = document.Origin
	alias.TTL = document.TTL
	alias.IsActive = document.IsActive
	alias.IsPermanent = document.IsPermanent

	return nil
}

// DecreaseTTLCounter decreases the alias redirect counter
func (a *AliasRepository) DecreaseTTLCounter(ctx context.Context, alias domain.Alias) error {
	const fn = "mongodb::DecreaseTTLCounter"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))

	if alias.TTL == 0 {
		return domain.ErrAliasExpired
	}

	filter := bson.M{"alias": alias.URL, "is_active": true}

	pipeline := bson.A{
		bson.M{
			"$set": bson.M{"TTL": bson.M{"$add": bson.A{"$TTL", -1}}},
		},
		bson.M{
			"$set": bson.M{"TTL": bson.M{"$cond": bson.A{bson.M{"$lt": bson.A{"$TTL", 0}}, 0, "$TTL"}}},
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
	return nil
}

// RemoveOne deletes a shortened link
func (a *AliasRepository) RemoveOne(ctx context.Context, alias *domain.Alias) error {
	const fn = "mongodb::RemoveOne"
	zap.S().Infow("repo",
		zap.String("name", "AliasRepository"),
		zap.String("fn", fn),
		zap.String("alias", alias.URL.String()))

	filter := bson.M{"alias": alias.URL, "is_active": true}
	update := bson.M{"$set": bson.M{"is_active": false}}

	result := a.collection.FindOneAndUpdate(ctx, filter, update)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return domain.ErrAliasNotFound
		}
		return result.Err()
	}

	alias.IsActive = false
	return nil
}
