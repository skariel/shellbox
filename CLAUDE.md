# CLAUDE.md

Shellbox.dev - SSH-based cloud development environments using Azure infrastructure.

## Project Overview

Cloud service providing instant development environments through SSH. Users connect to a bastion host that allocates Azure VMs running QEMU instances with full state preservation.

There is a pool of "free" volumes, and when a user "spins up" a box, one is chosen and tagged with the user ID. When the user wwants to connect to their box, the volume is connected to a "free" instance from the pool instances and ssh is forwarded. When the user disconnects, we "stop" the machine while preserving memory, processes etc. When the user reconnects, we resume is.

"free" volumes are duplicated from a "golden" snapshot that has a "stopped" vm with ssh access etc. 

user ca run:
ssh shellbox.dev spinup dev1
ssh ubuntu@shellbox.dev connect dev1

the shellbox username is built from the user public key. the user "ubuntu" above is the user in the box vm, which can change as the user pleases and in accordance with linux limitations.

The system is still WIP, so not everything is implemented yet.

I like this project because there is no UI, just CLI commands.

**Architecture**: Bastion host → Azure VM pool → QEMU boxes with volume-based persistence  
**User Interface**: Pure SSH (no web clients required)  
**Billing**: $0.70/hour active, $0.02/hour idle, auto-suspend at $5 balance

## Development

---------------------------------
MOST IMPORTANT:
- DONT ASSUME -- IF IN DOUBT, ASK FOR CLARIFICATIONS
- DO MINIMAL CODE CHANGES NEEDED TO ACOMPLISH TASKS
- BE CONSISTENT WITH EXISTING CODE: resue existing functions, maintain naming style, code patterns.
- NOTIFY ME OF ANY OPPORTUNITIES TO REMOVE UNNECESSARY TESTS OR OTHER CODE THAT IS MAINLY MAINTENANCE BURDEN: THEN NOTIFY THE USER!
- USE THE LSP MCP SERVER... FOR DISCOVERIUNG TYPES, SIGNATURES, REFERENCES, HOVER INFORMATION ETC.
---------------------------------------

format, lint and tast that everything builds:
./tst.sh
run the above command after every session of code changes. Then fix any errors.

You have permission to search the internet whenever needed.

Use the [comby tool](https://comby.dev) for structural search-and-replace operations. It understands code structure better than regex, handles nested expressions, comments, and strings correctly. Prefer comby over grep/sed for pattern-based search tasks, but always prefer LSP for symbol-based operations.

don't hard-code values and string, use instead constants from the constants.go file. Define new constants as needed.

Azure SDK: always use pointers and consistent polling

Retry operations with centralized helper

Resource naming with consistent patterns

resourse graph is used instead of keeping any local copy that could go inconsistent.

resources have tags for querying with the resource graph.

for operations on Azure, we wait with the retry function until we see it in the resource graph

use log/slog for structured logging, production uses json format, not printf, use key/value for structured logging

fail fatally for deployment -- simpler code handling

handle errors gracefully for runtime -- return errors


### Go Standards
- **Go 1.24** with modern idioms
- **Error Handling**: `log.Fatal()` for deployment, error returns for runtime
- **Concurrency**: Use `golang.org/x/sync` primitives
- **Dependencies**: Latest Azure SDK v6/v7, `gliderlabs/ssh`, Cobra
- **Logging**: Use `log/slog` with structured JSON (production) / text (tests)
- **Tool Dependencies**: Use `tools.go` pattern for development dependencies

## Key Components

- `cmd/server/`: SSH server on bastion host: manages pools and connections from users, commands etc.
- `cmd/deploy/`: Azure infrastructure deployment: deploys the bastion   
- `internal/infra/`: Azure resource management (VMs, networking, storage)
- `internal/sshserver/`: SSH proxy and command handling
- `internal/sshutil/`: SSH key management and remote operations

## Quick Commands

```bash
# List all functions
grep -rn -E "^func\s*(\([^)]+\))?\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\(" . --include="*.go" | grep -v
```
