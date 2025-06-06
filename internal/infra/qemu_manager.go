package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"shellbox/internal/sshutil"
)

// QEMUManager handles QEMU VM operations on instances
type QEMUManager struct {
	clients *AzureClients
}

// NewQEMUManager creates a new QEMU manager
func NewQEMUManager(clients *AzureClients) *QEMUManager {
	return &QEMUManager{
		clients: clients,
	}
}

// StartQEMUWithVolume starts QEMU VM with the attached volume
func (qm *QEMUManager) StartQEMUWithVolume(ctx context.Context, instanceIP, _ string) error {
	// Wait for volume to be available and then resume QEMU
	resumeCmd := `
# Wait for data disk to be available
while [ ! -e /dev/disk/azure/scsi1/lun0 ]; do
    echo "Waiting for data disk..."
    sleep 2
done

# Mount data disk if not already mounted
if ! mountpoint -q /mnt/userdata; then
    sudo mkdir -p /mnt/userdata
    sudo mount /dev/disk/azure/scsi1/lun0 /mnt/userdata
fi

# Change to working directory
cd /mnt/userdata

# Resume QEMU VM from saved state
sudo qemu-system-x86_64 \
   -enable-kvm \
   -m 24G \
   -mem-prealloc \
   -mem-path /mnt/userdata/qemu-memory/ubuntu-mem \
   -smp 8 \
   -cpu host \
   -drive file=/mnt/userdata/qemu-disks/ubuntu-base.qcow2,format=qcow2 \
   -drive file=/mnt/userdata/qemu-disks/cloud-init.iso,format=raw \
   -nographic \
   -monitor unix:/tmp/qemu-monitor.sock,server,nowait \
   -nic user,model=virtio,hostfwd=tcp::2222-:22,dns=8.8.8.8 \
   -loadvm ssh-ready &

# Wait for QEMU to be ready
echo "Waiting for QEMU to resume..."
sleep 10
`

	if err := sshutil.ExecuteCommand(ctx, resumeCmd, AdminUsername, instanceIP); err != nil {
		return fmt.Errorf("failed to start QEMU: %w", err)
	}

	// Wait for QEMU SSH to be ready
	if err := qm.waitForQEMUSSH(ctx, instanceIP); err != nil {
		return fmt.Errorf("QEMU SSH not ready: %w", err)
	}

	slog.Info("QEMU started", "instanceIP", instanceIP)
	return nil
}

// StopQEMU stops the QEMU VM and saves its state
func (qm *QEMUManager) StopQEMU(ctx context.Context, instanceIP string) error {
	stopCmd := `
# Save QEMU state and quit
echo -e "savevm ssh-ready\nquit" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock || true

# Fallback: kill QEMU if monitor command fails
sudo pkill qemu-system-x86_64 || true
`

	if err := sshutil.ExecuteCommand(ctx, stopCmd, AdminUsername, instanceIP); err != nil {
		slog.Warn("Error stopping QEMU (expected during shutdown)", "instanceIP", instanceIP, "error", err)
		// Don't return error - stopping QEMU often causes connection issues
	}

	slog.Info("QEMU stopped", "instanceIP", instanceIP)
	return nil
}

// waitForQEMUSSH waits for QEMU VM to be SSH-accessible
func (qm *QEMUManager) waitForQEMUSSH(ctx context.Context, instanceIP string) error {
	return RetryOperation(ctx, func(ctx context.Context) error {
		testCmd := fmt.Sprintf(`
timeout 5 ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no -p %d ubuntu@localhost 'echo QEMU SSH ready' || exit 1
`, BoxSSHPort)

		return sshutil.ExecuteCommand(ctx, testCmd, AdminUsername, instanceIP)
	}, 2*time.Minute, 10*time.Second, "QEMU SSH connectivity")
}
