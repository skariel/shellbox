# Code Refactoring Tracking

## Overview
Simplification refactoring focused on code organization and deduplication without behavioral changes.

## Refactoring Tasks

### Task 1: Move Pool Constants to pool.go
**Status**: 🟢 Complete  
**Rationale**: Pool-specific constants should be near their usage in pool.go  
**Files**: 
- Source: `internal/infra/constants.go` (lines 112-134)
- Target: `internal/infra/pool.go`

**Details**:
- ✅ Moved all `Default*` and `Dev*` pool constants (22 lines)
- ✅ Kept as package-level constants in pool.go
- ✅ No behavior change - just better locality

**Before**: constants.go: 202 lines  
**After**: constants.go: 180 lines, pool.go: +22 lines  
**Verification**: ✅ `./tst.sh` passed

---

### Task 2: Consolidate String Cleaning in resource_naming.go  
**Status**: 🟢 Complete  
**Rationale**: Remove duplicate string cleaning logic  
**Files**: `internal/infra/resource_naming.go`

**Details**:
- ✅ Extracted common helper `cleanSuffixAlphanumeric(allowUppercase bool)`
- ✅ Updated StorageAccountName() and cleanSuffixForTable() to use helper
- ✅ Eliminated duplicate alphanumeric filtering loops
- ✅ Preserved exact behavior for both functions

**Before**: resource_naming.go: 133 lines  
**After**: resource_naming.go: ~120 lines  
**Verification**: ✅ `./tst.sh` passed

---

### Task 3: Move NSG Rules to network.go
**Status**: 🟡 Planned  
**Rationale**: NSG rules should be in network.go where they're used  
**Files**:
- Source: `internal/infra/constants.go` (createNSGRule function + BastionNSGRules)
- Target: `internal/infra/network.go`

**Details**:
- Move createNSGRule() helper function (lines 142-156)
- Move BastionNSGRules variable (lines 159-165)
- Move formatNSGRules() helper if only used by moved code

**Before**: constants.go: 180 lines (after Task 1)  
**After**: constants.go: ~155 lines, network.go: +25 lines  
**Verification**: `./tst.sh` must pass

---

### Task 4: Remove Unused Config Functions (Conditional)
**Status**: 🟡 Planned  
**Rationale**: Remove dead code if functions are unused  
**Files**: `internal/infra/constants.go`

**Details**:
- Investigate usage of FormatConfig() and GenerateConfigHash()
- If unused: remove functions (lines 167-202)
- If used: keep unchanged

**Before**: constants.go: ~155 lines (after Task 3)  
**After**: constants.go: ~125 lines (if functions removed)  
**Verification**: `./tst.sh` must pass

---

## Progress Tracking

| Task | Status | Lines Removed | Issues | Completed |
|------|--------|---------------|--------|-----------|
| 1. Move Pool Constants | 🟡 Planned | 0 → 22 moved | None | - |
| 2. String Cleaning | 🟡 Planned | ~10 | None | - |
| 3. Move NSG Rules | 🟡 Planned | ~25 moved | None | - |
| 4. Remove Unused Code | 🟡 Planned | ~30 (if unused) | None | - |

**Legend**: 🟡 Planned, 🟠 In Progress, 🟢 Complete, 🔴 Issues

## Verification Protocol
1. Run `./tst.sh` after each task
2. Ensure all tests pass
3. Verify no behavioral changes
4. Check imports and dependencies
5. Update progress table

## Rollback Plan
Each task is independent and can be rolled back individually if issues arise.

## Final Expected Results
- **constants.go**: 202 → ~125 lines (-77 lines)
- **Better code locality**: Constants near their usage
- **Reduced duplication**: Common string cleaning logic
- **Zero behavioral changes**: All existing functionality preserved
- **All tests passing**: Full compatibility maintained