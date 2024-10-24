package config

import (
	"fmt"
	"github.com/spf13/viper"
	"github.com/xloki21/alias/internal/config/base"
	"github.com/xloki21/alias/internal/repository"
	"github.com/xloki21/alias/pkg/logger"
	"go.uber.org/zap"
	"log"
	"os"
)

type Service struct {
	HTTP        string `mapstructure:"http"`
	GRPC        string `mapstructure:"grpc"`
	GRPCGateway string `mapstructure:"grpc-gateway"`
	BaseURL     string `mapstructure:"base-url"`
}

type AppConfig struct {
	Service      *Service              `mapstructure:"service"`
	Storage      *base.StorageConfig   `mapstructure:"storage"`
	LoggerConfig *base.LoggerConfig    `mapstructure:"logger"`
	Producers    []base.ProducerConfig `mapstructure:"producers"`
	Consumers    []base.ConsumerConfig `mapstructure:"consumers"`
}

func (c AppConfig) GetProducerConfig(name string) *base.ProducerConfig {
	for _, p := range c.Producers {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

func (c AppConfig) GetConsumerConfig(name string) *base.ConsumerConfig {
	for _, c := range c.Consumers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func Load() (AppConfig, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("/etc/app/")
	viper.AddConfigPath("./config") // TODO: fixit
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("failed to read application config file")
	}

	cfg := AppConfig{}
	if err := viper.Unmarshal(&cfg); err != nil {
		return AppConfig{}, err
	}

	logger, err := logger.NewZapLogger(cfg.LoggerConfig)
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
		cfg.Storage.MongoDB.Credentials = base.Credentials{
			User:       os.Getenv("MONGO_USERNAME"),
			Password:   os.Getenv("MONGO_PASSWORD"),
			AuthSource: os.Getenv("MONGO_AUTHSOURCE"),
		}
	} else {
		cfg.Storage.MongoDB = nil
	}

	return cfg, nil
}
