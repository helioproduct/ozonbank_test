package logger

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func FromContext(ctx context.Context) *slog.Logger {
	l, ok := ctx.Value(ctxKey{}).(*slog.Logger)
	if !ok || l == nil {
		return slog.Default()
	}
	return l
}
