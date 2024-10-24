package interceptors

import (
	"context"
	"encoding/json"
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"time"
)

// LoggingInterceptor prints log
func LoggingInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	zap.S().Infow("gRPC", zap.String("method", info.FullMethod),
		zap.String("status", "received"), zap.String("payload", string(payload)))
	tic := time.Now()

	resp, err := handler(ctx, req)

	duration := time.Since(tic)

	durationString := fmt.Sprintf("%dms", duration.Milliseconds())
	if duration.Milliseconds() < 2 {
		durationString = fmt.Sprintf("%dμs", duration.Microseconds())
	}

	if err != nil {
		return nil, err
	}

	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	zap.S().Infow("gRPC", zap.String("method", info.FullMethod),
		zap.String("status", "processed"),
		zap.String("duration", durationString),
		zap.String("response", string(jsonResponse)))
	return resp, err
}
