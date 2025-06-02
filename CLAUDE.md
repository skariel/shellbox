# CLAUDE.md

Shellbox.dev - SSH-based cloud development environments using Azure infrastructure.

## Project Overview

Cloud service providing instant development environments through SSH. Users connect to a bastion host that allocates Azure VMs running QEMU instances with full state preservation.

**Architecture**: Bastion host → Azure VM pool → QEMU boxes with volume-based persistence  
**User Interface**: Pure SSH (no web clients required)  
**Billing**: $0.70/hour active, $0.02/hour idle, auto-suspend at $5 balance

## Development

### Build & Deploy
```bash
# Build binaries
go build -o server ./cmd/server
go build -o deploy ./cmd/deploy

# Run (requires resource group suffix)
./server <suffix>
./deploy <suffix>
```

### Code Quality
```bash
# All formatting, linting, security scanning, static analysis
./tst.sh
```

### Go Standards
- **Go 1.24** with modern idioms
- **Error Handling**: `log.Fatal()` for deployment, error returns for runtime
- **Concurrency**: Use `golang.org/x/sync` primitives
- **Dependencies**: Latest Azure SDK v6/v7, `gliderlabs/ssh`, Cobra
- **Logging**: Use `log/slog` with structured JSON format (see Logging section)
- **Tool Dependencies**: Use `tools.go` pattern for development dependencies

## Key Components

- `cmd/server/`: SSH server on bastion host
- `cmd/deploy/`: Azure infrastructure deployment  
- `internal/infra/`: Azure resource management (VMs, networking, storage)
- `internal/sshserver/`: SSH proxy and command handling
- `internal/sshutil/`: SSH key management and remote operations

## Configuration

- **SSH Keys**: `/home/shellbox/.ssh/id_rsa` (bastion), `$HOME/.ssh/id_ed25519` (deployment)
- **Azure Auth**: DefaultAzureCredential
- **Table Storage**: `/home/shellbox/.tablestorage.json`

## Logging

**Consistent JSON structured logging** across production and test environments using `log/slog`.

### Setup
```go
// Initialize logger (production setup automatically used in tests)
infra.SetDefaultLogger()
```

### Usage
```go
import "log/slog"

// Info level for normal operations
slog.Info("Creating volume", "name", volumeName, "sizeGB", sizeGB, "role", role)

// Debug level for detailed information  
slog.Debug("Test progress", "operation", "waiting", "timeout", "30s")

// Warn level for non-critical issues
slog.Warn("Resource not found", "resourceName", name, "error", err)

// Error level for failures
slog.Error("Failed to create resource", "type", "volume", "error", err)
```

### Guidelines
- **NEVER use `log.Printf()`** - always use structured `slog` calls
- **Include relevant context** as key-value pairs after the message
- **Use appropriate levels**: Debug for test details, Info for status, Warn for recoverable issues, Error for failures
- **Tests automatically use production logger** via `SetupTestEnvironment()` and `SetupMinimalTestEnvironment()`
- **JSON output** ensures consistent parsing across environments

## Code Patterns

### Struct Design
- **Tag-based metadata**: Use structs like `InstanceTags`, `VolumeTags` for resource metadata
- **Configuration structs**: Use `*Config` structs for function parameters (e.g., `VMConfig`, `VolumeConfig`)
- **Info structs**: Return structured information with `*Info` structs (e.g., `VolumeInfo`)

### Error Handling Patterns
```go
// Deployment-time failures (fail fast)
infra.FatalOnError(err, "failed to create resource")

// Runtime errors (graceful handling)
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

### Azure SDK Patterns
```go
// Always use to.Ptr() for Azure SDK pointer requirements
Location: to.Ptr(infra.Location),
VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(config.VMSize)),

// Standard polling with consistent options
poller, err := client.BeginCreateOrUpdate(ctx, params, nil)
result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
```

### Concurrency Patterns
```go
// Coordinate parallel operations
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // parallel work
}()
wg.Wait()

// Context-based timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

### Retry Pattern
```go
// Use centralized retry for Azure operations
err := infra.RetryOperation(ctx, func(ctx context.Context) error {
    // operation that may need retries
    return someAzureOperation(ctx)
}, 5*time.Minute, 5*time.Second, "operation description")
```

### Resource Naming
```go
// Use ResourceNamer for consistent naming
namer := infra.NewResourceNamer(suffix)
vmName := namer.BoxVMName(instanceID)
volumeName := namer.VolumePoolDiskName(volumeID)
```

### Testing Patterns
- **Shared resource group**: Use `shellbox-testing` for all integration tests
- **Production logger**: Tests use `infra.SetDefaultLogger()` for consistency
- **Category-based organization**: Tests organized by categories (unit, integration, golden, etc.)
- **Resource tracking**: Track created resources for cleanup with `env.TrackResource()`

## Guidelines

- Use `./tst.sh` for all code quality checks
- Maintain backwards compatibility
- Prefer minimal changes
- Handle errors gracefully at runtime, fail fast during deployment

## Memories

- test isolation is done by unique resource name suffix rather than having unique resource group

## Specific actions for when users asks to...

### list all functions

grep -rn --color=always -E "^func\s*(\([^)]+\))?\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\(" . --include="*.go" | grep -v -i test