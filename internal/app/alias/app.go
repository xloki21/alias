package alias

import (
	"context"
	"errors"
	"fmt"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/xloki21/alias/internal/config"
	"github.com/xloki21/alias/internal/controller/grpcc"
	"github.com/xloki21/alias/internal/controller/grpcc/interceptors"
	"github.com/xloki21/alias/internal/controller/httpc"
	"github.com/xloki21/alias/internal/controller/httpc/mw"
	"github.com/xloki21/alias/internal/domain"
	"github.com/xloki21/alias/internal/gen/go/pbuf/aliasapi"
	"github.com/xloki21/alias/internal/repository"
	"github.com/xloki21/alias/internal/repository/inmemory"
	"github.com/xloki21/alias/internal/repository/mongodb"
	"github.com/xloki21/alias/internal/service/alias"
	"github.com/xloki21/alias/pkg/kafker"
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

	producerAliasUsedCfg := cfg.GetProducerConfig("Alias Used Event Producer")
	producerAliasExpiredCfg := cfg.GetProducerConfig("Alias Expired Event Producer")

	producerAliasUsed := kafker.NewProducer(producerAliasUsedCfg.GetBrokersURI(), producerAliasUsedCfg.Topic, nil)

	producerAliasExpired := kafker.NewProducer(producerAliasExpiredCfg.GetBrokersURI(), producerAliasExpiredCfg.Topic, nil)
	var aliasService *alias.Alias

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
		aliasService = alias.NewAlias(producerAliasExpired, producerAliasUsed, aliasRepo, keyGen)

	case repository.InMemory:
		aliasRepo := inmemory.NewAliasRepository()
		aliasService = alias.NewAlias(producerAliasExpired, producerAliasUsed, aliasRepo, keyGen)

	default:
		zap.S().Fatalf("unknown storage type: %s", cfg.Storage.Type)
		return nil, domain.ErrUnknownStorageType
	}
	zap.S().Infow("core", zap.String("state", "selected storage type"), zap.String("type", string(cfg.Storage.Type)))

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(interceptors.LoggingInterceptor))
	reflection.Register(grpcServer)
	aliasapi.RegisterAliasAPIServer(grpcServer, grpcc.NewController(aliasService, cfg.Service.BaseURL))

	listener, err := net.Listen("tcp", cfg.Service.GRPC)
	if err != nil {
		zap.S().Fatalw("core", zap.String("application error", err.Error()))
		return nil, err
	}

	gwmux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	if err := aliasapi.RegisterAliasAPIHandlerFromEndpoint(ctx, gwmux, cfg.Service.GRPC, opts); err != nil {
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
	ctrlHTTP := httpc.NewController(aliasService, cfg.Service.BaseURL)
	app.initializeRoutes(ctrlHTTP)

	return app, nil
}

func (a *Application) Run(ctx context.Context) error {
	ctx, cancelFn := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	errChan := make(chan error)
	go func() {
		zap.S().Infow("HTTP", zap.String("state", fmt.Sprintf("start server, listening on %s", a.HTTPServer.Addr)))
		if err := a.HTTPServer.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}
	}()

	go func() {
		zap.S().Infow("gRPC-gw", zap.String("state", fmt.Sprintf("start server, listening on %s", a.GRPCGatewayServer.Addr)))
		if err := a.GRPCGatewayServer.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				errChan <- err
			}
		}
	}()

	go func() {
		zap.S().Infow("gRPC", zap.String("state", fmt.Sprintf("start server, listening on %s", a.grpcListener.Addr())))
		if err := a.GRPCServer.Serve(a.grpcListener); err != nil {
			zap.S().Fatalw("core", zap.String("application error", err.Error()))
		}
	}()

	select {
	case <-ctx.Done():
		zap.S().Infow("core", zap.String("state", "application graceful shutdown begins"))
		zap.S().Infow("core", zap.String("state", "shutting down http server"))
		err := a.HTTPServer.Shutdown(ctx)
		if err != nil {
			return err
		}
		zap.S().Infow("core", zap.String("state", "HTTP server stopped"))
		zap.S().Infow("core", zap.String("state", "shutting down gRPC-gateway server"))

		err = a.GRPCGatewayServer.Shutdown(ctx)
		if err != nil {
			return err
		}
		zap.S().Infow("core", zap.String("state", "gRPC-gateway server stopped"))

		zap.S().Infow("core", zap.String("state", "shutting down gRPC-server"))
		a.GRPCServer.GracefulStop()
		zap.S().Infow("core", zap.String("state", "gRPC-server stopped"))
		return nil
	case err := <-errChan:
		zap.S().Fatalw("core", zap.String("application error", err.Error()))
		return err
	}
}

func (a *Application) initializeRoutes(ctrl *httpc.Controller) {
	zap.S().Infow("core", zap.String("state", "initialize http-routes"))
	mux := http.NewServeMux()
	mux.HandleFunc(endpointAlias, mw.Use(ctrl.CreateAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(endpointHealthcheck, mw.Use(ctrl.Healthcheck, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(endpointAlias+"/{key}", mw.Use(ctrl.RemoveAlias, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	mux.HandleFunc(endpointRedirect+"/{key}", mw.Use(ctrl.Redirect, mw.RequestThrottler, mw.Logging, mw.PanicRecovery))
	a.HTTPServer.Handler = mux
}
