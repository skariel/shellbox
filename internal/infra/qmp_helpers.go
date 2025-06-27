package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"shellbox/internal/sshutil"
	"strings"
)

// QMPResponse represents a response from QEMU Machine Protocol
type QMPResponse struct {
	Return json.RawMessage `json:"return,omitempty"`
	Error  *QMPError       `json:"error,omitempty"`
	Event  string          `json:"event,omitempty"`
	QMP    *QMPVersion     `json:"QMP,omitempty"`
}

// QMPError represents an error response from QMP
type QMPError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// QMPVersion represents the QMP version information
type QMPVersion struct {
	Version struct {
		QEMU    QEMUVersion `json:"qemu"`
		Package string      `json:"package"`
	} `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// QEMUVersion represents the QEMU version details
type QEMUVersion struct {
	Micro int `json:"micro"`
	Minor int `json:"minor"`
	Major int `json:"major"`
}

// MigrationStatus represents the status of a migration operation
type MigrationStatus struct {
	Status string `json:"status"`
}

// executeQMPCommands executes QMP commands via SSH and returns parsed responses
func executeQMPCommands(ctx context.Context, commands []string, instanceIP string) ([]QMPResponse, error) {
	// Build the command string
	cmdParts := []string{"echo '{\"execute\":\"qmp_capabilities\"}'"}
	for _, cmd := range commands {
		cmdParts = append(cmdParts, fmt.Sprintf("sleep 0.1; echo '%s'", cmd))
	}

	cmdStr := fmt.Sprintf("(%s) | sudo socat - UNIX-CONNECT:%s 2>&1",
		strings.Join(cmdParts, "; "), QEMUMonitorSocket)

	// Execute via SSH
	output, err := sshutil.ExecuteCommandWithOutput(ctx, cmdStr, AdminUsername, instanceIP)
	if err != nil {
		// The command might return non-zero exit code but still have valid output
		if output == "" {
			return nil, fmt.Errorf("QMP command failed: %w", err)
		}
	}

	// Parse the JSON responses
	responses, parseErr := parseQMPResponses(output)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse QMP response: %w, output: %s", parseErr, output)
	}

	return responses, nil
}

// parseQMPResponses parses multiple JSON responses from QMP output
func parseQMPResponses(output string) ([]QMPResponse, error) {
	// Split by newlines and parse each line as JSON
	lines := strings.Split(output, "\n")
	responses := make([]QMPResponse, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip non-JSON lines (like error messages)
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var resp QMPResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			// Skip lines that don't parse as JSON
			continue
		}

		responses = append(responses, resp)
	}

	if len(responses) == 0 {
		return nil, fmt.Errorf("no valid JSON responses found in output")
	}

	return responses, nil
}

// checkQMPSuccess checks if QMP commands executed successfully
func checkQMPSuccess(responses []QMPResponse) error {
	// Check for any error responses
	for _, resp := range responses {
		if resp.Error != nil {
			return fmt.Errorf("QMP error: %s - %s", resp.Error.Class, resp.Error.Desc)
		}
	}

	// Count successful returns (excluding QMP handshake)
	returnCount := 0
	for _, resp := range responses {
		if resp.Return != nil && resp.QMP == nil {
			returnCount++
		}
	}

	if returnCount == 0 {
		return fmt.Errorf("no successful return responses found")
	}

	return nil
}

// ExecuteMigrationCommand executes a migration command and checks for success
func ExecuteMigrationCommand(ctx context.Context, instanceIP, stateFile string) error {
	migrateCmd := fmt.Sprintf(`{"execute":"migrate", "arguments":{"uri":"exec:cat>%s"}}`, stateFile)

	responses, err := executeQMPCommands(ctx, []string{migrateCmd}, instanceIP)
	if err != nil {
		return fmt.Errorf("failed to execute migration command: %w", err)
	}

	// Check for successful execution
	if err := checkQMPSuccess(responses); err != nil {
		return fmt.Errorf("migration command failed: %w", err)
	}

	// Check for STOP event which indicates VM paused for migration
	hasStopEvent := false
	for _, resp := range responses {
		if resp.Event == "STOP" {
			hasStopEvent = true
			break
		}
	}

	if hasStopEvent {
		// This is expected - VM stops for migration
		return nil
	}

	// Even without STOP event, if we have successful returns, migration was accepted
	return nil
}

// CheckMigrationStatus queries the migration status
func CheckMigrationStatus(ctx context.Context, instanceIP string) (*MigrationStatus, error) {
	queryCmd := `{"execute":"query-migrate"}`

	responses, err := executeQMPCommands(ctx, []string{queryCmd}, instanceIP)
	if err != nil {
		return nil, fmt.Errorf("failed to query migration status: %w", err)
	}

	// Find the response with migration status
	for _, resp := range responses {
		if resp.Return != nil {
			var status MigrationStatus
			if err := json.Unmarshal(resp.Return, &status); err == nil && status.Status != "" {
				return &status, nil
			}
		}
	}

	return nil, fmt.Errorf("migration status not found in response")
}

// ExecuteGuestPing checks if the QEMU guest agent is responsive
func ExecuteGuestPing(ctx context.Context, instanceIP string) error {
	pingCmd := `{"execute":"guest-ping"}`

	responses, err := executeQMPCommands(ctx, []string{pingCmd}, instanceIP)
	if err != nil {
		return fmt.Errorf("failed to execute guest-ping: %w", err)
	}

	// Check for successful execution
	if err := checkQMPSuccess(responses); err != nil {
		return fmt.Errorf("guest-ping failed: %w", err)
	}

	return nil
}

// ExecuteGuestSetTime synchronizes the guest time
func ExecuteGuestSetTime(ctx context.Context, instanceIP string) error {
	setTimeCmd := `{"execute":"guest-set-time"}`

	// This command might fail if guest agent isn't ready, so we don't check for errors
	_, _ = executeQMPCommands(ctx, []string{setTimeCmd}, instanceIP)

	return nil
}
