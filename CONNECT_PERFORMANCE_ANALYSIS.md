# Connect Command Performance Analysis

## Overview
The `connect` command in Shellbox is experiencing significant delays before QEMU even starts. This document analyzes the current flow and proposes optimizations.

## Current Flow Breakdown

When a user runs `ssh shellbox.dev connect dev33`, the following operations occur:

1. **Resource Queries** (fast, ~1-2s total)
   - Query Resource Graph for existing volume by userID and boxName
   - Query Resource Graph for free running instances

2. **UpdateInstanceStatusAndUser** (~1-2 minutes)
   - Get VM from Azure
   - Update VM tags (status, lastUsed, userID)
   - `BeginCreateOrUpdate` VM operation: **30-60 seconds**
   - Wait for Resource Graph sync: **up to 2 minutes**

3. **UpdateVolumeStatusUserAndBox** (~1-2 minutes)
   - Get Disk from Azure
   - Update disk tags (status, lastUsed, userID, boxName)
   - `BeginCreateOrUpdate` disk operation: **30-60 seconds**
   - Wait for Resource Graph sync: **up to 2 minutes**

4. **AttachVolumeToInstance** (~30-60 seconds)
   - Get VM from Azure
   - Add data disk to VM configuration
   - `BeginCreateOrUpdate` VM operation: **30-60 seconds**
   - No Resource Graph wait

5. **Start QEMU** (was slow, now fixed)
   - Get instance IP
   - Start QEMU with volume
   - Wait for SSH readiness

**Total time before QEMU starts: 3-6 minutes**

## Identified Bottlenecks

### 1. Multiple Sequential Azure Operations
- Two separate VM update operations (status update + disk attachment)
- Each Azure update operation takes 30-60 seconds
- Operations are done sequentially, not in parallel

### 2. Resource Graph Synchronization
- After each tag update, we wait up to 2 minutes for Resource Graph to reflect changes
- This adds 4 minutes of waiting in the worst case
- May not be necessary for the connect flow to work correctly

### 3. Tag-based State Management
- Using Azure resource tags for state management requires expensive update operations
- Resource Graph queries have eventual consistency delays

## Proposed Optimizations

### Priority 1: Combine VM Operations
**Impact: High | Effort: Low**

Instead of two separate VM updates:
```go
// Current: Two operations
UpdateInstanceStatusAndUser() // VM update #1
AttachVolumeToInstance()      // VM update #2

// Proposed: Single operation
UpdateInstanceAndAttachVolume() // Combined VM update
```

This would save 30-60 seconds per connect operation.

### Priority 2: Remove/Reduce Resource Graph Waits
**Impact: High | Effort: Low**

Options:
1. Remove the waits entirely if they're not critical for correctness
2. Reduce timeout from 2 minutes to 30 seconds
3. Make them async - don't block the connect flow
4. Only wait for critical tags (e.g., skip waiting for lastUsed timestamp)

This could save 2-4 minutes per connect operation.

### Priority 3: Parallelize Independent Operations
**Impact: Medium | Effort: Medium**

Update instance and volume tags in parallel:
```go
var g errgroup.Group
g.Go(func() error { return UpdateInstanceStatusAndUser(...) })
g.Go(func() error { return UpdateVolumeStatusUserAndBox(...) })
err := g.Wait()
```

This would save 30-60 seconds.

### Priority 4: Pre-attach Volumes to Instances
**Impact: High | Effort: High**

Keep a pool of instances with volumes already attached:
- Eliminates the `AttachVolumeToInstance` operation entirely
- Requires rethinking the resource allocation strategy
- Would save 30-60 seconds per connect

### Priority 5: Use Table Storage for State
**Impact: High | Effort: High**

Instead of Azure tags + Resource Graph:
- Use Table Storage for fast state updates (milliseconds vs minutes)
- Keep Azure tags for backup/debugging only
- Update tags asynchronously in the background

Benefits:
- Near-instant state updates
- No need to wait for Resource Graph sync
- More flexible state management

## Implementation Suggestions

### Quick Wins (implement first)
1. Remove or reduce Resource Graph wait timeouts
2. Combine the two VM update operations

### Medium-term Improvements
1. Parallelize independent operations
2. Investigate if all tag updates are necessary during connect

### Long-term Architecture Changes
1. Move to Table Storage for primary state management
2. Redesign resource pools with pre-attached volumes
3. Consider keeping QEMU running with suspended VMs instead of stopped

## Expected Performance After Optimizations

With quick wins only:
- Current: 3-6 minutes
- Optimized: 1-2 minutes

With all optimizations:
- Target: < 30 seconds

## Additional Notes

- The QEMU startup issue with `-mem-prealloc` has already been fixed
- Network operations within Azure VNet are fast
- The SSH timeout issue was a symptom of slow QEMU startup, now resolved