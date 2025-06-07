package unit

import (
	"log/slog"
	"testing"

	"shellbox/internal/infra"
)

func TestLoggerTestSuite(t *testing.T) {
	t.Run("TestNewLogger", func(t *testing.T) {
		logger := infra.NewLogger()

		if logger == nil {
			t.Errorf("NewLogger() should not return nil")
		}
		// Check that logger implements the basic slog.Logger interface
		// Since slog.Logger is concrete, we can check the type directly
		if logger == nil {
			t.Error("Logger should not be nil")
		}
	})

	t.Run("TestSetDefaultLogger", func(_ *testing.T) {
		// This should not panic and should set up the default logger
		infra.SetDefaultLogger()

		// After setting, the default logger should be available
		// We can't easily test the exact logger instance, but we can ensure it doesn't panic
		slog.Info("test message from default logger")
	})

	t.Run("TestLoggerBasicFunctionality", func(_ *testing.T) {
		logger := infra.NewLogger()

		// These should not panic when called
		logger.Info("test info message", "key", "value")
		logger.Debug("test debug message", "key", "value")
		logger.Warn("test warn message", "key", "value")
		logger.Error("test error message", "key", "value")
	})

	t.Run("TestLoggerWithAttributes", func(_ *testing.T) {
		logger := infra.NewLogger()

		// Test logger with various attribute types
		logger.Info("test with attributes",
			"string", "value",
			"int", 42,
			"bool", true,
			"nil", nil,
		)
	})
}
