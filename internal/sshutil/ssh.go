package sshutil

import (
	"fmt"
	"os/exec"
)

// CopyFile copies a file to a remote host using scp
func CopyFile(localPath, remotePath, username, hostname string) error {
	scpDest := fmt.Sprintf("%s@%s:%s", username, hostname, remotePath)
	cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=4", localPath, scpDest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// ExecuteCommand executes a command on a remote host using SSH
func ExecuteCommand(command, username, hostname string) error {
	cmd := exec.Command("ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=4",
		fmt.Sprintf("%s@%s", username, hostname),
		command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
