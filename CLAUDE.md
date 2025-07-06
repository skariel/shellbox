# CLAUDE.md

Shellbox.dev - SSH-based cloud development environments using Azure infrastructure.

## Project Overview

Cloud service providing instant development environments through SSH. Users connect to a bastion host that allocates Azure VMs running QEMU instances with full state preservation.

There is a pool of of volumes and a pool of instances. When a user "spins up" a box, a volume tagged as "free" is chosen and tagged with the user ID. When the user wants to connect to their box, the volume is connected to an instance tagged "free", qemu vm is resumed and ssh is forwarded to it. When the user disconnects, we "stop" the qemu vm while preserving memory, processes etc. When the user reconnects, we resume it.

"free" volumes are duplicated from a "golden" snapshot that has a "stopped" vm with ssh access. 

a user of this servie can use it by runing:
ssh shellbox.dev spinup dev1
ssh ubuntu@shellbox.dev connect dev1

the shellbox username is built from the user public key. the user "ubuntu" above is the user in the box vm, which can change as the user pleases and in accordance with linux limitations.

The system is still WIP, so not everything is implemented yet, and some thing that are implemented are just filler or temporary solutions to enable other developments.

I like this project because there is no UI, just CLI commands.

**Billing**: $0.70/hour active, $0.02/hour idle, auto-suspend at $5 balance

## Development

---------------------------------
MOST IMPORTANT:
- IF IN DOUBT, ALWAYS ASK FOR CLARIFICATIONS
- DO MINIMAL CODE CHANGES NEEDED TO ACOMPLISH TASKS
- BE CONSISTENT WITH EXISTING CODE: resue existing functions, maintain naming style, code patterns.
- NOTIFY ME OF ANY OPPORTUNITIES TO REMOVE UNNECESSARY TESTS OR OTHER CODE THAT IS MAINLY MAINTENANCE BURDEN
- USE THE SERENA MCP: initially, always read the instructions
---------------------------------------

format, lint and test that everything builds:
./tst.sh
run the above command after every session of code changes. Then fix any errors.

You have permission to search the internet whenever needed. Take advnatage of this!

use constants from the constants.go file instead of hard-coding values. Define new constants as needed.

Azure SDK: always use pointers and consistent polling

Retry operations with centralized helper

Resource naming with consistent patterns

resourse graph is used instead of keeping any local copy that could go inconsistent.

resources have tags for querying with the resource graph.

for operations on Azure, we wait with the retry function until we see it in the resource graph

use log/slog for structured logging, production uses json format, not printf, use key/value for structured logging

fail fatally for deployment -- simpler code handling

handle errors gracefully for runtime -- return errors

we use modern go with modern idioms (go 1.224)

Concurrency: Use `golang.org/x/sync` primitives

Dependencies: Latest Azure SDK v6/v7, `gliderlabs/ssh`, Cobra


## Key Components

- `cmd/server/`: SSH server on bastion host: manages pools and connections from users, commands etc.
- `cmd/deploy/`: Azure infrastructure deployment: deploys the bastion   
- `internal/infra/`: Azure resource management (VMs, networking, storage)
- `internal/sshserver/`: SSH proxy and command handling
- `internal/sshutil/`: SSH key management and remote operations

## Quick Commands

```bash
# List all functions
grep -rn -E "^func\s*(\([^)]+\))?\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\(" . --include="*.go"
```
