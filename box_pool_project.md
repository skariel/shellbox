# Box Pool Project: Separated Instance and Volume Pools

## Project Goal

Transform the current shellbox architecture from a simple box pool with ephemeral storage to a sophisticated dual-pool system where:
- **Instance Pool**: Manages Azure VMs ready to host QEMU boxes
- **Volume Pool**: Manages persistent Azure managed disks containing user box states
- **Dynamic Attachment**: Instances and volumes are attached/detached on-demand during user connections

This enables true state persistence where user boxes survive instance recycling, and provides the foundation for advanced features like box sharing, snapshots, and multi-region deployment.

## Why This Architecture

### Current Limitations
- Box state is stored in VM local storage (ephemeral)
- Box state is lost when VM is deallocated
- No ability to migrate boxes between instances
- Scaling requires recreating user environments

### Target Benefits
- **True Persistence**: User boxes survive infrastructure changes
- **Instant Suspend/Resume**: QEMU state preserved in volumes with memory snapshots
- **Efficient Resource Usage**: Instances can be shared across users over time
- **Scalability**: Independent scaling of compute vs storage
- **Cost Optimization**: Instances can be deallocated without losing user data
- **Advanced Features**: Foundation for box sharing, snapshots, migration

### Technical Approach
- **Golden Snapshot**: Standard Ubuntu 22.04 LTS (using existing VM constants) with QEMU setup, created once, duplicated for new users
- **Azure Tables State Tracking**: Authoritative state for instance/volume assignments
- **Idempotent Operations**: Safe for concurrent deployment and bastion startup
- **Minimal Disruption**: Extends existing architecture rather than replacing it

## Current Project Status: **PLANNING PHASE - REFINED**

**Last Updated**: 2025-01-31  
**Phase**: Code Analysis Complete, Implementation Plan Refined  
**Next Milestone**: Begin Phase 1 - Foundation Infrastructure

## Key Insights from Code Analysis

After analyzing the actual codebase, several critical issues and opportunities were identified:

1. **Current Limitations**:
   - SSH server hardcoded to connect to `10.1.0.4:2222` - no dynamic box allocation
   - QEMU runs with local qcow2 files inside Azure VMs - no persistence across VM restarts
   - No user identification or box-to-user mapping
   - Pool maintains ready VMs but doesn't allocate them to users
   - No mechanism to track which user is connected to which box

2. **Reusable Components**:
   - Excellent pool management pattern in `pool.go`
   - Robust VM creation logic in `box.go` with proper networking
   - Well-structured Azure client management
   - Good logging and error handling patterns
   - Table Storage integration ready for extension

3. **Minimal Changes Strategy**:
   - Add disk/snapshot clients to existing `AzureClients` (~10 lines)
   - Fix hardcoded IP by adding allocation tracking to `BoxPool` (~50 lines)
   - Create volumes from golden snapshot instead of local qcow2 (~100 lines)
   - Update init script to use attached volumes (~20 lines modified)
   - Total: ~300-400 lines of code changes

## Refined Implementation Plan

### Phase 1: Foundation Infrastructure ⏸️ *Not Started* (30 minutes)

#### 1.1 Azure SDK Integration ⏸️ *Not Started*
- [ ] **Add Disk Clients to AzureClients struct** 
  ```go
  // Add to AzureClients struct:
  DisksClient     *armcompute.DisksClient
  SnapshotsClient *armcompute.SnapshotsClient
  ```
- [ ] **Initialize Disk Clients in createAzureClients()**
  ```go
  clients.DisksClient, err = armcompute.NewDisksClient(...)
  clients.SnapshotsClient, err = armcompute.NewSnapshotsClient(...)
  ```
- [ ] **Add Table Storage Constants**
  ```go
  tableUserBoxMapping = "UserBoxMapping"
  tableVolumeRegistry = "VolumeRegistry"
  ```
- [ ] **Add Resource Naming Functions**
  ```go
  func (r *ResourceNamer) GoldenSnapshotName() string
  func (r *ResourceNamer) UserVolumeName(userID string) string
  ```

### Phase 2: Fix Dynamic Box Allocation ⏸️ *Not Started* (2 hours)
**Critical**: This fixes the hardcoded IP issue and enables user-specific box allocation

#### 2.1 Enhance BoxPool Structure ⏸️ *Not Started*
- [ ] **Update BoxPool to Track Allocations**
  ```go
  type BoxInfo struct {
      BoxID      string
      Status     string // ready, allocated
      PrivateIP  string
      VolumeID   string
  }
  ```
- [ ] **Add User-to-Box Mapping**
  - Add `allocations map[string]string` to BoxPool
  - Implement `AllocateBox(userID string) (*BoxInfo, error)`
  - Track box private IPs when created
- [ ] **Implement Box Status Updates**
  - Update ResourceRegistry when boxes are allocated/deallocated
  - Add proper cleanup on disconnect

#### 2.2 Fix SSH Server Hardcoded IP ⏸️ *Not Started*
- [ ] **Extract User ID from SSH Key**
  - Use SSH public key fingerprint as user identifier
  - Add `getUserID(publicKey ssh.PublicKey) string` helper
- [ ] **Update HandleProxy for Dynamic Allocation**
  - Replace hardcoded `10.1.0.4` with allocation logic
  - Call `pool.AllocateBox(userID)` to get box info
  - Use returned `box.PrivateIP` for connection
- [ ] **Add Pool Reference to SSH Server**
  - Pass BoxPool to SSH server on creation
  - Update server struct to include pool reference

### Phase 3: Golden Snapshot Creation ⏸️ *Not Started* (1 hour)

#### 3.1 Golden Snapshot Implementation ⏸️ *Not Started*
- [ ] **Create golden_snapshot.go**
  - Reuse existing `CreateBox()` for temporary VM
  - Wait for cloud-init completion
  - Create snapshot from OS disk
  - Clean up temporary resources
- [ ] **Make Snapshot Creation Idempotent**
  - Check if snapshot already exists before creating
  - Register in Table Storage for tracking
- [ ] **Integration Points**
  - Call from `cmd/deploy/main.go` during infrastructure setup
  - Verify existence in `cmd/server/server.go` at startup

### Phase 4: Volume Management ⏸️ *Not Started* (1.5 hours)

#### 4.1 Volume Operations ⏸️ *Not Started*
- [ ] **Create volume_operations.go**
  - `CreateVolumeFromSnapshot()` - create user volumes from golden snapshot
  - `AttachVolumeToVM()` - attach volume as data disk
  - `DetachVolumeFromVM()` - detach on disconnect
- [ ] **Volume Pool Management**
  - Track volumes in Table Storage
  - Maintain pool of ready volumes (like box pool)
  - Assign volumes to users on first connection

#### 4.2 Update Box Initialization ⏸️ *Not Started*
- [ ] **Modify GenerateBoxInitScript()**
  - Wait for data disk at `/dev/disk/azure/scsi1/lun0`
  - Mount to `/mnt/userdata`
  - Use mounted volume for QEMU storage instead of local disk
  - Ensure QEMU state persists across VM restarts
- [ ] **Update Box Creation**
  - Remove local disk setup from init script
  - Ensure boxes start without data disk (attached later)

### Phase 5: Integration and Testing ⏸️ *Not Started* (1 hour)

#### 5.1 End-to-End Integration ⏸️ *Not Started*
- [ ] **Update Server Startup**
  - Pass BoxPool to SSH server for allocation
  - Ensure golden snapshot exists on startup
  - Start volume pool maintenance goroutine
- [ ] **Connection Flow Testing**
  - Test user connects and gets allocated box
  - Verify volume attachment and QEMU startup
  - Test disconnect and cleanup
- [ ] **Error Handling**
  - Handle no boxes available scenario
  - Handle volume attachment failures
  - Add proper user feedback for errors

#### 5.2 Testing Checklist ⏸️ *Not Started*
- [ ] **Unit Tests**
  - Test user ID extraction from SSH key
  - Test allocation/deallocation logic
  - Test volume operations
- [ ] **Integration Tests**
  - End-to-end user connection flow
  - Multiple concurrent users
  - VM restart and volume persistence
- [ ] **Performance Validation**
  - Measure connection time with pre-attached volumes
  - Test attachment latency for new volumes
  - Validate no regression in existing functionality

## Implementation Notes

### Critical Path Items
1. **Fix Hardcoded IP** (Phase 2) - This blocks all user-specific functionality
2. **Golden Snapshot** (Phase 3) - Required for volume creation
3. **Volume Attachment** (Phase 4) - Core persistence functionality

### Code Organization
- **New Files** (~300 lines total):
  - `internal/infra/golden_snapshot.go` - Snapshot creation using existing VM logic
  - `internal/infra/volume_operations.go` - Disk attach/detach operations
  - `internal/infra/box_allocation.go` - User-to-box allocation logic
  
- **Modified Files** (~100 lines changed):
  - `internal/infra/clients.go` - Add disk/snapshot clients (10 lines)
  - `internal/infra/constants.go` - Add table constants (3 lines)
  - `internal/infra/resource_naming.go` - Add naming functions (15 lines)
  - `internal/infra/pool.go` - Add allocation tracking (50 lines)
  - `internal/infra/box.go` - Update init script for volumes (20 lines)
  - `internal/sshserver/server.go` - Dynamic IP allocation (20 lines)
  - `cmd/server/server.go` - Pass pool to SSH server (5 lines)

### Key Design Decisions

1. **User Identification**: Use SSH public key fingerprint as user ID
   - Simple, secure, no additional auth needed
   - Fingerprint = first 16 chars of SHA256 hash

2. **Allocation Strategy**: In-memory tracking with Table Storage backup
   - Fast allocation decisions
   - Table Storage for persistence across restarts

3. **Volume Attachment**: Pre-attach to some instances for speed
   - Instant connections for users with pre-attached volumes
   - Lazy attachment for overflow

4. **Backward Compatibility**: Existing boxes continue working
   - Only new connections use volume system
   - Gradual migration path

### Implementation Timeline
- **Phase 1**: 30 minutes - Foundation setup
- **Phase 2**: 2 hours - Fix hardcoded IP and allocation
- **Phase 3**: 1 hour - Golden snapshot
- **Phase 4**: 1.5 hours - Volume operations
- **Phase 5**: 1 hour - Integration and testing
- **Total**: ~6 hours for MVP functionality

## How to Update This File

### Status Updates
- Update "Current Project Status" section with current phase and date
- Move completed tasks from active sections to "Completed Tasks" summary
- Update task status indicators: ⏸️ *Not Started*, 🔄 *In Progress*, ✅ *Completed*

### Task Management
- Mark completed tasks with ✅ and brief completion notes
- Add new subtasks as they are discovered during implementation
- Update time estimates and dependencies as they become clearer

### Historical Tracking
When tasks are completed, move them to a "Completed Tasks" section with format:
```markdown
### Completed Tasks

#### Phase X: Description ✅ *Completed YYYY-MM-DD*
- ✅ **Task Name** - Brief description of what was accomplished
```

### Progress Tracking
Add new sections as needed:
- **Current Blockers** - Issues preventing progress
- **Technical Decisions** - Key architectural choices made
- **Performance Metrics** - Measurements and benchmarks
- **Lessons Learned** - Insights gained during implementation

Keep this file as the single source of truth for project status and decisions.
