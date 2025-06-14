# QEMU Debugging Guide

## Check QEMU Status

```bash
# Check if QEMU process is running
pgrep -f qemu-system-x86_64

# See detailed process info
ps aux | grep qemu-system-x86_64

# Check QEMU logs (it runs in background, so check system logs)
sudo journalctl -u qemu -n 100
# or check dmesg
sudo dmesg | grep -i qemu
```

## Manual QEMU Start

If QEMU is not running, you can start it manually:

```bash
cd /mnt/userdata

# Start QEMU with saved state
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
   -loadvm ssh-ready
```

## Debug Connection

The QEMU VM should be accessible on port 2222:

```bash
# Test SSH connection to QEMU VM
ssh -p 2222 ubuntu@localhost

# Check if port is listening
sudo netstat -tlnp | grep 2222

# Check QEMU monitor socket
sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock
# Then type: info status
```

## Common Issues

1. **Memory file missing**: Check if `/mnt/userdata/qemu-memory/ubuntu-mem` exists
2. **Disk files missing**: Verify `/mnt/userdata/qemu-disks/ubuntu-base.qcow2` exists
3. **SSH not ready**: The VM might need more time to boot after resuming from saved state
4. **KVM not available**: Check if KVM is enabled with `kvm-ok`
5. **Permissions**: Ensure files are accessible (may need sudo)

## File Locations

- Memory file: `/mnt/userdata/qemu-memory/ubuntu-mem`
- Base disk: `/mnt/userdata/qemu-disks/ubuntu-base.qcow2`
- Cloud-init: `/mnt/userdata/qemu-disks/cloud-init.iso`
- Monitor socket: `/tmp/qemu-monitor.sock`
- SSH port: 2222

## Monitor Commands

When connected to QEMU monitor socket:
- `info status` - Check VM status
- `info network` - Check network configuration
- `savevm ssh-ready` - Save VM state
- `loadvm ssh-ready` - Load saved VM state
- `quit` - Gracefully shutdown QEMU