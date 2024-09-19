package tests

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/squeue"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/services/aliassvc"
	"github.com/xloki21/alias/internal/services/managersvc"
	"github.com/xloki21/alias/internal/services/statssvc"
	"github.com/xloki21/alias/pkg/keygen"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"testing"
)

const (
	aliasAppDatabase = "appdb"
	image            = "mongo:7.0.6"
)

func SetupMongoDBContainer(t *testing.T, testData []domain.Alias) (*tc.MongoDBContainer, *mongo.Database) {
	ctx := context.Background()

	mongodbContainer, err := tc.Run(ctx, image)
	require.NoError(t, err)

	connstr, err := mongodbContainer.ConnectionString(ctx)
	require.NoError(t, err)

	clientOptions := options.Client().
		ApplyURI(connstr)

	client, err := mongo.Connect(ctx, clientOptions)
	require.NoError(t, err)

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	db := client.Database(aliasAppDatabase)
	coll := db.Collection(mongodb.AliasCollectionName)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{"key", 1}},
		Options: options.Index().SetUnique(true),
	}
	name, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	require.NoError(t, err)
	zap.S().Info("index created", zap.String("name", name))

	if testData != nil {
		zap.S().Info("filling test data", zap.String("collection", mongodb.AliasCollectionName))
		docs := make([]interface{}, len(testData))
		for index, alias := range testData {
			docs[index] = bson.D{
				{"key", alias.Key},
				{"url", alias.URL},
				{"is_active", alias.IsActive},
				{"is_permanent", alias.Params.IsPermanent},
				{"tries_left", alias.Params.TriesLeft},
			}
		}
		_, err := coll.InsertMany(ctx, docs, options.InsertMany().SetOrdered(false))
		assert.NoError(t, err)
	}
	return mongodbContainer, db
}

func NewTestAliasService(ctx context.Context, db *mongo.Database) *aliassvc.Alias {
	usedQ := squeue.New()
	expiredQ := squeue.New()

	aliasRepo := mongodb.NewAliasRepository(db.Collection(mongodb.AliasCollectionName))
	statsRepo := mongodb.NewStatisticsRepository(db.Collection(mongodb.StatsCollectionName))
	managerSvc := managersvc.NewManager(aliasRepo, usedQ)
	managerSvc.Process(ctx)

	statsSvc := statssvc.NewStatistics(statsRepo, expiredQ)
	statsSvc.Process(ctx)

	keyGen := keygen.NewURLSafeRandomStringGenerator()

	aliasService := aliassvc.NewAlias(expiredQ, usedQ, aliasRepo, keyGen)
	return aliasService
}
