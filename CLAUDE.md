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

## Guidelines

- Use `./tst.sh` for all code quality checks
- Maintain backwards compatibility
- Prefer minimal changes
- Handle errors gracefully at runtime, fail fast during deployment
