# Refactoring Tasks

This document tracks code quality improvements identified during code review. Each task can be checked off when completed.

## Code Duplication

- [x] **Pool scaling logic duplication** ✅ REVIEWED - Decided to keep as-is
  - File: `internal/infra/pool.go`
  - Issue: `scaleUpInstances` (lines 138-185) and `scaleUpVolumes` (lines 249-310) share identical structure
  - Issue: `scaleDownInstances` (lines 187-247) and `scaleDownVolumes` (lines 312-369) are nearly identical
  - **DECISION**: Keep the current implementation without abstraction
  - **REASON 1**: Attempted abstraction added 29 lines instead of reducing code (YAGNI principle)
  - **REASON 2**: The business logic differences between instances and volumes are meaningful and distinct
  - **REASON 3**: Current code is clear, explicit, and straightforward - abstraction adds unnecessary complexity
  - **LESSON**: Not all code duplication needs to be eliminated if it makes the code harder to understand

- [x] **Hardcoded polling options** ✅ FIXED
  - Multiple files use `runtime.PollUntilDoneOptions{Frequency: 2 * time.Second}` instead of `DefaultPollOptions`
  - Solution: Use the existing `DefaultPollOptions` constant consistently
  - **FIXED**: Replaced 7 hardcoded instances with `&DefaultPollOptions`
  - **FILES**: `instances.go` (4 occurrences), `volumes.go` (3 occurrences)

- [ ] **Resource creation/deletion patterns**
  - Instance and volume creation follow similar patterns but aren't abstracted
  - Solution: Create a generic resource manager interface

- [ ] **Tag management duplication**
  - Tag creation and management logic is duplicated across files
  - Solution: Create a unified tag management system

## Naming Inconsistencies

- [x] **Inconsistent function visibility** ✅ FIXED
  - File: `internal/infra/instances.go`
  - Issue: `deleteVM` (line 448) is private but `DeleteNIC` (line 490) and `DeleteNSG` (line 517) are public
  - Solution: Make all internal deletion helpers private
  - **FIXED**: Renamed `deleteVM` → `DeleteVM` and `deleteDisk` → `DeleteDisk` for consistency
  - **NOTE**: Created TODO.md to write tests for these newly public functions

- [x] **Exported helper functions** ✅ REVIEWED - Keep as-is
  - File: `internal/infra/instances.go`
  - Issue: `ExtractInstanceIDFromVMName` (line 421) is exported but appears to be a helper
  - **DECISION**: Keep function public
  - **REASON**: Function is tested in `internal/test/unit/parsing_test.go`
  - **REASON**: Function is used internally within instances.go as well
  - **REASON**: Parsing/extraction functions are legitimately useful as public utilities

- [x] **Exported volume helper** ✅ REVIEWED - Keep as-is
  - File: `internal/infra/volumes.go`
  - Issue: `VolumeTagsToMap` (line 229) is exported but appears to be a helper
  - **DECISION**: Keep function public
  - **REASON**: Function is tested in `internal/test/unit/volumes_helpers_test.go` and `parsing_test.go`
  - **REASON**: Function is used internally within volumes.go at lines 90 and 140
  - **REASON**: Tag conversion functions are legitimately useful as public utilities

- [x] **Resource naming visibility** ✅ REVIEWED - Visibility is correct
  - File: `internal/infra/resource_naming.go`
  - Issue: Methods `SharedStorageAccountName` and `cleanSuffixForTable` have inconsistent visibility
  - **DECISION**: Keep current visibility as-is
  - **REASON**: `SharedStorageAccountName` is public and used in `internal/infra/network.go`
  - **REASON**: `cleanSuffixForTable` is private and only used internally (lines 113, 119)
  - **REASON**: The visibility difference is intentional and follows proper encapsulation

## Stale/Outdated Comments

- [x] **Incorrect comment for exported function** ✅ FIXED
  - File: `internal/infra/instances.go` (line 425)
  - Issue: Comment suggests function is internal but it's exported
  - Solution: Update comment to match exported status
  - **FIXED**: Updated comment from `extractInstanceIDFromVMName` to `ExtractInstanceIDFromVMName`

- [x] **Outdated deletion order comment** ✅ REVIEWED - Comment is correct
  - File: `internal/infra/instances.go` (line 358)
  - Issue: Comment doesn't match actual deletion order in code
  - **DECISION**: Keep comment as-is
  - **REASON**: Comment "VM, data disk, OS disk, NIC, NSG" matches actual code order

- [x] **Incorrect comment for exported function** ✅ FIXED
  - File: `internal/infra/volumes.go` (line 229)
  - Issue: Comment uses lowercase function name for exported function
  - Solution: Update comment to match function name
  - **FIXED**: Updated comment from `volumeTagsToMap` to `VolumeTagsToMap`

## Dead/Incomplete Code

- [x] **TODO with simulation code** ✅ MOVED TO TODO.md
  - File: `internal/sshserver/server.go` (lines 327-329)
  - Issue: TODO comment with simulation code for box creation
  - Solution: Implement actual box creation logic or remove if not needed
  - **ACTION**: Added task to TODO.md for future implementation

- [x] **Unused legacy constants** ✅ CONSOLIDATED
  - File: `internal/infra/constants.go` (lines 39-42)
  - Issue: Legacy table constants marked "for backward compatibility" appear unused
  - Solution: Remove if truly unused after verification
  - **FIXED**: Removed unused legacy functions: `CreateTableStorageResourcesLegacy`, `WriteEventLogLegacy`, `WriteResourceRegistryLegacy`
  - **FIXED**: Consolidated duplicate constants - removed `tableEventLogBase` and `tableResourceRegistryBase`
  - **FIXED**: Updated resource naming to use single set of table name constants
  - **IMPROVED**: Updated `createTables()` to return error if no table names provided
  - **RESULT**: Cleaner code with no duplication and better error handling

- [x] **Redundant logger variable** ✅ FIXED
  - File: `cmd/deploy/main.go` (line 12)
  - Issue: `logger` variable created but `infra.SetDefaultLogger()` already provides logging
  - Solution: Remove redundant variable
  - **FIXED**: Removed `logger := infra.NewLogger()` variable declaration
  - **FIXED**: Replaced all `logger.Info()` and `logger.Error()` calls with `slog.Info()` and `slog.Error()`
  - **FIXED**: Added `log/slog` import
  - **RESULT**: Cleaner code using the default logger set by `SetDefaultLogger()`

## Consistency Issues

- [x] **Error wrapping patterns** ✅ IMPROVED
  - Some functions use `fmt.Errorf` with `%w` for wrapping, others don't
  - Solution: Standardize on always using `%w` for error wrapping
  - **FIXED**: Added error context to bare `return err` statements in clients.go
  - **FIXED**: Improved table entity addition error with table name context
  - **FIXED**: Enhanced Resource Graph query errors with resource group context
  - **RESULT**: Better error messages with proper context for debugging

- [x] **Logging level consistency** ✅ REVIEWED - Already consistent
  - Inconsistent use of `slog.Warn` vs `slog.Error` for similar scenarios
  - Solution: Create guidelines for when to use each level
  - **ANALYSIS**: Comprehensive review shows logging levels are actually well-designed and consistent
  - **PATTERNS FOUND**: 
    - `Error`: Critical Azure operation failures, missing infrastructure
    - `Warn`: Expected failures, cleanup issues, optional operations
    - `Info`: Successful operations, state changes
    - `Debug`: Internal monitoring, queries
  - **CONCLUSION**: No changes needed - current logging strategy is appropriate

- [x] **Structured logging keys** ✅ FIXED
  - Issue: Mix of snake_case and camelCase in logging keys (e.g., "instance_id", "session_id", "time_remaining")
  - Solution: Standardize on camelCase for all logging keys while keeping Azure tag names as snake_case
  - **FIXED**: Changed all snake_case logging keys to camelCase:
    - `"instance_id"` → `"instanceID"`
    - `"volume_id"` → `"volumeID"`
    - `"user_id"` → `"userID"`
    - `"instance_ip"` → `"instanceIP"`
    - `"session_id"` → `"sessionID"`
    - `"time_remaining"` → `"timeRemaining"`
    - `"bastion_ip"` → `"bastionIP"`
  - **FILES**: `qemu_manager.go`, `resource_allocator.go`, `pool.go`, `server.go`, `cmd/deploy/main.go`
  - **RESULT**: Consistent camelCase logging keys across the entire codebase

- [ ] **Naming convention mix**
  - Mix of camelCase and snake_case in tag values and JSON fields
  - Inconsistent abbreviations (e.g., "VM" vs "vm", "ID" vs "Id")
  - Solution: Establish and apply consistent naming conventions

- [ ] **Tag key constants duplication**
  - Mix of generic (`TagKeyRole`) and specific (`GoldenTagKeyRole`) constants
  - Solution: Consolidate into a single set of constants

## Opportunities for Code Reuse

- [ ] **Abstract resource deletion pattern**
  - The deletion logic in `DeleteInstance` could be abstracted
  - Solution: Create a generic resource deletion framework

- [ ] **Unified event logging**
  - Event logging code is repeated throughout
  - Solution: Create centralized event logging methods

- [ ] **SSH command execution patterns**
  - Similar SSH execution patterns could be consolidated
  - Solution: Create SSH command execution utilities

- [ ] **Resource status updates**
  - `UpdateInstanceStatus` and `UpdateVolumeStatus` share identical logic
  - Solution: Create generic status update method

## Additional Improvements

- [ ] **Context usage**
  - Some functions accept context but don't use it properly
  - Inconsistent timeout configurations
  - Solution: Review and fix context usage patterns

- [ ] **Function comment typos**
  - File: `internal/infra/instances.go` (line 356)
  - Issue: "extractInstanceIDFromVMName" in comment should match actual function name
  - Solution: Fix typo in comment

## Priority Order

1. **High Priority**: Code duplication (especially pool scaling logic)
2. **Medium Priority**: Naming inconsistencies and dead code
3. **Low Priority**: Comment updates and minor consistency issues

## Notes

- Each task should be completed as a separate, focused change
- Run `./tst.sh` after each change to ensure code quality
- Update this file by checking off completed tasks