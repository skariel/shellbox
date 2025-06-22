package infra

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"shellbox/internal/sshutil"
	"time"
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

# Check if required QEMU files exist
echo "Checking QEMU files:"
ls -la ` + QEMUBaseDiskPath + ` || echo "Base disk missing"
ls -la ` + QEMUCloudInitPath + ` || echo "Cloud-init missing" 
ls -la ` + QEMUMemoryPath + ` || echo "Memory file missing"

# Resume QEMU VM from saved state
echo "Starting QEMU..."
sudo sh -c 'nohup qemu-system-x86_64 \
   -enable-kvm \
   -m 24G \
   -mem-path ` + QEMUMemoryPath + ` \
   -smp 8 \
   -cpu host,+invtsc \
   -drive file=` + QEMUBaseDiskPath + `,format=qcow2 \
   -cdrom ` + QEMUCloudInitPath + ` \
   -device virtio-rng-pci,rng=rng0 -object rng-random,id=rng0,filename=/dev/urandom \
   -nographic \
   -serial file:/mnt/userdata/qemu-serial.log \
   -monitor unix:` + QEMUMonitorSocket + `,server,nowait \
   -nic user,model=virtio,hostfwd=tcp::2222-:22,dns=8.8.8.8 \
   -S > /mnt/userdata/qemu.log 2>&1 < /dev/null &'

# Brief sleep to ensure process starts
sleep 2

# Check if QEMU started and capture status
if pgrep -f qemu-system-x86_64 > /dev/null; then
    QEMU_PID=$(pgrep -f qemu-system-x86_64)
    echo "SUCCESS: QEMU started with PID: $QEMU_PID"
    
    # Load the saved VM state
    echo "Loading saved VM state..."
    echo "loadvm ssh-ready" | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + `
    sleep 2
    
    # Resume the VM
    echo "Resuming VM..."
    echo "cont" | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + `
    
    # Check if log file was created
    if [ -f /mnt/userdata/qemu.log ]; then
        echo "Log file created at /mnt/userdata/qemu.log"
        echo "First few lines of QEMU output:"
        head -n 5 /mnt/userdata/qemu.log || true
    fi
else
    echo "ERROR: Failed to start QEMU"
    # Check if log file exists and show any errors
    if [ -f /mnt/userdata/qemu.log ]; then
        echo "QEMU log contents:"
        cat /mnt/userdata/qemu.log
    else
        echo "No QEMU log file found - process may have failed to start"
    fi
    exit 1
fi
`

	slog.Info("Starting QEMU with volume", "instanceIP", instanceIP)
	output, err := sshutil.ExecuteCommandWithOutput(ctx, resumeCmd, AdminUsername, instanceIP)
	if err != nil {
		slog.Error("Failed to start QEMU", "error", err, "output", output)
		return fmt.Errorf("failed to start QEMU: %w", err)
	}
	slog.Info("QEMU start command completed", "output", output)
	// Wait for QEMU SSH to be ready
	// AI: the ssh connectivity below takes forever. Why is that? the sanpshot was saved with fully completed cloud-init and ssh readiness. Then we resume the qemu vm, and I expected this to work fast. Especially since we'rer worikgin from a memory-mapped file as memory. No need to load all memory to memory
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
		// Test SSH connection directly to the QEMU VM from bastion
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "ConnectTimeout=5",
			"-o", "StrictHostKeyChecking=no",
			"-i", sshutil.SSHKeyPath,
			"-p", fmt.Sprintf("%d", BoxSSHPort),
			fmt.Sprintf("%s@%s", SystemUserUbuntu, instanceIP),
			"echo 'QEMU SSH ready'")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("QEMU VM SSH not yet ready: %w: %s", err, string(output))
		}
		return nil
	}, 5*time.Minute, 10*time.Second, "QEMU SSH connectivity")
}
