package unit

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"shellbox/internal/infra"
)

func TestLoggerTestSuite(t *testing.T) {
	t.Run("TestNewLogger", func(t *testing.T) {
		logger := infra.NewLogger()

		assert.NotNil(t, logger)
		assert.IsType(t, &slog.Logger{}, logger)
	})

	t.Run("TestSetDefaultLogger", func(t *testing.T) {
		// This should not panic and should set up the default logger
		assert.NotPanics(t, func() {
			infra.SetDefaultLogger()
		})

		// After setting, the default logger should be available
		// We can't easily test the exact logger instance, but we can ensure it doesn't panic
		assert.NotPanics(t, func() {
			slog.Info("test message from default logger")
		})
	})

	t.Run("TestLoggerBasicFunctionality", func(t *testing.T) {
		logger := infra.NewLogger()

		// These should not panic when called
		assert.NotPanics(t, func() {
			logger.Info("test info message", "key", "value")
		})

		assert.NotPanics(t, func() {
			logger.Debug("test debug message", "key", "value")
		})

		assert.NotPanics(t, func() {
			logger.Warn("test warn message", "key", "value")
		})

		assert.NotPanics(t, func() {
			logger.Error("test error message", "key", "value")
		})
	})

	t.Run("TestLoggerWithAttributes", func(t *testing.T) {
		logger := infra.NewLogger()

		// Test logger with various attribute types
		assert.NotPanics(t, func() {
			logger.Info("test with attributes",
				"string", "value",
				"int", 42,
				"bool", true,
				"nil", nil,
			)
		})
	})
}
