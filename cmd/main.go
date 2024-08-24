package main

import (
	"context"
	"github.com/joho/godotenv"
	"github.com/xloki21/alias/internal/app"
	"github.com/xloki21/alias/internal/app/config"
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

	cfg, err := config.MustLoad()
	if err != nil {
		log.Fatal("failed to init application config: " + err.Error())
	}

	application, err := app.New(cfg)
	if err != nil {
		zap.S().Fatal("failed to start application", zap.Error(err))
	}
	if err := application.Run(context.Background()); err != nil {
		zap.S().Fatal("failed to start application", zap.Error(err))
	}
}
