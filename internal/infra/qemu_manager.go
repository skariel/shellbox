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

// StartQEMUWithVolume starts QEMU VM with the attached volume using memory-mapped file persistence
func (qm *QEMUManager) StartQEMUWithVolume(ctx context.Context, instanceIP, _ string) error {
	// Wait for volume to be available and then start QEMU
	startCmd := `
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

# Check if state file exists and is valid
if [ ! -f ` + QEMUStatePath + ` ]; then
    echo "ERROR: State file missing at ` + QEMUStatePath + `"
    echo "This volume does not contain a valid golden snapshot with saved VM state"
    exit 1
fi

STATE_SIZE=$(stat -c%s ` + QEMUStatePath + `)
echo "State file found: ` + QEMUStatePath + ` (size: $STATE_SIZE bytes)"

if [ $STATE_SIZE -eq 0 ]; then
    echo "ERROR: State file exists but is empty"
    echo "The golden snapshot state file is corrupted"
    exit 1
fi

# Verify it looks like a valid QEMU state file (should start with QEMU save format markers)
echo "Verifying state file format..."
MAGIC=$(sudo hexdump -n 16 -e '16/1 "%02x"' ` + QEMUStatePath + `)
echo "State file magic bytes: $MAGIC"

# Start QEMU VM with memory-mapped file and load saved state
echo "Starting QEMU with saved state..."
sudo sh -c 'nohup qemu-system-x86_64 \
   -enable-kvm \
   -m 24G \
   -object memory-backend-file,id=mem,size=24G,mem-path=` + QEMUMemoryPath + `,share=on \
   -machine memory-backend=mem \
   -smp 8 \
   -cpu host,+kvmclock,+kvm-asyncpf \
   -rtc base=utc,driftfix=slew \
   -drive file=` + QEMUBaseDiskPath + `,format=qcow2 \
   -cdrom ` + QEMUCloudInitPath + ` \
   -device virtio-rng-pci,rng=rng0 -object rng-random,id=rng0,filename=/dev/urandom \
   -chardev socket,path=/tmp/qemu-ga.sock,server=on,wait=off,id=qga0 \
   -device virtio-serial \
   -device virtserialport,chardev=qga0,name=org.qemu.guest_agent.0 \
   -nographic \
   -serial file:/mnt/userdata/qemu-serial.log \
   -qmp unix:` + QEMUMonitorSocket + `,server,nowait \
   -nic user,model=virtio,hostfwd=tcp::2222-:22,dns=8.8.8.8 \
   -incoming defer > /mnt/userdata/qemu.log 2>&1 < /dev/null &'

# Brief sleep to ensure process starts
sleep 2

# Check if QEMU started and capture status
if pgrep -f qemu-system-x86_64 > /dev/null; then
    QEMU_PID=$(pgrep -f qemu-system-x86_64)
    echo "SUCCESS: QEMU started with PID: $QEMU_PID"
    
    # Initialize QMP and load the saved state
    echo "Initializing QMP and loading saved state..."
    (
    echo '{"execute":"qmp_capabilities"}'
    sleep 0.5
    echo '{"execute":"migrate-incoming", "arguments":{"uri":"file://` + QEMUStatePath + `"}}'
    ) | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + ` || true
    
    # Wait for migration to complete (max 10 seconds)
    for i in {1..20}; do
        STATUS=$((echo '{"execute":"qmp_capabilities"}'; echo '{"execute":"query-migrate"}') | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + ` 2>/dev/null | grep -o '"status":"[^"]*"' | cut -d'"' -f4)
        if [ "$STATUS" = "completed" ]; then
            echo "Migration completed successfully"
            break
        fi
        sleep 0.5
    done
    
    # Resume the VM immediately
    echo "Resuming VM execution..."
    (echo '{"execute":"qmp_capabilities"}'; sleep 0.1; echo '{"execute":"cont"}') | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + ` || true
    
    # Sync guest time if guest agent is available
    (echo '{"execute":"qmp_capabilities"}'; sleep 0.1; echo '{"execute":"guest-set-time"}') | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + ` 2>/dev/null || true
    
    echo "VM resumed and time synced"
    
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
	output, err := sshutil.ExecuteCommandWithOutput(ctx, startCmd, AdminUsername, instanceIP)
	if err != nil {
		slog.Error("Failed to start QEMU", "error", err, "output", output)
		return fmt.Errorf("failed to start QEMU: %w", err)
	}
	slog.Info("QEMU start command completed", "output", output)

	// Track timing for resume process
	resumeStartTime := time.Now()

	// Wait for QEMU SSH to be ready
	// Should be fast since VM is resumed immediately with time sync
	if err := qm.waitForQEMUSSH(ctx, instanceIP); err != nil {
		return fmt.Errorf("QEMU SSH not ready: %w", err)
	}

	resumeDuration := time.Since(resumeStartTime)
	slog.Info("QEMU started",
		"instanceIP", instanceIP,
		"resumeDuration", resumeDuration,
		"resumeSeconds", resumeDuration.Seconds())
	return nil
}

// StopQEMU stops the QEMU VM cleanly
func (qm *QEMUManager) StopQEMU(ctx context.Context, instanceIP string) error {
	stopCmd := `
# Quit QEMU cleanly using QMP
(echo '{"execute":"qmp_capabilities"}'; sleep 0.1; echo '{"execute":"quit"}') | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock || true

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
		// First check if guest agent is responsive (faster than SSH)
		guestPingCmd := fmt.Sprintf(`(echo '{"execute":"qmp_capabilities"}'; echo '{"execute":"guest-ping"}') | sudo socat - UNIX-CONNECT:%s 2>/dev/null | grep -q '"return":{}'`, QEMUMonitorSocket)
		if err := sshutil.ExecuteCommand(ctx, guestPingCmd, AdminUsername, instanceIP); err == nil {
			slog.Debug("Guest agent is responsive")
			// If guest agent works, sync time one more time
			syncTimeCmd := fmt.Sprintf(`(echo '{"execute":"qmp_capabilities"}'; echo '{"execute":"guest-set-time"}') | sudo socat - UNIX-CONNECT:%s 2>/dev/null || true`, QEMUMonitorSocket)
			_ = sshutil.ExecuteCommand(ctx, syncTimeCmd, AdminUsername, instanceIP)
		}

		// Test SSH connection directly to the QEMU VM from bastion
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "ConnectTimeout=3",
			"-o", "StrictHostKeyChecking=no",
			"-o", "ServerAliveInterval=2",
			"-i", sshutil.SSHKeyPath,
			"-p", fmt.Sprintf("%d", BoxSSHPort),
			fmt.Sprintf("%s@%s", SystemUserUbuntu, instanceIP),
			"echo 'QEMU SSH ready'")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("QEMU VM SSH not yet ready: %w: %s", err, string(output))
		}
		return nil
	}, 5*time.Minute, 5*time.Second, "QEMU SSH connectivity")
}
