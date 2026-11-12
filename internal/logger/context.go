package logger

import (
	"context"
	"log/slog"
)

type loggerContextKey struct{}

// WithContext stores a logger instance inside context.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, l)
}

// FromContext extracts the logger from context. If absent, returns the default global logger.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	return slog.Default()
}

// WithAttrs returns a new context with attributes appended to its context logger.
func WithAttrs(ctx context.Context, args ...any) context.Context {
	cur := FromContext(ctx)
	return WithContext(ctx, cur.With(args...))
}
