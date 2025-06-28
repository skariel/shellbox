# Function Usage Analysis Report

## Summary

Total functions analyzed: 188  
Functions with zero references: 13 (6.9%)

## Functions with Zero References

### From `internal/infra/tables.go`:
1. **UpdateResourceRegistry** (line 227)
   - Only the function definition and comment exist, no calls found

2. **CleanupTestTables** (line 234)
   - Only the function definition and comment exist, no calls found

### From `internal/infra/instances.go`:
3. **DeallocateBox** (line 348)
   - Only the function definition and comment exist, no calls found

4. **FindInstancesByStatus** (line 437)
   - Only the function definition and comment exist, no calls found

### From `internal/infra/qmp_helpers.go`:
5. **WaitForMigrationWithProgress** (line 256)
   - Function is defined but never called anywhere in the codebase

6. **CheckMigrationStatus** (line 368)
   - Function is defined but never called anywhere in the codebase

7. **SendTextViaKeys** (line 410)
   - Function is defined but never called anywhere in the codebase

### From `internal/infra/volumes.go`:
8. **FindVolumesByRole** (line 213)
   - No references found except its own declaration comment

### From `internal/infra/resource_graph_queries.go`:
9. **GetOldestFreeInstances** (line 183)
   - No references found except its own declaration comment

10. **GetAllInstances** (line 211)
    - No references found except its own declaration comment

11. **GetAllVolumes** (line 222)
    - No references found except its own declaration comment

### From `internal/infra/resource_naming.go`:
12. **ResourceGroup** (line 15)
    - Method `(r *ResourceNamer) ResourceGroup()` has no references

13. **GoldenSnapshotName** (line 74)
    - Method `(r *ResourceNamer) GoldenSnapshotName()` has no references

## Analysis

These unused functions appear to fall into several categories:

1. **Planned functionality not yet implemented**: Functions like `UpdateResourceRegistry`, `CleanupTestTables`, and migration-related functions may have been written for future features.

2. **Legacy code**: Some functions might have been used previously but are no longer needed after refactoring.

3. **Utility functions**: Functions like `SendTextViaKeys` and resource query functions might be utilities prepared for future use.

4. **Test utilities**: `CleanupTestTables` appears to be a test utility that was never needed.

## Recommendations

1. Consider removing these unused functions to reduce code maintenance burden
2. If any are intended for future use, consider adding TODO comments explaining their purpose
3. Some functions like `GetAllInstances` and `GetAllVolumes` might be useful for debugging or administrative purposes - confirm before removal
