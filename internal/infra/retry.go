package infra

import (
	"context"
	"fmt"
	"log"
	"time"
)

// RetryOptions configures the retry behavior
type RetryOptions struct {
	Timeout     time.Duration
	Interval    time.Duration
	Operation   string // Name of operation for logging
	MaxAttempts int    // Maximum number of attempts (0 for unlimited)
}

// DefaultRetryOptions returns standard retry settings
func DefaultRetryOptions() *RetryOptions {
	return &RetryOptions{
		Timeout:     2 * time.Minute,
		Interval:    5 * time.Second,
		MaxAttempts: 0,
	}
}

// RetryWithTimeout executes an operation with retries until success or timeout
func RetryWithTimeout[T any](ctx context.Context, opts *RetryOptions, operation func(context.Context) (T, error)) (T, error) {
	if opts == nil {
		opts = DefaultRetryOptions()
	}

	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	var lastErr error
	var zero T

	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return zero, fmt.Errorf("timeout waiting for %s: %w", opts.Operation, lastErr)
			}
			return zero, fmt.Errorf("timeout waiting for %s", opts.Operation)
		case <-ticker.C:
			result, err := operation(ctx)
			if err == nil {
				if opts.Operation != "" {
					log.Printf("%s completed successfully", opts.Operation)
				}
				return result, nil
			}
			lastErr = err
			if opts.Operation != "" {
				log.Printf("retrying %s: %v", opts.Operation, err)
			}
		}
	}
}
