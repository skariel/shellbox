package infra

import (
	"log/slog"
	"os"
)

// NewLogger creates a standardized JSON logger for the application
func NewLogger() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

// SetDefaultLogger configures the default slog logger to use JSON format
func SetDefaultLogger() {
	slog.SetDefault(NewLogger())
}
