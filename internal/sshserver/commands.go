package sshserver

import (
	"bytes"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// Command action constants
const (
	ActionSpinup  = "spinup"
	ActionConnect = "connect"
	ActionHelp    = "help"
	ActionVersion = "version"
	ActionWhoami  = "whoami"
	ActionError   = "error"
)

// CommandContext represents the SSH session context
type CommandContext struct {
	UserID     string // Full public key
	RemoteAddr string
	SessionID  string
}

// CommandResult represents the result of executing a command
type CommandResult struct {
	Action   string   // One of the Action* constants
	Args     []string // Command arguments
	Output   string   // Help/error messages from Cobra
	ExitCode int
}

// parseCommand parses an SSH command using Cobra and returns the result
func parseCommand(cmdLine string) CommandResult {
	if cmdLine == "" {
		return CommandResult{
			Action:   ActionError,
			Output:   "no command provided",
			ExitCode: 1,
		}
	}

	var result CommandResult
	rootCmd := createCobraCommand(&result)

	// Capture Cobra's output (help/error messages)
	var outputBuf bytes.Buffer
	rootCmd.SetOut(&outputBuf)
	rootCmd.SetErr(&outputBuf)

	// Parse command line into args and execute
	args := ParseArgs(cmdLine)
	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		errorMsg := outputBuf.String()
		if errorMsg == "" {
			errorMsg = err.Error()
		}
		return CommandResult{
			Action:   ActionError,
			Output:   errorMsg,
			ExitCode: 1,
		}
	}

	// Add any output from successful command execution
	if outputBuf.Len() > 0 {
		result.Output = outputBuf.String()
	}

	return result
}

// createCobraCommand creates the Cobra command structure
func createCobraCommand(result *CommandResult) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "shellbox",
		Short: "Shellbox development environment manager",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Remove default help command
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	// spinup command
	spinupCmd := &cobra.Command{
		Use:   ActionSpinup + " [box_name]",
		Short: "Create and start a development box",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result.Action = ActionSpinup
			result.Args = args
			result.ExitCode = 0
			return nil
		},
	}

	// connect command
	connectCmd := &cobra.Command{
		Use:   ActionConnect + " <box_name>",
		Short: "Connect to an existing development box",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result.Action = ActionConnect
			result.Args = args
			result.ExitCode = 0
			return nil
		},
	}

	// help command
	helpCmd := &cobra.Command{
		Use:   ActionHelp,
		Short: "Show help information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			result.Action = ActionHelp
			result.Args = args
			result.ExitCode = 0
			return nil
		},
	}

	// version command
	versionCmd := &cobra.Command{
		Use:   ActionVersion,
		Short: "Show version information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			result.Action = ActionVersion
			result.Args = args
			result.ExitCode = 0
			return nil
		},
	}

	// whoami command
	whoamiCmd := &cobra.Command{
		Use:   ActionWhoami,
		Short: "Show current user information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			result.Action = ActionWhoami
			result.Args = args
			result.ExitCode = 0
			return nil
		},
	}

	rootCmd.AddCommand(spinupCmd, connectCmd, helpCmd, versionCmd, whoamiCmd)

	return rootCmd
}

// parseArgs splits a command line into arguments
// Simple implementation - could be enhanced for quotes, escaping, etc.
func ParseArgs(cmdLine string) []string {
	args := strings.Fields(strings.TrimSpace(cmdLine))
	// Use slices.Clip for memory efficiency if args will be long-lived
	return slices.Clip(args)
}
