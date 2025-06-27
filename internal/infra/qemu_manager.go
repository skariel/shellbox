package infra

import (
	"context"
	"fmt"
	"log/slog"
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

# Verify state file is a valid QEMU savevm format
if ! file ` + QEMUStatePath + ` | grep -q "QEMU"; then
    # Try checking with hexdump for QEMU savevm signature
    HEADER=$(sudo dd if=` + QEMUStatePath + ` bs=1 count=8 2>/dev/null | od -An -tx1 | tr -d ' 
')
    if [[ ! "$HEADER" =~ ^(514556|7f454c46) ]]; then
        echo "ERROR: State file does not appear to be a valid QEMU save file"
        echo "Header: $HEADER"
        exit 1
    fi
fi

# Start QEMU VM with memory-mapped file and load saved state
echo "Starting QEMU with saved state..."
sudo sh -c 'nohup qemu-system-x86_64 \
   -machine pc,accel=kvm,memory-backend=mem \
   -cpu host,+kvmclock,+kvm-asyncpf \
   -m 24G \
   -object memory-backend-file,id=mem,size=24G,mem-path=` + QEMUMemoryPath + `,share=on,prealloc=off \
   -smp 8 \
   -rtc base=utc,driftfix=slew \
   -drive file=` + QEMUBaseDiskPath + `,format=qcow2,if=virtio \
   -cdrom ` + QEMUCloudInitPath + ` \
   -device virtio-rng-pci,rng=rng0 \
   -object rng-random,id=rng0,filename=/dev/urandom \
   -device virtio-net-pci,netdev=net0 \
   -netdev user,id=net0,hostfwd=tcp::2222-:22,dns=8.8.8.8 \
   -nographic \
   -serial file:/mnt/userdata/qemu-serial.log \
   -qmp unix:` + QEMUMonitorSocket + `,server,nowait \
   -monitor none \
   -incoming defer > /mnt/userdata/qemu.log 2>&1 < /dev/null &'

# Brief sleep to ensure process starts
sleep 2

# Check if QEMU started and capture status
if pgrep -f qemu-system-x86_64 > /dev/null; then
    QEMU_PID=$(pgrep -f qemu-system-x86_64)
    echo "SUCCESS: QEMU started with PID: $QEMU_PID"
    
    # Wait for QMP socket to be created
    echo "Waiting for QMP socket..."
    SOCKET_WAIT=0
    while [ ! -S ` + QEMUMonitorSocket + ` ]; do
        if [ $SOCKET_WAIT -ge 10 ]; then
            echo "ERROR: QMP socket not created after 10 seconds"
            exit 1
        fi
        echo "Waiting for QMP socket to be created..."
        sleep 1
        SOCKET_WAIT=$((SOCKET_WAIT + 1))
    done
    echo "QMP socket is ready"
    
    # Initialize QMP and load the saved state
    echo "Initializing QMP and loading saved state..."
    (
    echo '{"execute":"qmp_capabilities"}'
    sleep 0.5
    # Set migration capabilities to match the source
    echo '{"execute":"migrate-set-capabilities", "arguments":{"capabilities":[{"capability": "xbzrle", "state": false}, {"capability": "x-ignore-shared", "state": true}, {"capability": "auto-converge", "state": false}, {"capability": "postcopy-ram", "state": false}]}}'
    sleep 0.5
    echo '{"execute":"migrate-incoming", "arguments":{"uri":"exec:cat ` + QEMUStatePath + `"}}'
    ) | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + `
    
    # Check if the migration command succeeded
    if [ ${PIPESTATUS[1]} -ne 0 ]; then
        echo "ERROR: Failed to execute migration command via QMP"
        exit 1
    fi
    
    # Migration is synchronous - when the command returns, it's complete
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

	// Migration is synchronous - it's already complete if the command succeeded
	slog.Info("Incoming migration completed (synchronous operation)")

	// Resume the VM after migration completes
	resumeCmd := `(echo '{"execute":"qmp_capabilities"}'; sleep 0.1; echo '{"execute":"cont"}') | sudo socat - UNIX-CONNECT:` + QEMUMonitorSocket + ` || true`
	if _, err := sshutil.ExecuteCommandWithOutput(ctx, resumeCmd, AdminUsername, instanceIP); err != nil {
		slog.Warn("Failed to resume VM", "error", err)
	}
	slog.Info("VM resumed after migration")

	// Use sendkey to refresh network configuration
	slog.Info("Using sendkey to refresh network configuration")

	// Give VM a moment to stabilize after resume
	time.Sleep(300 * time.Millisecond)

	// Switch to tty1 console (Ctrl+Alt+F1)
	if err := SendKeyCommand(ctx, []string{"ctrl", "alt", "f1"}, instanceIP); err != nil {
		slog.Warn("Failed to switch to console", "error", err)
	}
	time.Sleep(200 * time.Millisecond)

	// Press Enter to ensure we're at a prompt (auto-login should have logged us in)
	if err := SendKeyCommand(ctx, []string{"ret"}, instanceIP); err != nil {
		slog.Warn("Failed to send enter key", "error", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Use dhclient to release and renew DHCP lease (faster than service restart)
	// Using semicolon to avoid && character issues
	slog.Info("Renewing DHCP lease")
	if err := SendTextViaKeys(ctx, "sudo dhclient -r eth0; sudo dhclient eth0", instanceIP); err != nil {
		slog.Warn("Failed to type dhclient command", "error", err)
	}
	if err := SendKeyCommand(ctx, []string{"ret"}, instanceIP); err != nil {
		slog.Warn("Failed to execute dhclient", "error", err)
	}
	time.Sleep(1 * time.Second) // dhclient is usually quick

	slog.Info("Network refresh completed via sendkey")

	// Skip SSH connectivity test - the actual user connection will handle retries
	// This eliminates redundant SSH testing and saves ~4-5 seconds
	slog.Info("QEMU started, ready for user connection",
		"instanceIP", instanceIP)
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
