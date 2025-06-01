//go:build unit

package unit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// RetryTestSuite tests the retry mechanism functionality
type RetryTestSuite struct {
	suite.Suite
	env *test.TestEnvironment
}

// SetupSuite runs once before all tests in the suite
func (suite *RetryTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestRetryOperationSuccess tests successful operation on first try
func (suite *RetryTestSuite) TestRetryOperationSuccess() {
	ctx := context.Background()
	callCount := 0

	operation := func(ctx context.Context) error {
		callCount++
		return nil // Success on first try
	}

	err := infra.RetryOperation(ctx, operation, 5*time.Second, 100*time.Millisecond, "test-operation")

	assert.NoError(suite.T(), err, "Operation should succeed")
	assert.Equal(suite.T(), 1, callCount, "Operation should be called exactly once")
}

// TestRetryOperationEventualSuccess tests operation that succeeds after retries
func (suite *RetryTestSuite) TestRetryOperationEventualSuccess() {
	ctx := context.Background()
	callCount := 0

	operation := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil // Success on third try
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 5*time.Second, 50*time.Millisecond, "test-operation")
	elapsed := time.Since(start)

	assert.NoError(suite.T(), err, "Operation should eventually succeed")
	assert.Equal(suite.T(), 3, callCount, "Operation should be called three times")
	assert.GreaterOrEqual(suite.T(), elapsed, 100*time.Millisecond, "Should wait between retries")
	assert.Less(suite.T(), elapsed, 1*time.Second, "Should not take too long")
}

// TestRetryOperationTimeout tests operation that times out
func (suite *RetryTestSuite) TestRetryOperationTimeout() {
	ctx := context.Background()
	callCount := 0

	operation := func(ctx context.Context) error {
		callCount++
		return errors.New("persistent failure")
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 200*time.Millisecond, 50*time.Millisecond, "test-operation")
	elapsed := time.Since(start)

	assert.Error(suite.T(), err, "Operation should fail due to timeout")
	assert.Contains(suite.T(), err.Error(), "timeout waiting for test-operation", "Error should mention timeout")
	assert.Contains(suite.T(), err.Error(), "persistent failure", "Error should include last error")
	assert.GreaterOrEqual(suite.T(), elapsed, 200*time.Millisecond, "Should respect timeout")
	assert.Less(suite.T(), elapsed, 500*time.Millisecond, "Should not wait much longer than timeout")
	assert.GreaterOrEqual(suite.T(), callCount, 3, "Should attempt multiple times")
}

// TestRetryOperationContextCancellation tests behavior when context is cancelled
func (suite *RetryTestSuite) TestRetryOperationContextCancellation() {
	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0

	operation := func(ctx context.Context) error {
		callCount++
		if callCount == 2 {
			cancel() // Cancel after second attempt
		}
		return errors.New("persistent failure")
	}

	err := infra.RetryOperation(ctx, operation, 5*time.Second, 50*time.Millisecond, "test-operation")

	assert.Error(suite.T(), err, "Operation should fail due to cancellation")
	assert.Contains(suite.T(), err.Error(), "timeout waiting for test-operation", "Error should mention timeout")
	assert.GreaterOrEqual(suite.T(), callCount, 2, "Should call operation at least twice")
}

// TestRetryOperationWithShortTimeout tests very short timeout
func (suite *RetryTestSuite) TestRetryOperationWithShortTimeout() {
	ctx := context.Background()
	callCount := 0

	operation := func(ctx context.Context) error {
		callCount++
		// Fast operation that always fails
		return errors.New("fast failing operation")
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 50*time.Millisecond, 20*time.Millisecond, "fast-operation")
	elapsed := time.Since(start)

	assert.Error(suite.T(), err, "Operation should timeout")
	assert.Contains(suite.T(), err.Error(), "timeout waiting for fast-operation", "Error should mention timeout")
	assert.GreaterOrEqual(suite.T(), callCount, 1, "Should call at least once")
	assert.LessOrEqual(suite.T(), callCount, 4, "Should not call too many times with short timeout")
	assert.GreaterOrEqual(suite.T(), elapsed, 50*time.Millisecond, "Should respect minimum timeout")
}

// TestRetryOperationWithLongInterval tests retry with longer intervals
func (suite *RetryTestSuite) TestRetryOperationWithLongInterval() {
	ctx := context.Background()
	callCount := 0

	operation := func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	start := time.Now()
	err := infra.RetryOperation(ctx, operation, 1*time.Second, 200*time.Millisecond, "test-operation")
	elapsed := time.Since(start)

	assert.NoError(suite.T(), err, "Operation should succeed")
	assert.Equal(suite.T(), 3, callCount, "Should call operation three times")
	assert.GreaterOrEqual(suite.T(), elapsed, 400*time.Millisecond, "Should wait for intervals")
}

// TestRetryOperationNoTimeout tests operation without timeout (using parent context)
func (suite *RetryTestSuite) TestRetryOperationNoTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	callCount := 0
	operation := func(ctx context.Context) error {
		callCount++
		return errors.New("persistent failure") // Always fail to ensure timeout
	}

	err := infra.RetryOperation(ctx, operation, 10*time.Second, 50*time.Millisecond, "test-operation")

	// Should timeout from parent context, not from RetryOperation timeout
	assert.Error(suite.T(), err, "Should timeout from parent context")
	assert.GreaterOrEqual(suite.T(), callCount, 4, "Should make multiple attempts before timing out")
}

// TestRetryOperationContextValuePropagation tests that context values are propagated
func (suite *RetryTestSuite) TestRetryOperationContextValuePropagation() {
	type contextKey string
	key := contextKey("test-key")
	expectedValue := "test-value"

	ctx := context.WithValue(context.Background(), key, expectedValue)

	operation := func(ctx context.Context) error {
		value := ctx.Value(key)
		assert.Equal(suite.T(), expectedValue, value, "Context value should be propagated")
		return nil // Success
	}

	err := infra.RetryOperation(ctx, operation, 1*time.Second, 50*time.Millisecond, "context-test")
	assert.NoError(suite.T(), err, "Operation should succeed")
}

// TestRetryOperationErrorWrapping tests that errors are properly wrapped
func (suite *RetryTestSuite) TestRetryOperationErrorWrapping() {
	ctx := context.Background()
	originalError := errors.New("original error message")

	operation := func(ctx context.Context) error {
		return originalError
	}

	err := infra.RetryOperation(ctx, operation, 100*time.Millisecond, 20*time.Millisecond, "error-wrap-test")

	require.Error(suite.T(), err, "Should return an error")
	assert.Contains(suite.T(), err.Error(), "timeout waiting for error-wrap-test", "Should contain operation name")
	assert.ErrorIs(suite.T(), err, originalError, "Should wrap the original error")
}

// TestRetryOperationConcurrentSafety tests that retry operations are safe to run concurrently
func (suite *RetryTestSuite) TestRetryOperationConcurrentSafety() {
	ctx := context.Background()

	// Run multiple retry operations concurrently
	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			callCount := 0
			operation := func(ctx context.Context) error {
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
		assert.NoError(suite.T(), err, "Concurrent retry operation should succeed")
	}
}

// Run the test suite
func TestRetryTestSuite(t *testing.T) {
	suite.Run(t, new(RetryTestSuite))
}
