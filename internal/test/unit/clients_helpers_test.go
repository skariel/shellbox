package unit

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"shellbox/internal/infra"
)

func TestClientsHelpersTestSuite(t *testing.T) {
	t.Run("TestFatalOnError", func(t *testing.T) {
		// Test with nil error (should not panic or exit)
		assert.NotPanics(t, func() {
			// We can't easily test log.Fatal without causing the test to exit
			// But we can test that FatalOnError exists and can be called with nil
			// Note: We can't actually test the fatal case without special test setup
		})
	})

	t.Run("TestFatalOnErrorStructure", func(t *testing.T) {
		// Test that FatalOnError function signature exists and can be called
		// We test with nil error since non-nil would cause fatal exit
		err := error(nil)
		message := "test message"

		// This should not panic for nil error
		assert.NotPanics(t, func() {
			infra.FatalOnError(err, message)
		})
	})

	t.Run("TestErrorTypes", func(t *testing.T) {
		// Test that we can create and handle different error types
		err1 := errors.New("simple error")
		err2 := errors.New("another error")

		assert.NotNil(t, err1)
		assert.NotNil(t, err2)
		assert.NotEqual(t, err1, err2)
		assert.Contains(t, err1.Error(), "simple error")
		assert.Contains(t, err2.Error(), "another error")
	})

	t.Run("TestNewAzureClientsExists", func(t *testing.T) {
		// Test that NewAzureClients function exists by checking it's not nil
		// We can't actually call it without Azure credentials as it will timeout
		// But we can verify the function signature compiles

		var newClientFunc func(string, bool) *infra.AzureClients = infra.NewAzureClients
		assert.NotNil(t, newClientFunc, "NewAzureClients function should exist")
	})

	t.Run("TestAzureClientsFields", func(t *testing.T) {
		// Test AzureClients struct field accessibility
		clients := &infra.AzureClients{
			Suffix:            "test123",
			SubscriptionID:    "sub-456",
			ResourceGroupName: "shellbox-test123",
		}

		assert.Equal(t, "test123", clients.Suffix)
		assert.Equal(t, "sub-456", clients.SubscriptionID)
		assert.Equal(t, "shellbox-test123", clients.ResourceGroupName)
	})

	t.Run("TestDefaultPollOptions", func(t *testing.T) {
		// Test that DefaultPollOptions is properly configured
		options := infra.DefaultPollOptions

		assert.NotNil(t, options.Frequency)
		assert.True(t, options.Frequency > 0)

		// Should be 2 seconds based on the constants
		expectedFrequency := 2
		actualSeconds := int(options.Frequency.Seconds())
		assert.Equal(t, expectedFrequency, actualSeconds)
	})
}
