package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/xloki21/alias/internal/app/config"
	"github.com/xloki21/alias/internal/controller"
	"github.com/xloki21/alias/internal/controller/mw"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/squeue"
	"github.com/xloki21/alias/internal/repository"
	"github.com/xloki21/alias/internal/repository/inmemory"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/service/alias"
	"github.com/xloki21/alias/internal/service/manager"
	"github.com/xloki21/alias/internal/service/stats"
	"github.com/xloki21/alias/pkg/keygen"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	httpServerReadTimeout   = 10 * time.Second
	httpServerWriteTimeout  = 10 * time.Second
	gracefulShutdownTimeout = 5 * time.Second
)

const (
	mongoDBServerSelectionTimeout = 5 * time.Second
)

const (
	apiV1               = "/api/v1"
	endpointCreateAlias = apiV1 + "/alias"
	endpointHealthcheck = apiV1 + "/healthcheck"
	endpointRedirect    = ""
	endpointRemoveLink  = apiV1 + "/remove"
)

type Application struct {
	address    string
	router     *http.ServeMux
	controller *controller.AliasController
}

func New(cfg config.AppConfig) (*Application, error) {
	ctx := context.Background()
	baseURLPrefix := fmt.Sprintf("http://%s%s", cfg.Server.Address, endpointRedirect)

	var aliasService *alias.AliasService

	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	keyGen := keygen.NewURLSafeRandomStringGenerator()

	switch cfg.Storage.Type {
	case repository.MongoDB:

		credential := options.Credential{
			AuthSource: cfg.Storage.MongoDB.Credentials.AuthSource,
			Username:   cfg.Storage.MongoDB.Credentials.User,
			Password:   cfg.Storage.MongoDB.Credentials.Password,
		}
		clientOptions := options.Client().
			ApplyURI(cfg.Storage.MongoDB.URI).
			SetAuth(credential).
			SetServerSelectionTimeout(mongoDBServerSelectionTimeout)

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

		aliasRepoMongoDB := mongodb.NewMongoDBAliasRepository(db.Collection(mongodb.AliasCollectionName))
		statsRepoMongoDB := mongodb.NewAliasStatsRepository(db.Collection(mongodb.StatsCollectionName))
		aliasManagerSvc := manager.NewAliasManagerService(aliasRepoMongoDB, aliasUsedQ)
		aliasManagerSvc.Process(ctx)

		aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoMongoDB, aliasExpiredQ)
		aliasStatsSvc.Process(ctx)

		aliasService = alias.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoMongoDB, keyGen)

	case repository.InMemory:
		zap.S().Info("using in-memory storage type")
		aliasRepoInMemory := inmemory.NewAliasRepository()
		statsRepoInMemory := inmemory.NewAliasStatsRepository()

		aliasManagerSvc := manager.NewAliasManagerService(aliasRepoInMemory, aliasUsedQ)
		aliasManagerSvc.Process(ctx)

		aliasStatsSvc := stats.NewAliasStatisticsService(statsRepoInMemory, aliasExpiredQ)
		aliasStatsSvc.Process(ctx)

		aliasService = alias.NewAliasService(aliasExpiredQ, aliasUsedQ, aliasRepoInMemory, keyGen)

	default:
		zap.S().Fatalf("unknown storage type: %s", cfg.Storage.Type)
		return nil, domain.ErrUnknownStorageType
	}

	ctrl := controller.NewAliasController(aliasService, baseURLPrefix)

	app := &Application{
		address:    cfg.Server.Address,
		router:     http.NewServeMux(),
		controller: ctrl,
	}

	app.initializeRoutes()

	return app, nil
}

func (a *Application) Run(ctx context.Context) error {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	server := &http.Server{
		Addr:         a.address,
		ReadTimeout:  httpServerReadTimeout,
		WriteTimeout: httpServerWriteTimeout,
		Handler:      a.router,
	}

	errChan := make(chan error)
	go func() {
		zap.S().Infof("listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	select {
	case <-quit:
		zap.S().Info("gracefully shutting down server")
		return server.Shutdown(ctx)
	case <-ctx.Done():
		zap.S().Warn("Context was cancelled, shutting down server")
		ctxTimeout, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()
		return server.Shutdown(ctxTimeout)

	case err := <-errChan:
		zap.S().Errorf("server main loop error: %s", err.Error())
		return err
	}
}

func (a *Application) initializeRoutes() {
	a.router.HandleFunc(endpointCreateAlias, mw.Use(a.controller.CreateAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.router.HandleFunc(endpointHealthcheck, mw.Use(a.controller.Healthcheck, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.router.HandleFunc(endpointRemoveLink, mw.Use(a.controller.RemoveAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.router.HandleFunc(endpointRedirect+"/{key}", mw.Use(a.controller.Redirect, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
}
