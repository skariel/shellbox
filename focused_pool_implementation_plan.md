# Shellbox Pool Implementation Plan

## Document Overview

**Purpose**: Implement a dual-pool system (instances + volumes) for shellbox.dev to enable instant box startup and true state persistence.

**Complexity**: High - Requires Azure Resource Graph integration, disk management, and coordinated resource lifecycle management.

**Estimated Duration**: 10-12 hours

**Lines of Code**: ~400-500 (mostly new, minimal modifications to existing code)

---

## Project Status Tracking

### Overall Progress
- [ ] Phase 1: Foundation Setup
- [ ] Phase 2: Golden Snapshot Implementation  
- [ ] Phase 3: Resource Graph Integration
- [ ] Phase 4: Volume Pool Management
- [ ] Phase 5: Instance Pool Enhancement
- [ ] Phase 6: Connection Flow Integration
- [ ] Phase 7: Testing & Validation
- [ ] Phase 8: Monitoring & Observability

### Current Status
**Status**: Not Started  
**Current Phase**: N/A  
**Next Step**: Begin Phase 1 - Add constants and SDK clients  
**Blockers**: None

### How to Update This Document
1. Check off completed tasks using `[x]`
2. Update "Current Status" section after each coding session
3. Add notes in "Implementation Notes" under each phase
4. Document any deviations from plan in "Deviations" section

---

## Architecture Overview

### Design Philosophy
The pool maintains optimal resource availability without user awareness. It manages two independent pools:
- **Instance Pool**: Azure VMs ready to host user boxes
- **Volume Pool**: Persistent disks created from golden snapshot containing pre-configured QEMU environments

### Key Architectural Decisions

1. **Golden Snapshot Strategy**
   - All volumes created from standardized base image
   - Ensures consistency and fast provisioning
   - Created idempotently during deployment and server startup

2. **Resource Graph as Source of Truth**
   - Use Azure tags for state management
   - Query Resource Graph for real-time resource status
   - Eliminates need for complex state synchronization

3. **Separation of Concerns**
   - Pool only manages resource availability
   - User mapping handled separately (future enhancement)
   - Clean boundaries enable independent scaling

### System Flow
```
User SSH → Bastion → Allocate Free Instance → Attach Free Volume → Resume QEMU → User Session
                           ↓                         ↓
                    Update Instance Status    Update Volume Status
                    (free → connected)        (free → attached)
```

---

## Phase 1: Foundation Setup (30 minutes)

### Objective
Add required constants, SDK clients, and configuration structures.

### Tasks

#### 1.1 Update Constants
- [ ] Add resource role constants to `internal/infra/constants.go`
- [ ] Add resource status constants
- [ ] Add tag key constants
- [ ] Add Azure resource type constants
- [ ] Add query and disk constants

**Code to add**:
```go
// Resource roles
const (
    ResourceRoleInstance = "instance"
    ResourceRoleVolume   = "volume"
)

// Resource statuses
const (
    ResourceStatusFree      = "free"
    ResourceStatusConnected = "connected"
    ResourceStatusAttached  = "attached"
)

// Tag keys
const (
    TagKeyRole     = "shellbox:role"
    TagKeyStatus   = "shellbox:status"
    TagKeyCreated  = "shellbox:created"
    TagKeyLastUsed = "shellbox:lastused"
)

// Azure resource types for Resource Graph queries
const (
    AzureResourceTypeVM   = "microsoft.compute/virtualmachines"
    AzureResourceTypeDisk = "microsoft.compute/disks"
)

// Query and disk constants
const (
    MaxQueryResults     = 10
    DefaultVolumeSizeGB = 32
    GoldenSnapshotPrefix = "golden-snapshot"
)
```

#### 1.2 Add Missing SDK Clients
- [ ] Add DisksClient to AzureClients struct
- [ ] Add SnapshotsClient to AzureClients struct
- [ ] Add ResourceGraphClient to AzureClients struct
- [ ] Initialize new clients in NewAzureClients function

**Location**: `internal/infra/clients.go`

#### 1.3 Create Pool Configuration Structure
- [ ] Create PoolConfig struct with dual pool settings
- [ ] Define DefaultPoolConfig and DevPoolConfig
- [ ] Add configuration to server initialization

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Code compiles without errors
- [ ] New constants are accessible from other packages
- [ ] SDK clients initialize successfully

---

## Phase 2: Golden Snapshot Implementation (2 hours)

### Objective
Create reusable golden snapshot containing pre-configured QEMU environment.

### Tasks

#### 2.1 Create Golden Snapshot Module
- [ ] Create new file `internal/infra/golden_snapshot.go`
- [ ] Implement `CreateGoldenSnapshotIfNotExists` function
- [ ] Add GoldenSnapshotInfo struct
- [ ] Implement idempotent snapshot creation logic

#### 2.2 Implement Snapshot Creation Logic
- [ ] Check for existing snapshot by name
- [ ] Create temporary data volume for QEMU setup
- [ ] Create temporary VM with data volume attached
- [ ] Wait for SSH accessibility (indicates QEMU setup complete)
- [ ] Create snapshot from data volume
- [ ] Cleanup temporary resources

#### 2.3 Update Resource Naming
- [ ] Add `GoldenSnapshotName()` to ResourceNamer
- [ ] Add volume naming functions

#### 2.4 Modify Box Creation for Data Volumes
- [ ] Create `CreateBoxWithDataVolume` function variant
- [ ] Modify init script to setup QEMU on data volume
- [ ] Add data disk mounting logic to init script

#### 2.5 Integration Points
- [ ] Call from `cmd/deploy/main.go` during deployment
- [ ] Call from `cmd/server/server.go` at startup
- [ ] Pass golden snapshot info to pool manager

### Key Implementation Details

**Critical Discovery**: Current implementation stores QEMU in `~/qemu-disks/` on ephemeral OS disk. Must change to persistent data volume mounted at `/mnt/userdata`.

**Init Script Modifications**:
1. Wait for data disk at `/dev/disk/azure/scsi1/lun0`
2. Mount to `/mnt/userdata`
3. Create QEMU environment on mounted volume
4. Ensure all user data persists on data volume

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Golden snapshot creates successfully
- [ ] Idempotent creation works (second run finds existing)
- [ ] Temporary resources cleaned up properly
- [ ] Snapshot contains valid QEMU setup

---

## Phase 3: Resource Graph Integration (2 hours)

### Objective
Implement Azure Resource Graph queries for efficient resource discovery and state management.

### Tasks

#### 3.1 Create Resource Graph Query Module
- [ ] Create `internal/infra/resource_graph_queries.go`
- [ ] Implement ResourceGraphQueries struct
- [ ] Add authentication and client initialization

#### 3.2 Implement Core Query Functions
- [ ] `CountResourcesByStatus` - Get resource counts by status
- [ ] `FindOldestFreeResources` - For scale-down decisions
- [ ] `GetResourcesByRole` - List all resources of specific role
- [ ] Add ResourceInfo struct for query results

#### 3.3 Query Templates
- [ ] Instance count query by status
- [ ] Volume count query by status
- [ ] Oldest free resources query with LastUsed sorting
- [ ] Add proper error handling for query failures

### Key Queries

**Count Resources**:
```kusto
Resources
| where type =~ 'microsoft.compute/virtualmachines'
| where tags['shellbox:role'] =~ 'instance'
| summarize count() by tostring(tags['shellbox:status'])
```

**Find Oldest Free Resources**:
```kusto
Resources
| where type =~ 'microsoft.compute/virtualmachines'
| where tags['shellbox:role'] =~ 'instance'
| where tags['shellbox:status'] =~ 'free'
| project name, id, lastused=todatetime(tags['shellbox:lastused'])
| order by lastused asc
| take 10
```

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Queries return expected results
- [ ] Tag filtering works correctly
- [ ] Performance acceptable (<1s for queries)
- [ ] Error handling for malformed queries

---

## Phase 4: Volume Pool Management (2 hours)

### Objective
Implement volume creation, lifecycle management, and status tracking.

### Tasks

#### 4.1 Volume Creation from Golden Snapshot
- [ ] Implement `CreateVolumeFromGoldenSnapshot` function
- [ ] Add proper tagging during creation
- [ ] Use armcompute.DisksClient for disk operations
- [ ] Generate unique volume IDs

#### 4.2 Volume Lifecycle Management
- [ ] Implement volume deletion for scale-down
- [ ] Add volume status update functions
- [ ] Implement attach/detach coordination

#### 4.3 Volume Pool Logic
- [ ] Query current volume counts by status
- [ ] Scale up when free volumes < minimum
- [ ] Scale down when free volumes > maximum
- [ ] Respect cooldown periods

### Critical Implementation Notes

**From Validation**: No existing disk management code. Must implement from scratch:
1. Disk creation using CreationData with snapshot source
2. Tag management for state tracking
3. Attachment state coordination with instances

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Volumes create successfully from snapshot
- [ ] Tags apply correctly during creation
- [ ] Scale up/down logic works as expected
- [ ] No orphaned volumes after operations

---

## Phase 5: Instance Pool Enhancement (2 hours)

### Objective
Refactor existing pool to support dual-pool architecture and proper tagging.

### Tasks

#### 5.1 Refactor BoxPool to ResourcePool
- [ ] Rename and restructure pool management
- [ ] Add dual pool support (instances + volumes)
- [ ] Implement shared cooldown tracking
- [ ] Update configuration handling

#### 5.2 Update Instance Creation
- [ ] Add proper tags during VM creation
- [ ] Set initial status to "free"
- [ ] Track creation and last-used timestamps

#### 5.3 Instance Status Management
- [ ] Implement status update functions using PATCH
- [ ] Update tags when instances get connections
- [ ] Handle connection lifecycle events

#### 5.4 Scale Down Implementation
- [ ] Query oldest free instances
- [ ] Implement parallel deletion
- [ ] Update tags during deletion to prevent races

### Key Refactoring Points

**Current State**: Pool creates boxes but never allocates them. Hardcoded SSH target `10.1.0.4:2222`.

**Required Changes**:
1. Add allocation mechanism
2. Dynamic IP resolution
3. Status tracking via tags
4. Connection counting

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Instance creation includes all required tags
- [ ] Status updates work without full VM update
- [ ] Scale down removes oldest instances first
- [ ] Pool maintains target free instance count

---

## Phase 6: Connection Flow Integration (3 hours)

### Objective
Integrate pool management with SSH connection flow for dynamic resource allocation.

### Tasks

#### 6.1 Fix Hardcoded IP Issue
- [ ] Remove hardcoded `10.1.0.4:2222` from SSH server
- [ ] Implement dynamic instance selection
- [ ] Add IP resolution from allocated instance

#### 6.2 Resource Allocation Flow
- [ ] Select free instance from pool
- [ ] Select free volume from pool
- [ ] Attach volume to instance
- [ ] Update both resource statuses
- [ ] Return connection details

#### 6.3 Resource Deallocation Flow
- [ ] Detect connection closure
- [ ] Suspend QEMU to volume
- [ ] Detach volume from instance
- [ ] Update statuses back to free
- [ ] Return resources to pool

#### 6.4 Connection Tracking
- [ ] Track active connections per instance
- [ ] Update LastUsed timestamps
- [ ] Handle multiple connections per instance
- [ ] Implement connection counting

### Critical Integration Points

**SSH Server Changes** (`internal/sshserver/server.go`):
1. Call allocation function instead of hardcoded IP
2. Handle allocation failures gracefully
3. Track connection lifecycle
4. Trigger deallocation on disconnect

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Dynamic allocation works correctly
- [ ] Resources return to pool after disconnect
- [ ] Multiple connections handled properly
- [ ] No resource leaks

---

## Phase 7: Testing & Validation (2 hours)

### Objective
Comprehensive testing of pool functionality and edge cases.

### Test Scenarios

#### 7.1 Pool Scaling Tests
- [ ] Test scale up when resources depleted
- [ ] Test scale down after cooldown period
- [ ] Test respecting max limits
- [ ] Test concurrent scaling operations

#### 7.2 Connection Lifecycle Tests
- [ ] Test successful allocation flow
- [ ] Test allocation when pool empty
- [ ] Test proper deallocation
- [ ] Test crash recovery

#### 7.3 Golden Snapshot Tests
- [ ] Test initial creation
- [ ] Test idempotent behavior
- [ ] Test volume creation from snapshot
- [ ] Test QEMU functionality on volumes

#### 7.4 Performance Tests
- [ ] Measure allocation latency
- [ ] Test Resource Graph query performance
- [ ] Measure pool maintenance overhead
- [ ] Test under high connection load

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] All tests pass consistently
- [ ] No resource leaks detected
- [ ] Performance meets requirements
- [ ] Edge cases handled gracefully

---

## Phase 8: Monitoring & Observability (1 hour)

### Objective
Add comprehensive monitoring and debugging capabilities.

### Tasks

#### 8.1 Pool Metrics
- [ ] Implement PoolMetrics struct
- [ ] Track resource counts by status
- [ ] Record scaling events
- [ ] Monitor allocation latency

#### 8.2 Logging Enhancements
- [ ] Add detailed pool operation logs
- [ ] Log all state transitions
- [ ] Include timing information
- [ ] Add correlation IDs

#### 8.3 Debug Endpoints
- [ ] Add `/pool/status` endpoint
- [ ] Show current pool state
- [ ] Display recent operations
- [ ] Include configuration details

#### 8.4 Table Storage Events
- [ ] Log pool scaling events
- [ ] Record allocation/deallocation
- [ ] Track resource lifecycle
- [ ] Enable historical analysis

### Implementation Notes
<!-- Add notes during implementation -->

### Validation Checklist
- [ ] Metrics accurately reflect pool state
- [ ] Logs provide sufficient debugging info
- [ ] Debug endpoints work correctly
- [ ] Historical data queryable

---

## Configuration Reference

### Development Configuration
```go
var DevPoolConfig = PoolConfig{
    MinFreeInstances:  1,
    MaxFreeInstances:  2,
    MaxTotalInstances: 5,
    MinFreeVolumes:    2,
    MaxFreeVolumes:    5,
    MaxTotalVolumes:   20,
    CheckInterval:     30 * time.Second,
    ScaleDownCooldown: 2 * time.Minute,
}
```

### Production Configuration
```go
var ProdPoolConfig = PoolConfig{
    MinFreeInstances:  5,
    MaxFreeInstances:  10,
    MaxTotalInstances: 100,
    MinFreeVolumes:    20,
    MaxFreeVolumes:    50,
    MaxTotalVolumes:   500,
    CheckInterval:     1 * time.Minute,
    ScaleDownCooldown: 10 * time.Minute,
}
```

---

## Risk Mitigation

### Identified Risks

1. **Resource Leak Risk**
   - Mitigation: Implement periodic reconciliation
   - Fallback: Manual cleanup scripts

2. **Tag Synchronization Issues**  
   - Mitigation: Use Resource Graph as single source of truth
   - Fallback: Periodic full scan and correction

3. **Scaling Thundering Herd**
   - Mitigation: Implement jitter and rate limiting
   - Fallback: Manual intervention capabilities

4. **Golden Snapshot Corruption**
   - Mitigation: Validation before use
   - Fallback: Snapshot versioning

---

## Deviations from Original Plan

_Document any changes made during implementation_

1. 
2. 
3. 

---

## Post-Implementation Tasks

- [ ] Update documentation
- [ ] Create runbooks for operations
- [ ] Set up monitoring alerts
- [ ] Plan user mapping integration
- [ ] Performance optimization pass

---

## Notes for Future Enhancement

### User Mapping Integration
When user/billing component is added:
1. Query Resource Graph for free resources
2. Update tags with user information
3. Track user→resource mappings separately
4. Handle billing based on connection time

### Multi-Region Support
Future considerations:
1. Regional pool management
2. Cross-region failover
3. Geo-distributed snapshots
4. Regional capacity planning

---

## Implementation Log

### Session 1: [Date]
- Completed: 
- Issues: 
- Next: 

### Session 2: [Date]
- Completed:
- Issues:
- Next:

<!-- Continue adding sessions as needed -->