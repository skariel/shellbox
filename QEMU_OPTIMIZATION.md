# QEMU Startup Optimization Guide

## Current Issue
- Connection time from bastion to QEMU VM takes over 1 minute
- SSH connectivity takes ~71 seconds after QEMU starts
- Multiple SSH connection timeouts occur before connectivity is established

## Root Cause Analysis
When loading from a snapshot with `-loadvm ssh-ready`, the network stack needs time to re-initialize. The guest OS might be in a state where it's not actively checking for network connectivity until prompted.

## Optimization Recommendations

### 1. Network Stack Re-initialization (Quick Fix)
When loading from a snapshot, the network stack needs to be "woken up". Add network recovery commands after QEMU starts:

```bash
# Add after QEMU starts
sleep 5  # Give QEMU time to fully load the snapshot

# Send network reset command via monitor
echo "system_reset" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock
```

### 2. Use virtio-net with vhost acceleration
Currently using basic virtio networking. Enable vhost-net for better performance:

```bash
# Current:
-nic user,model=virtio,hostfwd=tcp::2222-:22,dns=8.8.8.8

# Optimized:
-netdev user,id=net0,hostfwd=tcp::2222-:22 \
-device virtio-net-pci,netdev=net0,romfile=
```

### 3. Clear ARP Cache
Clear stale ARP entries that might be causing delays:

```bash
# Add before starting QEMU
sudo ip neigh flush all
```

### 4. Optimize SSH Connectivity Check
Instead of waiting 10 seconds between retries, use more aggressive timing:

```go
// Use nc instead of ssh for faster checks
cmd := exec.CommandContext(ctx, "nc", "-z", "-w", "1", instanceIP, "2222")
// Retry every 2 seconds instead of 10
```

### 5. Use Memory Backend with Shared Memory
For faster memory access:

```bash
-object memory-backend-file,id=mem,size=24G,mem-path=/mnt/userdata/qemu-memory/ubuntu-mem,share=on \
-numa node,memdev=mem
```

### 6. Alternative: Network Interface Reset in Guest
Send keystrokes to reset network interface inside the guest:

```bash
# Reset network in the guest via monitor
echo "sendkey ctrl-alt-f1" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock
sleep 1
# Send network restart command keystrokes
```

### 7. Consider microVM for Ultra-Fast Startup
For even faster startup, consider using QEMU microvm machine type:

```bash
-machine microvm,x-option-roms=off,rtc=on,acpi=on \
-no-acpi-tables
```

## Implementation Priority

1. **Immediate**: Network reset approach (system_reset via monitor)
2. **Short-term**: Optimize SSH connectivity check timing
3. **Medium-term**: Implement virtio-net with vhost
4. **Long-term**: Consider microVM or alternative snapshot approaches

## Expected Results
- Reduce connection time from 60+ seconds to under 10 seconds
- More reliable network connectivity after snapshot restoration
- Better overall VM performance