//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	tc "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/squeue"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/service/link"
	"github.com/xloki21/alias/internal/service/manager"
	"github.com/xloki21/alias/internal/service/stats"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"testing"
)

func setupMongoDBContainer(t *testing.T) (testcontainers.Container, *mongo.Database) {
	ctx := context.Background()

	mongodbContainer, err := tc.Run(ctx, "mongo:7.0.6")
	if err != nil {
		t.Fatal(err)
	}

	connstr, err := mongodbContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}

	clientOptions := options.Client().
		ApplyURI(connstr)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		t.Fatal(err)
	}

	db := client.Database("appdb")
	coll := db.Collection(mongodb.AliasCollectionName)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{"key", 1}},
		Options: options.Index().SetUnique(true),
	}
	name, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("index created: ", name)

	return mongodbContainer, db
}

func TestAliasServiceCreateManyMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := setupMongoDBContainer(t)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		if err != nil {
			t.Error(err)
		}
	}(container, ctx)

	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	aliasRepoMongoDB := mongodb.NewMongoDBAliasRepository(db.Collection(mongodb.AliasCollectionName))
	statsRepoMongoDB := mongodb.NewAliasStatsRepository(db.Collection(mongodb.StatsCollectionName))
	aliasManagerSvc := manager.NewAliasManagerService(aliasRepoMongoDB, aliasUsedQ)
	aliasManagerSvc.Process(ctx)

	aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoMongoDB, aliasExpiredQ)
	aliasStatsSvc.Process(ctx)

	aliasService := link.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoMongoDB)

	testCases := []struct {
		name        string
		args        []*domain.Alias
		expectedErr error
	}{
		{
			name:        "create multiple aliases with success",
			args:        link.TestURLSet(2000),
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.NoError(t, aliasService.CreateMany(ctx, testCase.args))

			for _, arg := range testCase.args {
				foundOne := &domain.Alias{
					Key:      arg.Key,
					IsActive: true,
				}

				err := aliasRepoMongoDB.FindOne(ctx, foundOne)
				if assert.NoError(t, err) {
					assert.Equal(t, arg, foundOne)
				}
			}
		})
	}
}

func TestAliasServiceFindOneMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := setupMongoDBContainer(t)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		if err != nil {
			t.Error(err)
		}
	}(container, ctx)

	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	aliasRepoMongoDB := mongodb.NewMongoDBAliasRepository(db.Collection(mongodb.AliasCollectionName))
	statsRepoMongoDB := mongodb.NewAliasStatsRepository(db.Collection(mongodb.StatsCollectionName))
	aliasManagerSvc := manager.NewAliasManagerService(aliasRepoMongoDB, aliasUsedQ)
	aliasManagerSvc.Process(ctx)

	aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoMongoDB, aliasExpiredQ)
	aliasStatsSvc.Process(ctx)

	aliasService := link.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoMongoDB)

	testCases := []struct {
		name        string
		prefill     bool
		args        []*domain.Alias
		expectedErr error
	}{
		{
			name:    "alias not found",
			prefill: false,
			args: []*domain.Alias{
				{
					Key:      "non-existent-alias",
					IsActive: true,
				},
				{
					Key:      "non-existent-alias",
					IsActive: false,
				},
			},
			expectedErr: domain.ErrAliasNotFound,
		},
		{
			name:        "alias found",
			prefill:     true,
			args:        link.TestURLSet(2000),
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.prefill {
				assert.NoError(t, aliasService.CreateMany(ctx, testCase.args))
			}

			for _, arg := range testCase.args {
				found, err := aliasService.FindOne(ctx, arg.Key)
				if testCase.expectedErr != nil {
					assert.ErrorIs(t, err, testCase.expectedErr)
					assert.Nil(t, found)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, found)
				}
			}
		})
	}
}

func TestAliasServiceRemoveOneMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := setupMongoDBContainer(t)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		if err != nil {
			t.Error(err)
		}
	}(container, ctx)

	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	aliasRepoMongoDB := mongodb.NewMongoDBAliasRepository(db.Collection(mongodb.AliasCollectionName))
	statsRepoMongoDB := mongodb.NewAliasStatsRepository(db.Collection(mongodb.StatsCollectionName))
	aliasManagerSvc := manager.NewAliasManagerService(aliasRepoMongoDB, aliasUsedQ)
	aliasManagerSvc.Process(ctx)

	aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoMongoDB, aliasExpiredQ)
	aliasStatsSvc.Process(ctx)

	aliasService := link.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoMongoDB)

	testCases := []struct {
		name        string
		prefill     bool
		args        []*domain.Alias
		expectedErr error
	}{
		{
			name:    "remove non-existent aliases",
			prefill: false,
			args: []*domain.Alias{
				{
					Key: "non-existent-alias",
				},
				{
					Key: "other-non-existent-alias",
				},
			},
			expectedErr: domain.ErrAliasNotFound,
		},
		{
			name:        "remove existing aliases",
			prefill:     true,
			args:        link.TestURLSet(2000),
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.prefill {
				assert.NoError(t, aliasService.CreateMany(ctx, testCase.args))
			}

			for _, arg := range testCase.args {
				err := aliasService.RemoveOne(ctx, arg)
				if testCase.expectedErr != nil {
					assert.ErrorIs(t, err, testCase.expectedErr)
				} else {
					assert.NoError(t, err)
				}
			}
		})
	}
}
