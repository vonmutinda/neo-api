package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/vonmutinda/neo/internal/config"
)

type contextKey struct{}

// NewLogger creates a structured JSON logger suitable for production audit trails.
func NewLogger(logConf *config.Log) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     parseLogLevel(logConf.Level),
		AddSource: true,
	})
	return slog.New(handler)
}

// WithContext returns a new context with the logger attached.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext extracts the logger from context. Falls back to the default
// slog logger if none is found.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// With returns a logger with additional key-value pairs attached.
// Useful for adding request-scoped fields (user_id, request_id, etc.).
func With(ctx context.Context, args ...any) *slog.Logger {
	return FromContext(ctx).With(args...)
}

// NewTestLogger creates a debug-level text logger suitable for tests.
// Output goes to stderr so `go test -v` captures it.
func NewTestLogger(_ testing.TB) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func parseLogLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
