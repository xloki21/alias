package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/xloki21/alias/internal/app/config"
	"github.com/xloki21/alias/internal/controller/rest"
	"github.com/xloki21/alias/internal/controller/rest/mw"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/infrastructure/squeue"
	"github.com/xloki21/alias/internal/repository"
	"github.com/xloki21/alias/internal/repository/inmemory"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/services/aliassvc"
	"github.com/xloki21/alias/internal/services/managersvc"
	"github.com/xloki21/alias/internal/services/statssvc"
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
	endpointAlias       = apiV1 + "/alias"
	endpointHealthcheck = apiV1 + "/healthcheck"
	endpointRedirect    = ""
)

type Application struct {
	Address    string
	Router     *http.ServeMux
	Controller *rest.Controller
}

func New(cfg config.AppConfig) (*Application, error) {
	ctx := context.Background()
	baseURLPrefix := fmt.Sprintf("http://%s%s", cfg.Server.Address, endpointRedirect)

	var aliasService *aliassvc.Alias

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

		aliasRepo := mongodb.NewAliasRepository(db.Collection(mongodb.AliasCollectionName))
		statsRepo := mongodb.NewStatisticsRepository(db.Collection(mongodb.StatsCollectionName))
		managerSvc := managersvc.NewManager(aliasRepo, aliasUsedQ)
		managerSvc.Process(ctx)

		statsSvc := statssvc.NewStatistics(statsRepo, aliasExpiredQ)
		statsSvc.Process(ctx)

		aliasService = aliassvc.NewAlias(aliasExpiredQ, aliasUsedQ, aliasRepo, keyGen)

	case repository.InMemory:
		zap.S().Info("using in-memory storage type")
		aliasRepo := inmemory.NewAliasRepository()
		statsRepo := inmemory.NewStatisticsRepository()

		managerSvc := managersvc.NewManager(aliasRepo, aliasUsedQ)
		managerSvc.Process(ctx)

		statsSvc := statssvc.NewStatistics(statsRepo, aliasExpiredQ)
		statsSvc.Process(ctx)

		aliasService = aliassvc.NewAlias(aliasExpiredQ, aliasUsedQ, aliasRepo, keyGen)

	default:
		zap.S().Fatalf("unknown storage type: %s", cfg.Storage.Type)
		return nil, domain.ErrUnknownStorageType
	}

	ctrl := rest.NewController(aliasService, baseURLPrefix)

	app := &Application{
		Address:    cfg.Server.Address,
		Router:     http.NewServeMux(),
		Controller: ctrl,
	}

	app.initializeRoutes()

	return app, nil
}

func (a *Application) Run(ctx context.Context) error {
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	server := &http.Server{
		Addr:         a.Address,
		ReadTimeout:  httpServerReadTimeout,
		WriteTimeout: httpServerWriteTimeout,
		Handler:      a.Router,
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
	a.Router.HandleFunc(endpointAlias, mw.Use(a.Controller.CreateAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.Router.HandleFunc(endpointHealthcheck, mw.Use(a.Controller.Healthcheck, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.Router.HandleFunc(endpointAlias+"/{key}", mw.Use(a.Controller.RemoveAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.Router.HandleFunc(endpointRedirect+"/{key}", mw.Use(a.Controller.Redirect, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
}
