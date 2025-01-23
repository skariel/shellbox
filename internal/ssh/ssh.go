package ssh

import (
	"fmt"
	"os/exec"
	"time"
)

// CopyFile copies a file to a remote host using scp
func CopyFile(localPath, remotePath, username, hostname string) error {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	scpDest := fmt.Sprintf("%s@%s:%s", username, hostname, remotePath)
	var lastErr error

	for {
		select {
		case <-timeout:
			if lastErr != nil {
				return fmt.Errorf("timeout copying file: %w", lastErr)
			}
			return fmt.Errorf("timeout copying file")
		case <-ticker.C:
			cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=4", localPath, scpDest)
			if output, err := cmd.CombinedOutput(); err == nil {
				return nil
			} else {
				lastErr = fmt.Errorf("%w: %s", err, string(output))
			}
		}
	}
}

// ExecuteCommand executes a command on a remote host using SSH
func ExecuteCommand(command, username, hostname string) error {
	cmd := exec.Command("ssh",
		fmt.Sprintf("%s@%s", username, hostname),
		command)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}
	return nil
}
