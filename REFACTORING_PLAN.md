# Shellbox Refactoring Plan

This document outlines the comprehensive refactoring plan for the Shellbox codebase. Each phase is designed to be completed independently, with progress tracked in the checkboxes.

## Overview

The refactoring aims to:
- Reduce code duplication by ~30-40%
- Improve package organization and separation of concerns
- Standardize naming conventions
- Introduce targeted use of Go generics
- Maintain simplicity and avoid over-engineering

## Phase 1: Quick Wins (Low Risk, High Impact)

### 1.1 Fix Naming Inconsistencies
- [ ] `getStorageConnectionString()` → `GetStorageConnectionString()` (internal/infra/tables.go:102)
- [ ] `getBastionRoleID()` → `formatBastionRoleID()` (internal/infra/bastion.go:269)
- [ ] `ExecuteCommand()` → `ExecuteCommandNoOutput()` (internal/sshutil/ssh.go:90)
- [ ] `New()` → `NewServer()` (internal/sshserver/server.go:31)
- [ ] `ParseArgs()` → `ParseSSHCommandArgs()` (internal/sshserver/commands.go:153)

### 1.2 Consolidate Small Files
- [ ] Merge `retry.go` into new `internal/infra/azure_helpers.go`
- [ ] Merge `logger.go` into new `internal/common/setup.go`

### 1.3 Extract Constants
- [ ] Add to `constants.go`:
  ```go
  ResourceGraphWaitTimeout = 2 * time.Minute
  ResourceGraphWaitInterval = 5 * time.Second
  DefaultPollInterval = 10 * time.Second
  ```
- [ ] Replace hardcoded values throughout codebase

## Phase 2: Function Consolidation

### 2.1 Consolidate Wait Functions
- [ ] Create generic `waitForResourceInGraph()` function
- [ ] Replace `waitForVolumeInResourceGraph()` (internal/infra/resource_graph_wait.go:11)
- [ ] Replace `waitForVolumeTagsInResourceGraph()` (internal/infra/resource_graph_wait.go:54)
- [ ] Replace `waitForInstanceTagsInResourceGraph()` (internal/infra/resource_graph_wait.go:102)
- [ ] Delete old wait functions

### 2.2 Merge Volume Update Functions
- [ ] Create `UpdateVolumeTags(ctx, clients, volumeID string, tags map[string]string)`
- [ ] Refactor `UpdateVolumeStatus()` to use new function
- [ ] Refactor `UpdateVolumeStatusUserAndBox()` to use new function
- [ ] Apply same pattern to instance updates

### 2.3 Consolidate Volume Creation
- [ ] Merge `CreateVolume()` logic into `CreateVolumeWithTags()`
- [ ] Create internal `createVolumeInternal()` for shared logic
- [ ] Update `CreateVolumeFromSnapshot()` to use shared function

### 2.4 Standardize Delete Operations
- [ ] Create generic `deleteAzureResource()` helper
- [ ] Standardize error handling across all delete functions
- [ ] Ensure consistent empty name validation

## Phase 3: Package Reorganization

### 3.1 Create New Package Structure
```
internal/
├── azure/          # Azure-specific operations
├── compute/        # VM and disk management  
├── pool/           # Pool management
├── qemu/           # QEMU operations
└── common/         # Shared utilities
```

### 3.2 Move Azure-Specific Code
- [ ] Create `internal/azure/` directory
- [ ] Move `AzureClients` struct from `network.go` to `azure/clients.go`
- [ ] Move network infrastructure code to `azure/network.go`
- [ ] Move table storage operations to `azure/tables.go`
- [ ] Move authentication/role code to `azure/auth.go`
- [ ] Move retry logic to `azure/retry.go`
- [ ] Move resource naming to `azure/naming.go`

### 3.3 Extract Compute Operations
- [ ] Create `internal/compute/` directory
- [ ] Split `instances.go`:
  - Core CRUD → `compute/instances.go`
  - Cleanup operations → `compute/cleanup.go`
  - Attachment operations → `compute/attachments.go`
- [ ] Move `volumes.go` → `compute/volumes.go`

### 3.4 Extract Pool Management
- [ ] Create `internal/pool/` directory
- [ ] Split `pool.go`:
  - Pool manager → `pool/manager.go`
  - Scaling operations → `pool/scaling.go`
  - Instance pool → `pool/instances.go`
  - Volume pool → `pool/volumes.go`

### 3.5 Extract QEMU Operations
- [ ] Create `internal/qemu/` directory
- [ ] Move `qemu_manager.go` → `qemu/manager.go`
- [ ] Move `qmp_helpers.go` → `qemu/qmp.go`
- [ ] Extract migration operations → `qemu/migration.go`

### 3.6 Update Import Paths
- [ ] Update all import statements
- [ ] Fix any circular dependencies
- [ ] Run tests to ensure nothing broke

## Phase 4: Introduce Targeted Generics

### 4.1 Generic Resource Update Pattern
- [ ] Create `UpdateResourceTags[T any]()` generic function
- [ ] Implement for volumes
- [ ] Implement for instances
- [ ] Remove old update functions

### 4.2 Generic Resource Graph Queries
- [ ] Create `ResourceTypeConfig` struct
- [ ] Implement generic `CountResourcesByStatus()`
- [ ] Implement generic `GetResourcesByStatus()`
- [ ] Update all callers

### 4.3 Generic Tag Builder
- [ ] Implement `TagBuilder` struct with fluent interface
- [ ] Replace manual tag building throughout codebase
- [ ] Add tag validation in builder

### 4.4 Generic Pool Scaling
- [ ] Create `ScaleConfig` struct
- [ ] Implement generic `scaleUpResources()`
- [ ] Implement generic `scaleDownResources()`
- [ ] Update pool manager to use generics

## Phase 5: Cleanup and Optimization

### 5.1 Remove Unnecessary Code
- [ ] Remove `CreateVolume()` wrapper function
- [ ] Identify and remove unused helper functions
- [ ] Remove any duplicate test utilities
- [ ] Clean up commented code

### 5.2 Final Consistency Pass
- [ ] Ensure all error messages follow same format
- [ ] Verify all logging uses structured format
- [ ] Check all functions have appropriate comments
- [ ] Update any outdated documentation

## Progress Tracking

| Phase | Status | Completion | Notes |
|-------|--------|------------|-------|
| Phase 1 | Not Started | 0% | |
| Phase 2 | Not Started | 0% | |
| Phase 3 | Not Started | 0% | |
| Phase 4 | Not Started | 0% | |
| Phase 5 | Not Started | 0% | |

## Risk Mitigation

1. **Before starting each phase**:
   - Ensure all tests pass
   - Create a git branch for the phase
   - Document any assumptions

2. **After completing each phase**:
   - Run `./tst.sh` to verify build, lint, and tests
   - Update this document with completion status
   - Get code review before merging

3. **If issues arise**:
   - Document the issue in this file
   - Consider reverting if necessary
   - Adjust plan based on learnings

## Expected Outcomes

- **Code Reduction**: ~30-40% fewer lines of code
- **Improved Maintainability**: Clear package boundaries
- **Better Testing**: Smaller, focused packages easier to test
- **Consistency**: Unified patterns across similar operations
- **Performance**: No degradation, possible improvements from reduced duplication

## Notes for Implementers

1. **Always consult CLAUDE.md** for project conventions
2. **Maintain backwards compatibility** during transitions
3. **Write tests** for new generic functions
4. **Update functions.md** after significant changes
5. **Keep changes minimal** - don't refactor beyond the plan

---

Last Updated: [Date]
Next Review: [After Phase 1 completion]