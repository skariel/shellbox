# Shellbox.dev Project Overview

## Project Purpose
Shellbox.dev is an SSH-based cloud development environment service using Azure infrastructure. Users connect to a bastion host that allocates Azure VMs running QEMU instances with full state preservation (memory, processes). The service provides instant development environments through pure SSH without requiring web clients.

## Core Architecture
- **Bastion Host**: Entry point for SSH connections, manages pools and user commands
- **Azure VM Pool**: Maintains pools of "free" instances that can be allocated to users
- **QEMU Boxes**: Run inside Azure VMs, provide the actual development environment with state preservation
- **Volume-based Persistence**: Uses Azure disk volumes duplicated from "golden" snapshots

## Key Features
- SSH-only interface (no web UI)
- Full state preservation when disconnected (memory, processes)
- Pool-based resource allocation for instant connections
- Billing: $0.70/hour active, $0.02/hour idle, auto-suspend at $5 balance

## User Commands
```bash
ssh shellbox.dev spinup dev1      # Create a new box
ssh ubuntu@shellbox.dev connect dev1  # Connect to existing box
```

## Tech Stack
- **Language**: Go 1.24
- **Cloud**: Azure (SDK v6/v7)
- **Key Dependencies**:
  - Azure SDK for compute, network, storage, resource graph
  - gliderlabs/ssh for SSH server
  - Cobra for CLI
  - golang.org/x/sync for concurrency
- **Logging**: log/slog with structured JSON (production) / text (tests)

## Project Status
Work in progress (WIP) - not all features are fully implemented yet.