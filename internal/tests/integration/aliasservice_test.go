package integration

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	"go.uber.org/zap"
	"testing"
)

const mongoDockerImage = "mongo:7.0.6"

type TestHelper struct {
	service *link.AliasService
	repo    *mongodb.AliasRepository
}

func NewTestHelper(ctx context.Context, db *mongo.Database) *TestHelper {
	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	aliasRepoMongoDB := mongodb.NewMongoDBAliasRepository(db.Collection(mongodb.AliasCollectionName))
	statsRepoMongoDB := mongodb.NewAliasStatsRepository(db.Collection(mongodb.StatsCollectionName))
	aliasManagerSvc := manager.NewAliasManagerService(aliasRepoMongoDB, aliasUsedQ)
	aliasManagerSvc.Process(ctx)

	aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoMongoDB, aliasExpiredQ)
	aliasStatsSvc.Process(ctx)

	return &TestHelper{
		service: link.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoMongoDB),
		repo:    aliasRepoMongoDB,
	}
}

func setupMongoDBContainer(t *testing.T) (testcontainers.Container, *mongo.Database) {
	ctx := context.Background()

	mongodbContainer, err := tc.Run(ctx, mongoDockerImage)
	require.NoError(t, err)

	connstr, err := mongodbContainer.ConnectionString(ctx)
	require.NoError(t, err)

	clientOptions := options.Client().
		ApplyURI(connstr)

	client, err := mongo.Connect(ctx, clientOptions)
	require.NoError(t, err)

	err = client.Ping(ctx, nil)
	require.NoError(t, err)

	db := client.Database("appdb")
	coll := db.Collection(mongodb.AliasCollectionName)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{"key", 1}},
		Options: options.Index().SetUnique(true),
	}
	name, err := coll.Indexes().CreateOne(context.TODO(), indexModel)
	require.NoError(t, err)
	zap.S().Info("index created", zap.String("name", name))

	return mongodbContainer, db
}

func TestAliasServiceCreateManyMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := setupMongoDBContainer(t)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	th := NewTestHelper(ctx, db)

	type args struct {
		ctx     context.Context
		aliases []*domain.Alias
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr error
	}{
		{
			name:        "create multiple aliases with success",
			args:        args{ctx: context.Background(), aliases: link.TestURLSet(t, 2000)},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.NoError(t, th.service.CreateMany(testCase.args.ctx, testCase.args.aliases))

			for _, arg := range testCase.args.aliases {
				foundOne := &domain.Alias{
					Key:      arg.Key,
					IsActive: true,
				}

				err := th.repo.FindOne(ctx, foundOne)
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
		require.NoError(t, err)
	}(container, ctx)

	th := NewTestHelper(ctx, db)

	type args struct {
		ctx context.Context
		key string
	}

	testCases := []struct {
		name        string
		args        args
		wants       *domain.Alias
		expectedErr error
	}{
		{
			name: "alias not found",
			args: args{
				ctx: context.Background(),
				key: "non-existent-key",
			},
			wants:       nil,
			expectedErr: domain.ErrAliasNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			aliases := link.TestURLSet(t, 2000)
			assert.NoError(t, th.service.CreateMany(ctx, aliases))

			got, err := th.service.FindOne(ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
			assert.Equal(t, got, testCase.wants)
		})
	}
}

func TestAliasServiceRemoveOneMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := setupMongoDBContainer(t)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	th := NewTestHelper(ctx, db)

	type args struct {
		ctx   context.Context
		alias *domain.Alias
	}

	testCases := []struct {
		name        string
		prefill     bool
		args        args
		expectedErr error
	}{
		{
			name:        "remove non-existent aliases",
			prefill:     false,
			args:        args{ctx: context.Background(), alias: &domain.Alias{Key: "non-existent-key"}},
			expectedErr: domain.ErrAliasNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			err := th.service.RemoveOne(testCase.args.ctx, testCase.args.alias)
			assert.ErrorIs(t, err, testCase.expectedErr)
		})
	}
}
