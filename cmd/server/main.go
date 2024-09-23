package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/xloki21/alias/internal/app"
	"github.com/xloki21/alias/internal/app/config"
	"go.uber.org/zap"
)

func main() {

	if _, err := os.Stat(".env"); err == nil {
		err := godotenv.Load()
		if err != nil {
			log.Fatal("error loading .env file")
		}
	}

	cfg, err := config.MustLoad()
	if err != nil {
		log.Fatal("failed to init application config: " + err.Error())
	}
	zap.S().Infow("core", zap.String("state", "application config loaded"))

	application, err := app.New(cfg)
	if err != nil {
		zap.S().Fatal("failed to start application", zap.Error(err))
	}

	if err := application.Run(context.Background()); err != nil {
		zap.S().Fatal("failed to start application", zap.Error(err))
	}
}
