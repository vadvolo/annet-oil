package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"annet-oil/internal/config"
)

type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserKey      contextKey = "user"
)

var (
	globalLogger *slog.Logger
	once         sync.Once
)

func Init(cfg config.LoggingConfig) (*slog.Logger, error) {
	var err error
	once.Do(func() {
		globalLogger, err = newLogger(cfg)
	})
	return globalLogger, err
}

func Get() *slog.Logger {
	if globalLogger == nil {
		globalLogger = slog.Default()
	}
	return globalLogger
}

func newLogger(cfg config.LoggingConfig) (*slog.Logger, error) {
	level := parseLevel(cfg.Level)

	var writers []io.Writer
	writers = append(writers, os.Stdout)

	if cfg.Output != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.Output), 0755); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		writers = append(writers, file)
	}

	w := io.MultiWriter(writers...)

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: level == slog.LevelDebug,
	}

	if cfg.Format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler), nil
}

func parseLevel(s string) slog.Level {
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

func WithContext(ctx context.Context) *slog.Logger {
	logger := Get()

	if reqID, ok := ctx.Value(RequestIDKey).(string); ok && reqID != "" {
		logger = logger.With("request_id", reqID)
	}
	if user, ok := ctx.Value(UserKey).(string); ok && user != "" {
		logger = logger.With("user", user)
	}

	return logger
}

func Debug(msg string, args ...any) {
	Get().Debug(msg, args...)
}

func Info(msg string, args ...any) {
	Get().Info(msg, args...)
}

func Warn(msg string, args ...any) {
	Get().Warn(msg, args...)
}

func Error(msg string, args ...any) {
	Get().Error(msg, args...)
}
