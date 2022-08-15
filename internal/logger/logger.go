package logger

import (
	"context"
	"log"
)

type ctxKey int

const (
	_ ctxKey = iota
	loggerCtx
)

// FromContext returns the Logger instance from the ctx, or log.Default() if
// none.
func FromContext(ctx context.Context) *log.Logger {
	logger, ok := ctx.Value(loggerCtx).(*log.Logger)
	if ok {
		return logger
	}
	return log.Default()
}

// WithLogger injects an additional logger into ctx.
func WithLogger(ctx context.Context, logger *log.Logger) context.Context {
	return context.WithValue(ctx, loggerCtx, logger)
}
