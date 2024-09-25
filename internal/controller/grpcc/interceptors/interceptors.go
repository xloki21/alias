package interceptors

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"time"
)

const maxUrlListLength = 10

// LoggingInterceptor prints log
func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	zap.S().Infow("gRPC", zap.String("method", info.FullMethod),
		zap.String("status", "received"))
	tic := time.Now()

	resp, err := handler(ctx, req)
	duration := time.Since(tic)

	durationString := fmt.Sprintf("%dms", duration.Milliseconds())
	if duration.Milliseconds() < 2 {
		durationString = fmt.Sprintf("%dμs", duration.Microseconds())
	}

	zap.S().Infow("gRPC", zap.String("method", info.FullMethod),
		zap.String("status", "processed"),
		zap.String("duration", durationString))
	return resp, err
}
