package main

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/xloki21/alias/internal/app/manager"
	"github.com/xloki21/alias/internal/config"
	"go.uber.org/zap"
	"log"
	"os"
)

func main() {
	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("error loading .env file")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to init application config: " + err.Error())
	}
	zap.S().Infow("core", zap.String("state", "application config loaded"))

	application, err := manager.New(cfg)
	if err != nil {
		zap.S().Fatal("failed to start application", zap.Error(err))
	}

	if err := application.Run(context.Background()); err != nil {
		zap.S().Fatal("failed to start application", zap.Error(err))
	}
}