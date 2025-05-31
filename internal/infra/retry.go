package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RetryOperation executes an operation with retries until success or timeout
func RetryOperation(ctx context.Context, operation func(context.Context) error, timeout time.Duration, interval time.Duration, operationName string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastErr error

	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("timeout waiting for %s: %w", operationName, lastErr)
			}
			return fmt.Errorf("timeout waiting for %s", operationName)
		case <-ticker.C:
			err := operation(ctx)
			if err == nil {
				slog.Info("operation completed successfully", "operation", operationName)
				return nil
			}
			lastErr = err
			slog.Warn("retrying operation", "operation", operationName, "error", err)
		}
	}
}
