package ssh

import (
	"context"
	"fmt"
	"os/exec"
	"shellbox/internal/infra"
)

// CopyFile copies a file to a remote host using scp
func CopyFile(localPath, remotePath, username, hostname string) error {
	scpDest := fmt.Sprintf("%s@%s:%s", username, hostname, remotePath)
	opts := infra.DefaultRetryOptions()
	opts.Operation = "scp file transfer"

	_, err := infra.RetryWithTimeout(context.Background(), opts, func(ctx context.Context) (bool, error) {
		cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=4", localPath, scpDest)
		if output, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("%w: %s", err, string(output))
		}
		return true, nil
	})
	return err
}

// ExecuteCommand executes a command on a remote host using SSH
func ExecuteCommand(command, username, hostname string) error {
	opts := infra.DefaultRetryOptions()
	opts.Operation = "ssh command execution"

	_, err := infra.RetryWithTimeout(context.Background(), opts, func(ctx context.Context) (bool, error) {
		cmd := exec.Command("ssh",
			"-o", "StrictHostKeyChecking=no",
			"-o", "ConnectTimeout=4",
			fmt.Sprintf("%s@%s", username, hostname),
			command)
		if output, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("%w: %s", err, string(output))
		}
		return true, nil
	})
	return err
}
