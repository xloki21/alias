package manager

import (
	"context"
	"github.com/xloki21/alias/internal/config"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/repository"
	"github.com/xloki21/alias/internal/repository/inmemory"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/service/manager"
	"github.com/xloki21/alias/pkg/kafker"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	mongoDBServerSelectionTimeout = 5 * time.Second
)

type Application struct {
	service *manager.Manager
}

func New(cfg config.AppConfig) (*Application, error) {
	ctx := context.Background()
	consumerCfg := cfg.GetConsumerConfig("Alias Used Event Consumer")
	consumer, err := kafker.NewConsumer(consumerCfg.GroupID, consumerCfg.Topic, consumerCfg.GetBrokersURI(), nil)
	if err != nil {
		zap.S().Fatalf("cannot create consumer: topic=%s", consumerCfg.Topic)
		return nil, err
	}

	var managerService *manager.Manager

	switch cfg.Storage.Type {
	case repository.MongoDB:

		clientOptions := options.Client().
			ApplyURI(cfg.Storage.MongoDB.URI).
			SetServerSelectionTimeout(mongoDBServerSelectionTimeout)

		if cfg.Storage.MongoDB.Credentials.AuthSource != "" {
			credential := options.Credential{
				AuthSource: cfg.Storage.MongoDB.Credentials.AuthSource,
				Username:   cfg.Storage.MongoDB.Credentials.User,
				Password:   cfg.Storage.MongoDB.Credentials.Password,
			}
			clientOptions.SetAuth(credential)
		}

		client, err := mongo.Connect(ctx, clientOptions)

		if err != nil {
			zap.S().Fatalf("cannot connect to mongodb: %s", cfg.Storage.MongoDB.URI)
			return nil, err
		}

		if err := client.Ping(ctx, nil); err != nil {
			zap.S().Fatalf("mongodb: ping failed: %s", err.Error())
			return nil, err
		}

		db := client.Database(cfg.Storage.MongoDB.Database)

		if db == nil {
			zap.S().Fatalf("cannot connect to mongodb: %s", cfg.Storage.MongoDB.URI)
			return nil, err
		}

		repo := mongodb.NewAliasRepository(db.Collection(mongodb.AliasCollectionName))
		managerService = manager.NewManager(repo, consumer)

	case repository.InMemory:
		statsRepo := inmemory.NewAliasRepository()
		managerService = manager.NewManager(statsRepo, consumer)

	default:
		zap.S().Fatalf("unknown storage type: %s", cfg.Storage.Type)
		return nil, domain.ErrUnknownStorageType
	}
	zap.S().Infow("core", zap.String("state", "selected storage type"), zap.String("type", string(cfg.Storage.Type)))

	app := &Application{
		service: managerService,
	}
	return app, nil
}

func (a *Application) Run(ctx context.Context) error {
	ctx, cancelFn := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	a.service.Process(ctx)

	select {
	case <-ctx.Done():
		zap.S().Infow("core", zap.String("state", "application graceful shutdown begins"))
		return nil
	}
}
