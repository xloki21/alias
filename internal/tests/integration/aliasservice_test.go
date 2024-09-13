package integration

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tc "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/squeue"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/service/alias"
	"github.com/xloki21/alias/internal/service/manager"
	"github.com/xloki21/alias/internal/service/stats"
	"github.com/xloki21/alias/pkg/keygen"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"testing"
)

const (
	aliasAppDatabase = "appdb"
	tag              = "7.0.6"
)

func setupMongoDBContainer(t *testing.T, testData []domain.Alias) (testcontainers.Container, *mongo.Database) {
	ctx := context.Background()

	mongodbContainer, err := tc.Run(ctx, fmt.Sprintf("mongo:%s", tag))
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

func NewAliasTestService(ctx context.Context, db *mongo.Database) *alias.AliasService {
	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	aliasRepoMongoDB := mongodb.NewMongoDBAliasRepository(db.Collection(mongodb.AliasCollectionName))
	statsRepoMongoDB := mongodb.NewAliasStatsRepository(db.Collection(mongodb.StatsCollectionName))
	aliasManagerSvc := manager.NewAliasManagerService(aliasRepoMongoDB, aliasUsedQ)
	aliasManagerSvc.Process(ctx)

	aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoMongoDB, aliasExpiredQ)
	aliasStatsSvc.Process(ctx)

	keyGen := keygen.NewURLSafeRandomStringGenerator()

	aliasService := alias.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoMongoDB, keyGen)
	return aliasService

}

func TestAliasServiceCreateManyMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	container, db := setupMongoDBContainer(t, nil)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := NewAliasTestService(ctx, db)

	type args struct {
		ctx      context.Context
		requests []domain.AliasCreationRequest
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr error
	}{
		{
			name:        "create multiple aliases with success",
			args:        args{ctx: context.Background(), requests: alias.TestSetAliasCreationRequests(2000)},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := aliasService.CreateMany(testCase.args.ctx, testCase.args.requests)
			assert.NoError(t, err)
		})
	}
}

func TestAliasServiceFindOneMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		alias.TestAlias(t, false),
		alias.TestAlias(t, true),
	}

	container, db := setupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := NewAliasTestService(ctx, db)

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
			name: "alias is not found",
			args: args{
				ctx: context.Background(),
				key: "non-existent-key",
			},
			wants:       nil,
			expectedErr: domain.ErrAliasNotFound,
		},
		{
			name: "alias is found",
			args: args{
				ctx: context.Background(),
				key: testData[0].Key,
			},
			wants:       &testData[0],
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := aliasService.FindOne(ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
			if testCase.wants != nil {
				assert.Equal(t, testCase.wants.URL, got.URL)
			}
		})
	}
}

func TestAliasServiceRemoveOneMongoDB(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	testData := []domain.Alias{
		alias.TestAlias(t, false),
		alias.TestAlias(t, true),
	}

	container, db := setupMongoDBContainer(t, testData)
	defer func(container testcontainers.Container, ctx context.Context) {
		err := container.Terminate(ctx)
		require.NoError(t, err)
	}(container, ctx)

	aliasService := NewAliasTestService(ctx, db)

	type args struct {
		ctx context.Context
		key string
	}

	testCases := []struct {
		name        string
		args        args
		expectedErr error
	}{
		{
			name:        "remove non-existent aliases",
			args:        args{ctx: context.Background(), key: "non-existent-key"},
			expectedErr: domain.ErrAliasNotFound,
		},
		{
			name:        "remove alias successfully",
			args:        args{ctx: context.Background(), key: testData[0].Key},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := aliasService.RemoveOne(testCase.args.ctx, testCase.args.key)
			assert.ErrorIs(t, err, testCase.expectedErr)
		})
	}
}
