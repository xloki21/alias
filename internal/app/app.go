package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/xloki21/alias/internal/app/config"
	grpc2 "github.com/xloki21/alias/internal/controller/grpc"
	"github.com/xloki21/alias/internal/controller/grpc/interceptors"
	"github.com/xloki21/alias/internal/controller/rest"
	"github.com/xloki21/alias/internal/controller/rest/mw"
	"github.com/xloki21/alias/internal/domain"
	aliasapi "github.com/xloki21/alias/internal/gen/go/pbuf/alias"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	httpServerReadTimeout  = 10 * time.Second
	httpServerWriteTimeout = 10 * time.Second
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
	HTTPServer        *http.Server
	GRPCGatewayServer *http.Server
	GRPCServer        *grpc.Server
	grpcListener      net.Listener
}

func New(cfg config.AppConfig) (*Application, error) {
	ctx := context.Background()
	baseURLPrefix := fmt.Sprintf("http://%s%s", cfg.Service.HTTP, endpointRedirect)

	var aliasService *aliassvc.Alias

	aliasUsedQ := squeue.New()
	aliasExpiredQ := squeue.New()

	keyGen := keygen.NewURLSafeRandomStringGenerator()

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

		//clientOptions := options.Client().
		//	ApplyURI(cfg.Storage.MongoDB.URI).
		//	SetServerSelectionTimeout(mongoDBServerSelectionTimeout).SetAuth(credential)

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

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptors.LoggingInterceptor))
	reflection.Register(grpcServer)
	aliasapi.RegisterAliasAPIServer(grpcServer, grpc2.NewController(aliasService, baseURLPrefix))

	listener, err := net.Listen("tcp", cfg.Service.GRPC)
	if err != nil {
		zap.S().Fatalw("core", zap.String("application error", err.Error()))
		return nil, err
	}

	gwmux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := aliasapi.RegisterAliasAPIHandlerFromEndpoint(ctx, gwmux, cfg.Service.GRPCGateway, opts); err != nil {
		zap.S().Fatalw("core", zap.String("application error", err.Error()))
		return nil, err
	}

	app := &Application{
		HTTPServer: &http.Server{
			Addr:         cfg.Service.HTTP,
			ReadTimeout:  httpServerReadTimeout,
			WriteTimeout: httpServerWriteTimeout,
			Handler:      http.DefaultServeMux,
		},
		GRPCServer: grpcServer,
		GRPCGatewayServer: &http.Server{
			Addr:    cfg.Service.GRPCGateway,
			Handler: gwmux,
		},
		grpcListener: listener,
	}
	ctrlHTTP := rest.NewController(aliasService, baseURLPrefix)
	app.initializeRoutes(ctrlHTTP)

	return app, nil
}

func (a *Application) Run(ctx context.Context) error {
	ctx, cancelFn := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	errChan := make(chan error)
	go func() {
		zap.S().Infow("HTTP", zap.String("stage", fmt.Sprintf("start server, listening on %s", a.HTTPServer.Addr)))
		if err := a.HTTPServer.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}
	}()

	go func() {
		zap.S().Infow("gRPC-gw", zap.String("stage", fmt.Sprintf("start server, listening on %s", a.GRPCGatewayServer.Addr)))
		if err := a.GRPCGatewayServer.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}
	}()

	go func() {
		zap.S().Infow("gRPC", zap.String("stage", fmt.Sprintf("start server, listening on %s", a.grpcListener.Addr())))
		if err := a.GRPCServer.Serve(a.grpcListener); err != nil {
			zap.S().Fatalw("core", zap.String("application error", err.Error()))
		}
	}()

	select {
	case <-ctx.Done():
		zap.S().Infow("core", zap.String("stage", "application graceful shutdown begins"))
		zap.S().Infow("core", zap.String("stage", "shutting down http server"))
		err := a.HTTPServer.Shutdown(ctx)
		if err != nil {
			return err
		}
		zap.S().Infow("core", zap.String("stage", "HTTP server stopped"))
		zap.S().Infow("core", zap.String("stage", "shutting down gRPC-gateway server"))

		err = a.GRPCGatewayServer.Shutdown(ctx)
		if err != nil {
			return err
		}
		zap.S().Infow("core", zap.String("stage", "gRPC-gateway server stopped"))

		zap.S().Infow("core", zap.String("stage", "shutting down gRPC-server"))
		a.GRPCServer.GracefulStop()
		zap.S().Infow("core", zap.String("stage", "gRPC-server stopped"))
		return nil
	case err := <-errChan:
		zap.S().Fatalw("core", zap.String("application error", err.Error()))
		return err
	}
}

func (a *Application) initializeRoutes(ctrl *rest.Controller) {
	zap.S().Infow("HTTP", zap.String("stage", "initialize routes"))
	mux := http.NewServeMux()
	mux.HandleFunc(endpointAlias, mw.Use(ctrl.CreateAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(endpointHealthcheck, mw.Use(ctrl.Healthcheck, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(endpointAlias+"/{key}", mw.Use(ctrl.RemoveAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(endpointRedirect+"/{key}", mw.Use(ctrl.Redirect, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.HTTPServer.Handler = mux
}
