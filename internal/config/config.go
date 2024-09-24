package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/viper"
	"github.com/xloki21/alias/internal/repository"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Service struct {
	HTTP        string `mapstructure:"http"`
	GRPC        string `mapstructure:"grpc"`
	GRPCGateway string `mapstructure:"grpc-gateway"`
	BaseURL     string `mapstructure:"base-url"`
}

type LoggerConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
}

type Credentials struct {
	AuthSource string
	User       string
	Password   string
}

type MongoDBStorageConfig struct {
	URI         string `mapstructure:"uri"`
	Credentials Credentials
	Database    string `mapstructure:"database"`
}

type StorageConfig struct {
	Type    repository.Type       `yaml:"type"` // mongodb/postgres/inmemory
	MongoDB *MongoDBStorageConfig `mapstructure:"config" yaml:"config"`
}

type AppConfig struct {
	Service      Service       `mapstructure:"service"`
	Storage      StorageConfig `mapstructure:"storage"`
	LoggerConfig LoggerConfig  `mapstructure:"logger"`
}

func NewZapLogger(cfg LoggerConfig) (*zap.Logger, error) {
	zcfg := zap.NewProductionConfig()
	zcfg.EncoderConfig = zap.NewProductionEncoderConfig()
	zcfg.EncoderConfig.CallerKey = zapcore.OmitKey
	zcfg.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)
	zcfg.OutputPaths = []string{"stdout"}
	zcfg.Encoding = cfg.Encoding

	parsedLevel, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	zcfg.Level = zap.NewAtomicLevelAt(parsedLevel)

	return zcfg.Build()
}

func Load() (AppConfig, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/alias/")
	viper.AddConfigPath("$HOME/.alias")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("no config file found, using defaults\n")
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			viper.SetDefault("service.http", "localhost:8080")
			viper.SetDefault("service.grpc", "localhost:8081")
			viper.SetDefault("service.grpc-gateway", "localhost:8082")
			viper.SetDefault("service.base-url", "http://localhost:8080")
			viper.SetDefault("storage.type", repository.InMemory)
			viper.SetDefault("logger.level", "info")
			viper.SetDefault("logger.encoding", "json")

		} else {
			return AppConfig{}, err
		}
	}

	cfg := AppConfig{}
	if err := viper.Unmarshal(&cfg); err != nil {
		return AppConfig{}, err
	}

	logger, err := NewZapLogger(cfg.LoggerConfig)
	if err != nil {
		log.Fatal("failed to init logger: " + err.Error())
	}

	// create a new one global logger instance
	zap.ReplaceGlobals(logger)

	if cfg.Storage.Type == repository.MongoDB {
		requiredEnvVars := []string{
			"MONGO_USERNAME", "MONGO_PASSWORD", "MONGO_AUTHSOURCE",
		}

		for _, requiredEnvVar := range requiredEnvVars {
			if _, ok := os.LookupEnv(requiredEnvVar); !ok {
				zap.S().Errorf("missing environment variable %s", requiredEnvVar)
				return AppConfig{}, fmt.Errorf("missing environment variable %s", requiredEnvVar)
			}
		}
		cfg.Storage.MongoDB.Credentials = Credentials{
			User:       os.Getenv("MONGO_USERNAME"),
			Password:   os.Getenv("MONGO_PASSWORD"),
			AuthSource: os.Getenv("MONGO_AUTHSOURCE"),
		}
	} else {
		cfg.Storage.MongoDB = nil
	}

	return cfg, nil
}
