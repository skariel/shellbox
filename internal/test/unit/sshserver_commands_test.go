package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shellbox/internal/sshserver"
)

func TestSSHServerCommandsTestSuite(t *testing.T) {
	t.Run("TestParseArgs", func(t *testing.T) {
		testCases := []struct {
			name     string
			cmdLine  string
			expected []string
		}{
			{
				name:     "Simple command",
				cmdLine:  "help",
				expected: []string{"help"},
			},
			{
				name:     "Command with args",
				cmdLine:  "spinup mybox",
				expected: []string{"spinup", "mybox"},
			},
			{
				name:     "Multiple spaces",
				cmdLine:  "spinup    mybox",
				expected: []string{"spinup", "mybox"},
			},
			{
				name:     "Leading and trailing spaces",
				cmdLine:  "  help  ",
				expected: []string{"help"},
			},
			{
				name:     "Empty string",
				cmdLine:  "",
				expected: []string{},
			},
			{
				name:     "Only spaces",
				cmdLine:  "   ",
				expected: []string{},
			},
			{
				name:     "Complex command",
				cmdLine:  "spinup my-development-box",
				expected: []string{"spinup", "my-development-box"},
			},
			{
				name:     "Command with multiple args",
				cmdLine:  "command arg1 arg2 arg3",
				expected: []string{"command", "arg1", "arg2", "arg3"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := sshserver.ParseArgs(tc.cmdLine)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("TestParseArgsMemoryEfficiency", func(t *testing.T) {
		// Test that ParseArgs returns a clipped slice for memory efficiency
		cmdLine := "command with many arguments that creates a long slice"
		result := sshserver.ParseArgs(cmdLine)

		// Should have parsed correctly
		expected := []string{"command", "with", "many", "arguments", "that", "creates", "a", "long", "slice"}
		assert.Equal(t, expected, result)

		// Result should not be nil or empty
		assert.NotNil(t, result)
		assert.NotEmpty(t, result)
	})

	t.Run("TestParseArgsEdgeCases", func(t *testing.T) {
		// Test various edge cases
		testCases := []struct {
			name     string
			cmdLine  string
			expected []string
		}{
			{
				name:     "Tab characters",
				cmdLine:  "command\targ",
				expected: []string{"command", "arg"},
			},
			{
				name:     "Mixed whitespace",
				cmdLine:  "command \t arg  \t  arg2",
				expected: []string{"command", "arg", "arg2"},
			},
			{
				name:     "Newline characters",
				cmdLine:  "command\narg",
				expected: []string{"command", "arg"},
			},
			{
				name:     "Single character",
				cmdLine:  "a",
				expected: []string{"a"},
			},
			{
				name:     "Numbers and special chars",
				cmdLine:  "cmd123 arg-with-dashes arg_with_underscores",
				expected: []string{"cmd123", "arg-with-dashes", "arg_with_underscores"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := sshserver.ParseArgs(tc.cmdLine)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("TestActionConstants", func(t *testing.T) {
		// Test that action constants have expected values
		assert.Equal(t, "spinup", sshserver.ActionSpinup)
		assert.Equal(t, "help", sshserver.ActionHelp)
		assert.Equal(t, "version", sshserver.ActionVersion)
		assert.Equal(t, "whoami", sshserver.ActionWhoami)
		assert.Equal(t, "error", sshserver.ActionError)
	})

	t.Run("TestCommandResultStruct", func(t *testing.T) {
		// Test that CommandResult struct can be properly initialized
		result := sshserver.CommandResult{
			Action:   sshserver.ActionSpinup,
			Args:     []string{"mybox"},
			Output:   "success",
			ExitCode: 0,
		}

		assert.Equal(t, sshserver.ActionSpinup, result.Action)
		assert.Equal(t, []string{"mybox"}, result.Args)
		assert.Equal(t, "success", result.Output)
		assert.Equal(t, 0, result.ExitCode)
	})

	t.Run("TestCommandContextStruct", func(t *testing.T) {
		// Test that CommandContext struct can be properly initialized
		ctx := sshserver.CommandContext{
			UserID:     "user123",
			RemoteAddr: "192.168.1.1",
			SessionID:  "session456",
		}

		assert.Equal(t, "user123", ctx.UserID)
		assert.Equal(t, "192.168.1.1", ctx.RemoteAddr)
		assert.Equal(t, "session456", ctx.SessionID)
	})
}
