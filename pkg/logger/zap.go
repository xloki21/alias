package logger

import (
	"github.com/xloki21/alias/internal/config/base"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"time"
)

func NewZapLogger(cfg *base.LoggerConfig) (*zap.Logger, error) {
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
