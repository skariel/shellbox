package unit

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"shellbox/internal/infra"
)

// TestRetryOperationSuccess tests successful operation on first try
func TestRetryOperationSuccess(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func(_ context.Context) error {
		callCount++
		return nil // Success on first try
	}

	err := infra.RetryOperation(ctx, operation, 5*time.Second, 100*time.Millisecond, "test-operation")
	if err != nil {
		t.Errorf("Operation should succeed, got error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Operation should be called exactly once, got %d calls", callCount)
	}
}

// TestRetryOperationEventualSuccess tests operation that succeeds after retries
func TestRetryOperationEventualSuccess(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func(_ context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil // Success on third try
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 5*time.Second, 50*time.Millisecond, "test-operation")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Operation should eventually succeed, got error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Operation should be called three times, got %d calls", callCount)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("Should wait between retries, elapsed: %v", elapsed)
	}
	if elapsed >= 1*time.Second {
		t.Errorf("Should not take too long, elapsed: %v", elapsed)
	}
}

// TestRetryOperationTimeout tests operation that times out
func TestRetryOperationTimeout(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func(_ context.Context) error {
		callCount++
		return errors.New("persistent failure")
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 200*time.Millisecond, 50*time.Millisecond, "test-operation")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Operation should fail due to timeout")
	}
	if err != nil && !strings.Contains(err.Error(), "timeout waiting for test-operation") {
		t.Errorf("Error should mention timeout, got: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "persistent failure") {
		t.Errorf("Error should include last error, got: %v", err)
	}
	if elapsed < 200*time.Millisecond {
		t.Errorf("Should respect timeout, elapsed: %v", elapsed)
	}
	if elapsed >= 500*time.Millisecond {
		t.Errorf("Should not wait much longer than timeout, elapsed: %v", elapsed)
	}
	if callCount < 3 {
		t.Errorf("Should attempt multiple times, got %d calls", callCount)
	}
}

// TestRetryOperationContextCancellation tests behavior when context is cancelled
func TestRetryOperationContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	operation := func(_ context.Context) error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after second attempt
		}
		return errors.New("persistent failure")
	}

	err := infra.RetryOperation(ctx, operation, 5*time.Second, 50*time.Millisecond, "test-operation")

	if err == nil {
		t.Error("Operation should fail due to cancellation")
	}
	if err != nil && !strings.Contains(err.Error(), "timeout waiting for test-operation") {
		t.Errorf("Error should mention timeout, got: %v", err)
	}
	if callCount < 2 {
		t.Errorf("Should call operation at least twice, got %d calls", callCount)
	}
}

// TestRetryOperationWithShortTimeout tests very short timeout
func TestRetryOperationWithShortTimeout(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func(_ context.Context) error {
		callCount++
		// Fast operation that always fails
		return errors.New("fast failing operation")
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 50*time.Millisecond, 20*time.Millisecond, "fast-operation")
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Operation should timeout")
	}
	if !strings.Contains(err.Error(), "timeout waiting for fast-operation") {
		t.Error("Error should mention timeout")
	}
	if callCount < 1 {
		t.Error("Should call at least once")
	}
	if callCount > 4 {
		t.Error("Should not call too many times with short timeout")
	}
	if elapsed < 50*time.Millisecond {
		t.Error("Should respect minimum timeout")
	}
}

// TestRetryOperationWithLongInterval tests retry with longer intervals
func TestRetryOperationWithLongInterval(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	operation := func(_ context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 1*time.Second, 200*time.Millisecond, "test-operation")
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Operation should succeed, got error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("Should call operation three times, got %d", callCount)
	}
	if elapsed < 400*time.Millisecond {
		t.Error("Should wait for intervals")
	}
}

// TestRetryOperationNoTimeout tests operation without timeout (using parent context)
func TestRetryOperationNoTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	callCount := 0
	operation := func(_ context.Context) error {
		callCount++
		return errors.New("persistent failure") // Always fail to ensure timeout
	}

	err := infra.RetryOperation(ctx, operation, 10*time.Second, 50*time.Millisecond, "test-operation")

	// Should timeout from parent context, not from RetryOperation timeout
	if err == nil {
		t.Error("Should timeout from parent context")
	}
	if callCount < 4 {
		t.Errorf("Should make multiple attempts before timing out, got %d", callCount)
	}
}

// TestRetryOperationContextValuePropagation tests that context values are propagated
func TestRetryOperationContextValuePropagation(t *testing.T) {
	type contextKey string
	key := contextKey("test-key")
	expectedValue := "test-value"

	ctx := context.WithValue(context.Background(), key, expectedValue)

	operation := func(ctx context.Context) error {
		value := ctx.Value(key)
		if value != expectedValue {
			t.Errorf("Context value should be propagated, expected %v, got %v", expectedValue, value)
		}
		return nil // Success
	}

	err := infra.RetryOperation(ctx, operation, 1*time.Second, 50*time.Millisecond, "context-test")
	if err != nil {
		t.Errorf("Operation should succeed, got error: %v", err)
	}
}

// TestRetryOperationErrorWrapping tests that errors are properly wrapped
func TestRetryOperationErrorWrapping(t *testing.T) {
	ctx := context.Background()
	originalError := errors.New("original error message")

	operation := func(_ context.Context) error {
		return originalError
	}

	err := infra.RetryOperation(ctx, operation, 100*time.Millisecond, 20*time.Millisecond, "error-wrap-test")

	if err == nil {
		t.Fatal("Should return an error")
	}
	if !strings.Contains(err.Error(), "timeout waiting for error-wrap-test") {
		t.Errorf("Should contain operation name, got error: %v", err)
	}
	if !errors.Is(err, originalError) {
		t.Errorf("Should wrap the original error, got: %v", err)
	}
}

// TestRetryOperationConcurrentSafety tests that retry operations are safe to run concurrently
func TestRetryOperationConcurrentSafety(t *testing.T) {
	ctx := context.Background()

	// Run multiple retry operations concurrently
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(_ int) {
			callCount := 0
			operation := func(_ context.Context) error {
				callCount++
				if callCount < 2 {
					return errors.New("temporary failure")
				}
				return nil
			}

			err := infra.RetryOperation(ctx, operation, 1*time.Second, 50*time.Millisecond, "concurrent-test")
			results <- err
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent retry operation should succeed, got error: %v", err)
		}
	}
}
