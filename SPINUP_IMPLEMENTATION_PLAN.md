# Spinup Command Implementation Plan

## Overview

Implement the `spinup` command to allow users to create named volumes (boxes) that persist their data. The architecture uses:

- **Volumes**: Persist user data, belong to specific users with names
- **VMs**: Ephemeral compute from shared pool, always return to "free" state
- **Flow**: Connect → find free VM + user's volume → attach → use → disconnect → detach → VM becomes free

## Architecture Understanding

When a user runs `ssh shellbox.dev spinup mybox1`, we:
1. Take a **free volume** from the pool
2. Tag it with the user's SSH key hash and box name
3. Volume becomes **owned by that user**
4. VMs remain in the shared pool, only temporarily allocated during sessions

When user connects for interactive session:
1. Find free VM from pool
2. Find user's volume by name
3. Attach volume to VM
4. User works on their persistent data
5. On disconnect: detach volume, VM returns to "free" pool

## Implementation Plan

### 1. Add User Volume Tags

**New constants in `internal/infra/constants.go`:**
```go
TagKeyUserID = "shellbox:userid"     // SHA256 hash of SSH public key  
TagKeyBoxName = "shellbox:boxname"   // User-provided box name
```

**Extended `VolumeTags` struct in `internal/infra/volumes.go`:**
```go
type VolumeTags struct {
    Role      string // volume, temp, golden
    Status    string // free, attached  
    CreatedAt string
    LastUsed  string
    VolumeID  string
    UserID    string // NEW: User identification
    BoxName   string // NEW: User-provided name
}
```

### 2. Box Name Validation

**New validation function in `internal/infra/volumes.go`:**
```go
func ValidateBoxName(name string) error {
    if len(name) < 3 || len(name) > 32 {
        return fmt.Errorf("box name must be 3-32 characters")
    }
    if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`).MatchString(name) {
        return fmt.Errorf("box name must contain only lowercase letters, numbers, and hyphens")
    }
    return nil
}
```

### 3. User Volume Management

**New methods in `internal/infra/volumes.go`:**

```go
// AllocateUserVolume converts a free volume to user ownership
func AllocateUserVolume(ctx context.Context, clients *AzureClients, userID, boxName string) (*VolumeInfo, error) {
    // 1. Validate box name
    if err := ValidateBoxName(boxName); err != nil {
        return nil, err
    }
    
    // 2. Check if user already has this box name
    if exists, err := UserVolumeExists(ctx, clients, userID, boxName); err != nil {
        return nil, err
    } else if exists {
        return nil, fmt.Errorf("box '%s' already exists", boxName)
    }
    
    // 3. Find free volume
    freeVolumes, err := FindVolumesByStatus(ctx, clients, ResourceStatusFree)
    if len(freeVolumes) == 0 {
        return nil, fmt.Errorf("no free volumes available")
    }
    
    volume := freeVolumes[0]
    
    // 4. Update volume tags with user info
    if err := UpdateVolumeWithUserInfo(ctx, clients, volume.ResourceID, userID, boxName); err != nil {
        return nil, err
    }
    
    // 5. Wait for tag visibility in Resource Graph
    if err := waitForVolumeInResourceGraph(ctx, clients, volume.ResourceID, userID, boxName); err != nil {
        return nil, err
    }
    
    return volume, nil
}

// FindUserVolumes returns all volumes owned by a user
func FindUserVolumes(ctx context.Context, clients *AzureClients, userID string) ([]VolumeInfo, error)

// UserVolumeExists checks if user already has a box with given name  
func UserVolumeExists(ctx context.Context, clients *AzureClients, userID, boxName string) (bool, error)

// FindUserVolumeByName finds specific user volume by box name
func FindUserVolumeByName(ctx context.Context, clients *AzureClients, userID, boxName string) (*VolumeInfo, error)
```

### 4. Resource Graph Query Extensions

**New method in `internal/infra/resource_graph_queries.go`:**
```go
// GetVolumesByUser returns all volumes owned by a specific user
func (rq *ResourceGraphQueries) GetVolumesByUser(ctx context.Context, userID string) ([]ResourceInfo, error) {
    query := fmt.Sprintf(`
        Resources
        | where type == "%s"
        | where resourceGroup == "%s" 
        | where tags["%s"] == "%s"
        | project id, name, location, tags
    `, AzureResourceTypeDisk, rq.resourceGroup, TagKeyUserID, userID)
    
    return rq.executeResourceQuery(ctx, query)
}
```

### 5. Modified Resource Allocator

**New method in `internal/infra/resource_allocator.go`:**
```go
// AllocateResourcesForUserBox finds free VM and user's specific volume
func (ra *ResourceAllocator) AllocateResourcesForUserBox(ctx context.Context, userID, boxName string) (*AllocatedResources, error) {
    // 1. Find user's volume by name
    volume, err := FindUserVolumeByName(ctx, ra.clients, userID, boxName)
    if err != nil {
        return nil, fmt.Errorf("volume not found: %w", err)
    }
    
    // 2. Check volume is free (not attached to another VM)
    if volume.Status != ResourceStatusFree {
        return nil, fmt.Errorf("volume '%s' is currently in use", boxName)
    }
    
    // 3. Find free VM (existing logic)
    freeInstances, err := ra.resourceQueries.GetInstancesByStatus(ctx, ResourceStatusFree)
    if len(freeInstances) == 0 {
        return nil, fmt.Errorf("no free VMs available")
    }
    
    instance := freeInstances[0]
    
    // 4. Allocate VM and attach volume (existing logic)
    if err := ra.performAllocation(ctx, instance, *volume); err != nil {
        return nil, err
    }
    
    // 5. Get IP and start QEMU (existing logic)
    instanceIP, err := ra.finalizeAllocation(ctx, instance, *volume)
    if err != nil {
        ra.rollbackAllocation(ctx, instance.ResourceID, volume.ResourceID)
        return nil, err
    }
    
    return &AllocatedResources{
        InstanceID: instance.ResourceID,
        VolumeID:   volume.ResourceID,
        InstanceIP: instanceIP,
    }, nil
}
```

### 6. Wait for Volume Visibility

**New function in `internal/infra/volumes.go`:**
```go
func waitForVolumeInResourceGraph(ctx context.Context, clients *AzureClients, volumeID, userID, boxName string) error {
    rq := NewResourceGraphQueries(clients.ResourceGraphClient, clients.SubscriptionID, clients.ResourceGroupName)
    
    verifyOperation := func(ctx context.Context) error {
        volumes, err := rq.GetVolumesByUser(ctx, userID)
        if err != nil {
            return fmt.Errorf("querying user volumes: %w", err)
        }
        
        for _, volume := range volumes {
            if volume.ResourceID == volumeID && volume.Tags[TagKeyBoxName] == boxName {
                return nil // Found it!
            }
        }
        
        return fmt.Errorf("volume %s not yet visible with user tags", volumeID)
    }
    
    return RetryOperation(ctx, verifyOperation, 2*time.Minute, 5*time.Second, "wait for volume visibility")
}
```

### 7. Implement Spinup Command

**Replace TODO in `internal/sshserver/server.go`:**
```go
func (s *Server) handleSpinupCommand(ctx CommandContext, result CommandResult, sess gssh.Session) {
    if len(result.Args) == 0 {
        sess.Write([]byte("Error: box name required\n"))
        sess.Exit(1)
        return
    }
    
    boxName := result.Args[0]
    userID := generateUserHash(ctx.UserID) // Use hash of SSH key
    
    s.logger.Info("Spinup command received", "user", userID[:16], "box", boxName)
    
    // Validate box name
    if err := ValidateBoxName(boxName); err != nil {
        sess.Write([]byte(fmt.Sprintf("Error: %v\n", err)))
        sess.Exit(1)
        return
    }
    
    // Allocate volume for user
    _, err := infra.AllocateUserVolume(context.Background(), s.clients, userID, boxName)
    if err != nil {
        sess.Write([]byte(fmt.Sprintf("Error: %v\n", err)))
        sess.Exit(1)
        return
    }
    
    successMsg := fmt.Sprintf("Box '%s' created successfully!\n\nTo connect: ssh shellbox.dev\n", boxName)
    sess.Write([]byte(successMsg))
    sess.Exit(0)
}
```

### 8. Modify Interactive Session Handling
# AI: the user will connect interactively to a specific box, using a specific name. There are different laternaitves for this: could be embedded in the user name. Or maybe as a subdomain, though not sure it is supported by ssh. Can you give suggestions here?
**Changes to `handleShellSession` in `internal/sshserver/server.go`:**

```go
// Before allocating resources, check if user has volumes
userID := generateUserHash(userKeyHash)
userVolumes, err := infra.FindUserVolumes(context.Background(), s.clients, userID)

if len(userVolumes) == 0 {
    // No boxes, show message and exit
    sess.Write([]byte("No boxes found. Create one with: ssh shellbox.dev spinup mybox\n"))
    return
}

if len(userVolumes) == 1 {
    // One box, use it
    resources, err := s.allocator.AllocateResourcesForUserBox(ctx, userID, userVolumes[0].BoxName)
} else {
    // Multiple boxes, show selection (future enhancement)
    sess.Write([]byte("Multiple boxes found. Box selection coming soon!\n"))
    return
}
```

## Files to Modify

1. **`internal/infra/constants.go`** - Add user/box tag constants
2. **`internal/infra/volumes.go`** - Extend VolumeTags, add user volume functions
3. **`internal/infra/resource_graph_queries.go`** - Add user volume queries  
4. **`internal/infra/resource_allocator.go`** - Add user box allocation method
5. **`internal/sshserver/server.go`** - Implement spinup command, modify session handling

## Key Features

- **Volume Ownership**: Volumes tagged with user identity and box name
- **Name Validation**: Simple rules for box names (3-32 chars, alphanumeric + hyphens)
- **Uniqueness**: Box names unique per user
- **Visibility Waiting**: Uses existing retry pattern to ensure tag changes are visible
- **VM Ephemeral**: VMs always return to free state after disconnect
- **Multi-box Support**: Framework ready for multiple user volumes

## Current Status

**Status**: Planning phase complete, ready for implementation

**Next Steps**:
1. Add new tag constants to `constants.go`
2. Extend `VolumeTags` struct and implement user volume management functions
3. Add Resource Graph queries for user volumes
4. Implement the spinup command handler
5. Modify session handling to use user volumes
6. Test the complete flow

**Dependencies**: 
- Existing retry mechanism (`RetryOperation`)
- Existing Resource Graph integration
- Current volume and VM management infrastructure

**Testing Plan**:
1. Unit tests for box name validation
2. Integration tests for volume allocation flow
3. End-to-end tests for spinup command
4. Test multiple users with same box names (should be isolated)
5. Test tag propagation timing
