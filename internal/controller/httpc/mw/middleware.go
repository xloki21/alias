package mw

import (
	"fmt"
	"go.uber.org/zap"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"
)

type Middleware func(next http.HandlerFunc) http.HandlerFunc

const maxIncomingRequests = 100

var totalRequestsInProcessing atomic.Int32

func Use(handlerFunc http.HandlerFunc, middlewares ...Middleware) http.HandlerFunc {
	handlerFn := handlerFunc
	for _, mw := range middlewares {
		handlerFn = mw(handlerFn)
	}
	return handlerFn
}

func Logging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		zap.S().Infow("HTTP",
			zap.String("request", r.Method),
			zap.String("status", "received"),
			zap.String("uri", r.RequestURI))

		next(w, r)

		duration := time.Since(start)

		durationString := fmt.Sprintf("%dms", duration.Milliseconds())
		if duration.Milliseconds() < 2 {
			durationString = fmt.Sprintf("%dμs", duration.Microseconds())
		}

		zap.S().Infow("HTTP",
			zap.String("request", r.Method),
			zap.String("status", "processed"),
			zap.String("duration", durationString))
	}
}

func PanicRecovery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				zap.S().Errorw("panic recovered! ", zap.ByteString("stack", debug.Stack()))
			}
		}()
		next(w, req)
	}
}

func RequestThrottler(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		totalRequestsInProcessing.Add(1)
		defer totalRequestsInProcessing.Add(-1)
		if totalRequestsInProcessing.Load() >= maxIncomingRequests {
			zap.S().Error(zap.String("error", "too many requests"))
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		}
		next(w, r)
	}
}
