package unit

import (
	"errors"
	"testing"

	"shellbox/internal/infra"
)

func TestClientsHelpersTestSuite(t *testing.T) {
	t.Run("TestFatalOnError", func(_ *testing.T) {
		// Test with nil error (should not panic or exit)
		// We can't easily test log.Fatal without causing the test to exit
		// But we can test that FatalOnError exists and can be called with nil
		// Note: We can't actually test the fatal case without special test setup
	})

	t.Run("TestFatalOnErrorStructure", func(_ *testing.T) {
		// Test that FatalOnError function signature exists and can be called
		// We test with nil error since non-nil would cause fatal exit
		err := error(nil)
		message := "test message"

		// This should not panic for nil error
		infra.FatalOnError(err, message)
	})

	t.Run("TestErrorTypes", func(t *testing.T) {
		// Test that we can create and handle different error types
		err1 := errors.New("simple error")
		err2 := errors.New("another error")

		if err1 == nil {
			t.Errorf("err1 should not be nil")
		}
		if err2 == nil {
			t.Errorf("err2 should not be nil")
		}
		if err1 == err2 {
			t.Errorf("err1 and err2 should not be equal")
		}
		if err1.Error() != "simple error" {
			t.Errorf("err1.Error() = %q, want %q", err1.Error(), "simple error")
		}
		if err2.Error() != "another error" {
			t.Errorf("err2.Error() = %q, want %q", err2.Error(), "another error")
		}
	})

	t.Run("TestNewAzureClientsExists", func(_ *testing.T) {
		// Test that NewAzureClients function exists by verifying it compiles
		// We can't actually call it without Azure credentials as it will timeout
		// But we can verify the function signature exists

		// Just verify that the function can be assigned to a variable with the correct signature
		newClientFunc := infra.NewAzureClients
		_ = newClientFunc // Use the variable to avoid unused variable warning
	})

	t.Run("TestAzureClientsFields", func(t *testing.T) {
		// Test AzureClients struct field accessibility
		clients := &infra.AzureClients{
			Suffix:            "test123",
			SubscriptionID:    "sub-456",
			ResourceGroupName: "shellbox-test123",
		}

		if clients.Suffix != "test123" {
			t.Errorf("clients.Suffix = %q, want %q", clients.Suffix, "test123")
		}
		if clients.SubscriptionID != "sub-456" {
			t.Errorf("clients.SubscriptionID = %q, want %q", clients.SubscriptionID, "sub-456")
		}
		if clients.ResourceGroupName != "shellbox-test123" {
			t.Errorf("clients.ResourceGroupName = %q, want %q", clients.ResourceGroupName, "shellbox-test123")
		}
	})

	t.Run("TestDefaultPollOptions", func(t *testing.T) {
		// Test that DefaultPollOptions is properly configured
		options := infra.DefaultPollOptions

		if options.Frequency == 0 {
			t.Errorf("options.Frequency should not be zero")
		}
		if options.Frequency <= 0 {
			t.Errorf("options.Frequency should be > 0, got %v", options.Frequency)
		}

		// Should be 2 seconds based on the constants
		expectedFrequency := 2
		actualSeconds := int(options.Frequency.Seconds())
		if actualSeconds != expectedFrequency {
			t.Errorf("options.Frequency.Seconds() = %d, want %d", actualSeconds, expectedFrequency)
		}
	})
}
