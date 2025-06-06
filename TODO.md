# TODO

## Testing Tasks

- [x] **Write tests for internal deletion helper functions** ✅ PARTIALLY COMPLETED
  - File: `internal/infra/instances.go`
  - Functions to test:
    - `DeleteVM` - Tests for VM deletion with and without the VM existing (TODO: needs network setup)
    - `DeleteDisk` - Tests for disk deletion (both OS and data disks) ✅ COMPLETED
  - **COMPLETED**: Created `internal/test/integration/deletion_functions_test.go`
  - **COMPLETED**: `DeleteDisk` function test passes successfully
  - **REMAINING**: `DeleteVM`, `DeleteNIC`, `DeleteNSG` tests need network infrastructure setup

- [ ] **TODO with simulation code**
  - File: `internal/sshserver/server.go` (lines 327-329)
  - Issue: TODO comment with simulation code for box creation
  - Action: Implement actual box creation logic or remove if not needed