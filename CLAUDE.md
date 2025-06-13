# CLAUDE.md

Shellbox.dev - SSH-based cloud development environments using Azure infrastructure.

## Project Overview

Cloud service providing instant development environments through SSH. Users connect to a bastion host that allocates Azure VMs running QEMU instances with full state preservation.

**Architecture**: Bastion host → Azure VM pool → QEMU boxes with volume-based persistence  
**User Interface**: Pure SSH (no web clients required)  
**Billing**: $0.70/hour active, $0.02/hour idle, auto-suspend at $5 balance

## Development


---------------------------------
MOST IMPORTANT:
- DONT ASSUME -- IF IN DOUBT, ASK FOR CLARIFICATIONS
- KEEP CODE SIMPLE
- DO MINIMAL CODE CHANGES NEEDED TO ACOMPLISH TASKS
- BE CONSISTENT WITH EXISTING CODE: resue existing functions, maintain naming style, code patterns.
- IF YOU SEE ANY OPPORTUNITY TO REMOVE UNNECESSARY TESTS OR OTHER CODE THAT IS MAINLY MAINTENANCE BURDEN: THEN NOTIFY THE USER!
- USE THE LSP MCP SERVER... FOR DISCOVERIUNG TYPES, SIGNATURES, REFERENCES. RENAMING, SEARCHING ETC.ND MOST IMPORTANTLY FOR EDITING! THE EDITS WITH THE LSP ARE VERY ACCURATE
-----------------------------------------


format, lint and tast that everything builds:
./tst.sh
run the above command after every session of code changes. Then fix any errors.

You have permission to search the internet whenever needed.

Always use LSP (Language Server Protocol) for code navigation and refactoring**. The Go LSP provides accurate, context-aware code intelligence that is far superior to text-based search tools.
For methods, always use fully qualified names with the receiver type:
- Use `Type.Method` format (e.g., `ResourceGraphQueries.CountInstancesByStatus`)
- For standalone functions, just the function name is sufficient
- For interface methods, use `InterfaceName.MethodName`

The language server has to be used with exact names -- when you actually know these. When you don't know exact names use instead the indexing tools to do semantic searching for code. When using the indexing service for the first time each session, remember to first set the project path

When you change some files -- run the reindexing again!

Use the [comby tool](https://comby.dev) for structural search-and-replace operations. It understands code structure better than regex, handles nested expressions, comments, and strings correctly. Prefer comby over grep/sed for pattern-based search tasks, but always prefer LSP for symbol-based operations.

don't hard-code values and string, use instead constants from the constants.go file. Define new constants as needed.

Azure SDK: always use pointers and consistent polling

Retry operations with centralized helper

Resource naming with consistent patterns

resourse graph is used instead of keeping any local copy that could go inconsistent.

resources have tags for querying with the resource graph.

for operations on Azure, we wait with the retry function until we see it in the resource graph

### Go Standards
- **Go 1.24** with modern idioms
- **Error Handling**: `log.Fatal()` for deployment, error returns for runtime
- **Concurrency**: Use `golang.org/x/sync` primitives
- **Dependencies**: Latest Azure SDK v6/v7, `gliderlabs/ssh`, Cobra
- **Logging**: Use `log/slog` with structured JSON (production) / text (tests)
- **Tool Dependencies**: Use `tools.go` pattern for development dependencies

## Key Components

- `cmd/server/`: SSH server on bastion host
- `cmd/deploy/`: Azure infrastructure deployment  
- `internal/infra/`: Azure resource management (VMs, networking, storage)
- `internal/sshserver/`: SSH proxy and command handling
- `internal/sshutil/`: SSH key management and remote operations

### SSH Server (`internal/sshserver/`)
- `server.go` (415 lines) - SSH proxy with session management
- `commands.go` (155 lines) - Cobra CLI parsing (spinup/help/version/whoami)

### Key Functions by Module
- **Resource Discovery**: `ResourceGraphQueries.FindFreeInstances()`, `ResourceGraphQueries.CountInstancesByStatus()`
- **Resource Allocation**: `ResourceAllocator.AllocateResourcesForUser()`, `ResourceAllocator.ReleaseResourcesForUser()`
- **VM Operations**: `CreateInstance()`, `DestroyInstance()`, `QEMUManager.StartQEMUWithVolume()`
- **Retry Logic**: `RetryOperation()` - used throughout for Azure operations
- **SSH Operations**: `ExecuteSSHCommand()`, `LoadKeyPair()`

## Logging

Use `log/slog` for structured logging. Production uses JSON format, tests use text format for readability.

**Never use `log.Printf()`** - always use structured `slog` calls with key-value context.

## Code Patterns

### Common Patterns
```go
// Error handling: fail fast (deployment) vs graceful (runtime)
infra.FatalOnError(err, "deployment failed")
if err != nil { return fmt.Errorf("operation failed: %w", err) }
```


### Struct Conventions
- **Metadata**: `*Tags` structs (e.g., `InstanceTags`, `VolumeTags`)
- **Configuration**: `*Config` structs for parameters (e.g., `VMConfig`, `VolumeConfig`)
- **Results**: `*Info` structs for return data (e.g., `VolumeInfo`)

## Testing

- **Resource Group**: Use `shellbox-testing` with unique resource name prefixes per test
- **Cleanup**: Each test removes its own resources; verification test runs after all tests
- **Tracking**: Use `env.TrackResource()` for resource cleanup

## Quick Commands

```bash
# List all functions
grep -rn -E "^func\s*(\([^)]+\))?\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\(" . --include="*.go" | grep -v
```
