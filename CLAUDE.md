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

### Code Quality & Refactoring
```bash
# All formatting, linting, security scanning, static analysis
./tst.sh
```

### Code Navigation & Refactoring

**IMPORTANT: Always use LSP (Language Server Protocol) for code navigation and refactoring**. The Go LSP provides accurate, context-aware code intelligence that is far superior to text-based search tools.

#### LSP Operations (STRONGLY PREFERRED)
Use these LSP commands for all code navigation:
- **Find function/type definitions**: Use `mcp__go-language-server__definition` to jump to where a symbol is defined
- **Find all references**: Use `mcp__go-language-server__references` to find all usages of a symbol across the codebase
- **Get type information**: Use `mcp__go-language-server__hover` to see type info and documentation at a position
- **Rename symbols**: Use `mcp__go-language-server__rename_symbol` to safely rename across all files
- **Get diagnostics**: Use `mcp__go-language-server__diagnostics` to check for errors in a file
- **Edit files**: Use `mcp__go-language-server__edit_file` for LSP-aware file modifications

Examples:
```bash
# Find where a function is defined
mcp__go-language-server__definition symbolName="CountInstancesByStatus"

# Find all places where a type/function is used
mcp__go-language-server__references symbolName="ResourceGraphQueries"

# Get type info at specific position
mcp__go-language-server__hover filePath="/path/to/file.go" line=42 column=15

# Rename a symbol across entire codebase
mcp__go-language-server__rename_symbol filePath="/path/to/file.go" line=10 column=5 newName="NewSymbolName"
```

**IMPORTANT**: For methods, always use fully qualified names with the receiver type:
- Use `Type.Method` format (e.g., `ResourceGraphQueries.CountInstancesByStatus`)
- For standalone functions, just the function name is sufficient
- For interface methods, use `InterfaceName.MethodName`

#### Structural Search (Fallback Only)
Only use comby when LSP cannot help (e.g., finding patterns rather than specific symbols):
```bash
# Structural code search (use only when LSP is insufficient)
comby ':[pattern]' '' -language go -match-only

# Structural code refactoring (use only when LSP rename is insufficient)
comby ':[pattern]' ':[replacement]' -language go
```

**Comby**: Use the [comby tool](https://comby.dev) for structural search-and-replace operations. It understands code structure better than regex, handles nested expressions, comments, and strings correctly. Prefer comby over grep/sed for pattern-based search tasks, but always prefer LSP for symbol-based operations.

#### Comby Search Examples
```bash
# Find all function calls to specific function
comby 'slog.:[level](:[args])' '' -language go -match-only

# Find error handling patterns
comby 'if err != nil { :[body] }' '' -language go -match-only

# Find struct field assignments
comby ':[var] := :[type]{:[fields]}' '' -language go -match-only

# Find Azure SDK polling patterns
comby ':[var], err := :[client].BeginCreateOrUpdate(:[args])' '' -language go -match-only

# Find retry operation calls
comby 'infra.RetryOperation(:[args])' '' -language go -match-only
```

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

## Configuration

- **SSH Keys**: `/home/shellbox/.ssh/id_rsa` (bastion), `$HOME/.ssh/id_ed25519` (deployment)
- **Azure Auth**: DefaultAzureCredential
- **Table Storage**: `/home/shellbox/.tablestorage.json`

## Logging

Use `log/slog` for structured logging. Production uses JSON format, tests use text format for readability.

```go
// Setup: infra.SetDefaultLogger() (production) or SetupTestEnvironment() (tests)
slog.Info("Creating volume", "name", volumeName, "sizeGB", sizeGB)
slog.Error("Failed to create resource", "type", "volume", "error", err)
```

**Never use `log.Printf()`** - always use structured `slog` calls with key-value context.

## Code Patterns

### Common Patterns
```go
// Error handling: fail fast (deployment) vs graceful (runtime)
infra.FatalOnError(err, "deployment failed")
if err != nil { return fmt.Errorf("operation failed: %w", err) }

// Azure SDK: always use pointers and consistent polling
Location: to.Ptr(infra.Location)
poller, err := client.BeginCreateOrUpdate(ctx, params, nil)
result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)

// Retry operations with centralized helper
err := infra.RetryOperation(ctx, operation, 5*time.Minute, 5*time.Second, "description")

// Resource naming with consistent patterns
namer := infra.NewResourceNamer(suffix)
vmName := namer.BoxVMName(instanceID)
```

### Struct Conventions
- **Metadata**: `*Tags` structs (e.g., `InstanceTags`, `VolumeTags`)
- **Configuration**: `*Config` structs for parameters (e.g., `VMConfig`, `VolumeConfig`)
- **Results**: `*Info` structs for return data (e.g., `VolumeInfo`)

## Testing

- **Resource Group**: Use `shellbox-testing` with unique resource name prefixes per test
- **Cleanup**: Each test removes its own resources; verification test runs after all tests
- **Categories**: Organized by unit, integration, e2e
- **Tracking**: Use `env.TrackResource()` for resource cleanup

## Guidelines

- **ALWAYS use LSP** for finding functions, types, references, and renaming symbols
- Use `./tst.sh` for all code quality checks
- Use `comby` for structural pattern matching (only when LSP is insufficient)
- Maintain backwards compatibility, prefer minimal changes
- Handle errors gracefully at runtime, fail fast during deployment

## Quick Commands

```bash
# List all functions
grep -rn -E "^func\s*(\([^)]+\))?\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\(" . --include="*.go" | grep -v -i test
```
