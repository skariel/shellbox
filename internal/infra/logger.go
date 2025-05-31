package infra

import (
	"log/slog"
	"os"
)

// NewLogger creates a standardized JSON logger for the application
func NewLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// SetDefaultLogger configures the default slog logger to use JSON format
func SetDefaultLogger() {
	slog.SetDefault(NewLogger())
}
