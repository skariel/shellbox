package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"shellbox/internal/sshutil"
	"strings"
	"time"
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

	// Debug log the raw QMP output
	if strings.Contains(cmdStr, "migrate") && !strings.Contains(cmdStr, "query-migrate") {
		slog.Debug("Raw QMP migration command output", "output", output)
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

// MigrationInfo represents detailed migration information
type MigrationInfo struct {
	Status           string `json:"status"`
	TotalTime        int64  `json:"total-time,omitempty"`        // milliseconds
	ExpectedDowntime int64  `json:"expected-downtime,omitempty"` // milliseconds
	Downtime         int64  `json:"downtime,omitempty"`          // milliseconds
	SetupTime        int64  `json:"setup-time,omitempty"`        // milliseconds
	RAM              *struct {
		Transferred    int64   `json:"transferred"`  // bytes
		Remaining      int64   `json:"remaining"`    // bytes
		Total          int64   `json:"total"`        // bytes
		Duplicate      int64   `json:"duplicate"`    // pages
		Skipped        int64   `json:"skipped"`      // pages
		Normal         int64   `json:"normal"`       // pages
		NormalBytes    int64   `json:"normal-bytes"` // bytes
		DirtyPages     int64   `json:"dirty-pages-rate"`
		MBps           float64 `json:"mbps"` // MB/s
		DirtySyncCount int64   `json:"dirty-sync-count"`
	} `json:"ram,omitempty"`
}

// GetMigrationInfo queries detailed migration information
func GetMigrationInfo(ctx context.Context, instanceIP string) (*MigrationInfo, error) {
	queryCmd := `{"execute":"query-migrate"}`

	responses, err := executeQMPCommands(ctx, []string{queryCmd}, instanceIP)
	if err != nil {
		return nil, fmt.Errorf("failed to query migration info: %w", err)
	}

	// Find the response with migration info
	for _, resp := range responses {
		if resp.Return != nil {
			var info MigrationInfo
			if err := json.Unmarshal(resp.Return, &info); err == nil {
				// If status is empty, it means no migration is active (QEMU returns {})
				if info.Status == "" {
					info.Status = "none"
				}
				return &info, nil
			}
		}
	}

	return nil, fmt.Errorf("migration info not found in response")
}

// migrationProgress tracks migration progress state
type migrationProgress struct {
	lastTransferred    int64
	lastCheckTime      time.Time
	progressStallCount int
}

// calculateCheckInterval returns polling interval based on progress
func calculateCheckInterval(progress float64) time.Duration {
	switch {
	case progress > 90:
		return 50 * time.Millisecond
	case progress > 75:
		return 100 * time.Millisecond
	case progress > 50:
		return 200 * time.Millisecond
	default:
		return 500 * time.Millisecond
	}
}

// handleActiveMigration processes active migration status
func handleActiveMigration(info *MigrationInfo, progress *migrationProgress, startTime time.Time) time.Duration {
	if info.RAM == nil {
		return 100 * time.Millisecond
	}

	transferred := info.RAM.Transferred
	remaining := info.RAM.Remaining
	total := info.RAM.Total
	speed := info.RAM.MBps

	// Calculate progress percentage
	var progressPct float64
	if total > 0 {
		progressPct = float64(transferred) / float64(total) * 100
	}

	// Check if progress is stalled
	if progress.lastTransferred > 0 && transferred == progress.lastTransferred && time.Since(progress.lastCheckTime) > 5*time.Second {
		progress.progressStallCount++
		if progress.progressStallCount > 3 {
			slog.Warn("Migration progress stalled",
				"transferred", transferred,
				"remaining", remaining,
				"stallDuration", time.Since(progress.lastCheckTime))
		}
	} else if transferred != progress.lastTransferred {
		progress.progressStallCount = 0
	}

	// Log progress every second or when significant progress is made
	shouldLog := time.Since(progress.lastCheckTime) >= time.Second ||
		(progress.lastTransferred > 0 && transferred-progress.lastTransferred > 100*1024*1024) // 100MB progress

	if shouldLog {
		slog.Debug("Migration progress",
			"status", info.Status,
			"progress", fmt.Sprintf("%.1f%%", progressPct),
			"transferred", fmt.Sprintf("%.1f MB", float64(transferred)/1024/1024),
			"remaining", fmt.Sprintf("%.1f MB", float64(remaining)/1024/1024),
			"speed", fmt.Sprintf("%.1f MB/s", speed),
			"elapsed", time.Since(startTime).Round(time.Second))
		progress.lastCheckTime = time.Now()
	}

	progress.lastTransferred = transferred
	return calculateCheckInterval(progressPct)
}

// WaitForMigrationWithProgress waits for migration to complete with progress tracking
func WaitForMigrationWithProgress(ctx context.Context, instanceIP string, timeoutSeconds int) error {
	checkInterval := 100 * time.Millisecond
	startTime := time.Now()
	timeout := time.Duration(timeoutSeconds) * time.Second
	progress := &migrationProgress{}

	for {
		if time.Since(startTime) > timeout {
			return fmt.Errorf("migration timeout after %v", timeout)
		}

		info, err := GetMigrationInfo(ctx, instanceIP)
		if err != nil {
			// Don't fail immediately, migration might be in transition
			time.Sleep(checkInterval)
			continue
		}

		// Log initial status on first check
		if progress.lastCheckTime.IsZero() {
			slog.Info("Initial migration status check", "status", info.Status)
			progress.lastCheckTime = time.Now()
		}

		switch info.Status {
		case "completed":
			slog.Info("Migration completed successfully",
				"totalTime", time.Duration(info.TotalTime)*time.Millisecond,
				"downtime", time.Duration(info.Downtime)*time.Millisecond)
			return nil

		case "failed":
			return fmt.Errorf("migration failed")

		case "cancelled":
			return fmt.Errorf("migration cancelled")

		case "active":
			checkInterval = handleActiveMigration(info, progress, startTime)

		case "none", "setup":
			// Migration is initializing
			slog.Debug("Migration initializing", "status", info.Status)
			checkInterval = 500 * time.Millisecond

		default:
			// Handle empty status (migration not started) or unknown status
			if info.Status == "" {
				slog.Debug("Migration status is empty, migration may not have started yet")
			} else {
				slog.Debug("Unknown migration status", "status", info.Status)
			}
			// Use a reasonable check interval for initial state
			checkInterval = 500 * time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(checkInterval):
			// Continue checking
		}
	}
}

// ExecuteMigrationCommand executes a migration command and checks for success
func ExecuteMigrationCommand(ctx context.Context, instanceIP, stateFile string) error {
	// Note: Space after > is required for shell redirection to work properly
	migrateCmd := fmt.Sprintf(`{"execute":"migrate", "arguments":{"uri":"exec:cat > %s"}}`, stateFile)

	responses, err := executeQMPCommands(ctx, []string{migrateCmd}, instanceIP)
	if err != nil {
		return fmt.Errorf("failed to execute migration command: %w", err)
	}

	// Log the responses for debugging
	for i, resp := range responses {
		if resp.Return != nil {
			slog.Debug("Migration command response", "index", i, "return", string(resp.Return))
		}
		if resp.Error != nil {
			slog.Warn("Migration command error response", "index", i, "error", resp.Error)
		}
		if resp.Event != "" {
			slog.Debug("Migration command event", "index", i, "event", resp.Event)
		}
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

// Guest agent functions removed - using sendkey approach instead

// SendKeyCommand sends a key or key combination via QMP
func SendKeyCommand(ctx context.Context, keys []string, instanceIP string) error {
	// Build the keys array for QMP
	keyObjs := make([]string, 0, len(keys))
	for _, key := range keys {
		keyObjs = append(keyObjs, fmt.Sprintf(`{"type":"qcode","data":%q}`, key))
	}

	keyCmd := fmt.Sprintf(`{"execute":"send-key", "arguments":{"keys":[%s]}}`, strings.Join(keyObjs, ","))

	responses, err := executeQMPCommands(ctx, []string{keyCmd}, instanceIP)
	if err != nil {
		return fmt.Errorf("failed to send key command: %w", err)
	}

	return checkQMPSuccess(responses)
}

// SendTextViaKeys sends text by converting each character to sendkey commands
func SendTextViaKeys(ctx context.Context, text, instanceIP string) error {
	// Character to QEMU key mapping
	charToKey := map[rune][]string{
		'a': {"a"}, 'b': {"b"}, 'c': {"c"}, 'd': {"d"}, 'e': {"e"},
		'f': {"f"}, 'g': {"g"}, 'h': {"h"}, 'i': {"i"}, 'j': {"j"},
		'k': {"k"}, 'l': {"l"}, 'm': {"m"}, 'n': {"n"}, 'o': {"o"},
		'p': {"p"}, 'q': {"q"}, 'r': {"r"}, 's': {"s"}, 't': {"t"},
		'u': {"u"}, 'v': {"v"}, 'w': {"w"}, 'x': {"x"}, 'y': {"y"},
		'z': {"z"},
		'A': {"shift", "a"}, 'B': {"shift", "b"}, 'C': {"shift", "c"},
		'D': {"shift", "d"}, 'E': {"shift", "e"}, 'F': {"shift", "f"},
		'G': {"shift", "g"}, 'H': {"shift", "h"}, 'I': {"shift", "i"},
		'J': {"shift", "j"}, 'K': {"shift", "k"}, 'L': {"shift", "l"},
		'M': {"shift", "m"}, 'N': {"shift", "n"}, 'O': {"shift", "o"},
		'P': {"shift", "p"}, 'Q': {"shift", "q"}, 'R': {"shift", "r"},
		'S': {"shift", "s"}, 'T': {"shift", "t"}, 'U': {"shift", "u"},
		'V': {"shift", "v"}, 'W': {"shift", "w"}, 'X': {"shift", "x"},
		'Y': {"shift", "y"}, 'Z': {"shift", "z"},
		'0': {"0"}, '1': {"1"}, '2': {"2"}, '3': {"3"}, '4': {"4"},
		'5': {"5"}, '6': {"6"}, '7': {"7"}, '8': {"8"}, '9': {"9"},
		' ': {"spc"}, '-': {"minus"}, '.': {"dot"}, '/': {"slash"},
		':': {"shift", "semicolon"}, ';': {"semicolon"},
		'=': {"equal"}, '_': {"shift", "minus"},
		'\n': {"ret"}, '\t': {"tab"},
	}

	for _, char := range text {
		keys, ok := charToKey[char]
		if !ok {
			// Skip unsupported characters
			slog.Warn("Unsupported character in sendkey", "char", string(char))
			continue
		}

		if err := SendKeyCommand(ctx, keys, instanceIP); err != nil {
			return fmt.Errorf("failed to send key for char %c: %w", char, err)
		}

		// Small delay between keystrokes
		time.Sleep(50 * time.Millisecond)
	}

	return nil
}
