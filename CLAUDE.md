# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Shellbox.dev is a cloud development environment service using SSH as its primary interface. Users connect and manage environments (aka Boxes) through standard SSH commands, with web browser only used for payment processing via QR codes.

Key service characteristics:
- Pay-per-use billing: Only charged during active SSH sessions
- Instant suspend on disconnect with complete state preservation (filesystem, memory, processes)
- Volume-based state management: Each Box lives in its own Azure-managed volume
- Instant connect: Azure VMs from ready pool connect to volumes and QEMU resumes the VM

## Architecture

### Core Components

- **Bastion Host**: Entry point (port 22) for SSH connections, runs shellbox server that manages box allocation
- **Boxes**: QEMU VMs with SSH access and suspend/restore capabilities, each living in its own Azure-managed volume
- **Instance Pool**: Ready-to-connect Azure VMs that can be connected to volumes on demand
- **Volume Pool**: Some volumes connected to active VMs, others waiting for users to request connection
- **Network Infrastructure**: Segmented Azure VNet - instances can only receive connections from bastion but have internet outbound access
- **Agentless Design**: Instances hosting boxes have no custom code; bastion manages them through Azure execution features

### Key Modules

- `cmd/server/`: Main server application that runs on the bastion host
- `cmd/deploy/`: Infrastructure deployment tool for setting up Azure resources
- `internal/infra/`: Azure infrastructure management (VMs, networking, CosmosDB)
- `internal/sshserver/`: SSH proxy server that forwards connections to allocated boxes
- `internal/sshutil/`: SSH utilities for key management and remote operations

### Data Flow

1. User connects via SSH to bastion host (port 22)
2. SSH server allocates available Azure VM from ready pool and connects it to user's volume
3. QEMU resumes the user's Box VM from the volume with full state preservation
4. Connection flows: User -> Bastion -> Instance -> Box (port 22)
5. On disconnect: Box suspends instantly, preserving complete state (filesystem, memory, processes)
6. Azure VM returns to ready pool, volume waits for next connection

## Development Commands

### Build and Run
```bash
# Build server binary
go build -o server ./cmd/server

# Build deployment tool
go build -o deploy ./cmd/deploy

# Run server (requires resource group suffix)
./server <resource-group-suffix>

# Deploy infrastructure
./deploy <resource-group-suffix>
```

### Code Quality and Testing
```bash
# Run all testing, linting, and formatting
./tst.sh
```

**Note**: All code quality tasks (testing, linting, formatting) are handled by `./tst.sh` - this is the single command for all quality checks.

## Configuration

- SSH keys: Server expects SSH keys at `/home/shellbox/.ssh/id_rsa` (bastion) and `$HOME/.ssh/id_ed25519` (deployment)
- Azure authentication: Uses DefaultAzureCredential (environment variables, managed identity, or Azure CLI)
- Azure Table Storage: Connection string stored in `/home/shellbox/.tablestorage.json` on bastion host

## Infrastructure

The project creates these Azure resources:
- Resource Group with configurable suffix
- Virtual Network `shellbox-network` (10.0.0.0/8) with bastion subnet (10.0.0.0/24) and boxes subnet (10.1.0.0/16)
- Network Security Groups: shared bastion NSG and individual per-box NSGs for complete network isolation
- Bastion VM with public IP running the shellbox server (accessible on ports 22, 443, and 2222)
- Pool of box VMs in the private subnet running nested QEMU instances
- Azure volumes for persistent Box state storage
- Azure Table Storage for state management and session tracking

### Network Security Architecture

- **Bastion NSG**: Applied at subnet level, allows SSH/HTTPS from internet and all traffic to boxes subnet
- **Box NSGs**: Individual NSG per box (format: `box-nsg-{uuid}`) applied at NIC level with rules:
  - Inbound: Allow SSH (ports 22, 2222) and ICMP from bastion subnet only
  - Outbound: Allow internet access, deny lateral movement to other boxes and reverse connections to bastion
- **Traffic Flow**: User → Bastion (port 22) → Box (port 22) with complete network isolation between boxes

## Development Guidelines

- Maintain simplicity in implementation
- Ensure consistency with existing codebase  
- Write essential comments explaining the why, not the what - cluster these at the beginning of logic blocks
- Always make minimal changes possible: if a task can be achieved in different ways, prefer the one that has minimal change.
- Maintain backwards compatiblity: as much as possible,  prefer non-braking changes to the codebase

### Error Handling Pattern

The codebase uses a deliberate mix of fatal and error value returns based on context:
- **Deployment time**: Use `log.Fatal()` for immediate failure when infrastructure setup encounters issues - deployment should fail fast and clearly
- **Runtime**: Return error values for graceful handling and service stability - the running service should handle errors gracefully to maintain stability for connected users  
- **Bastion creation**: Uses fatal errors during idempotent infrastructure creation as a simpler client creation mechanism instead of passing information through the wire during deployment
