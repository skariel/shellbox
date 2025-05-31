# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Shellbox.dev (https://shellbox.dev/) is a cloud development environment service using SSH as its primary interface. Users connect and manage environments (aka Boxes) through standard SSH commands, with web browser only used for payment processing via QR codes.

### Service Specifications
- **High-Performance Instances**: 8 vCPUs, 32GB RAM, 96GB SSD, 100GB networking at 1Gbps
- **Billing Model**: $0.70/hour while actively connected, $0.02/hour while idle
- **Automatic Cost Control**: Instances stop when balance falls below $5
- **Prepaid Balance System**: Minimum $10 top-up with refund options

### Key Service Characteristics
- Pay-per-use billing: Only charged during active SSH sessions
- Instant suspend on disconnect with complete state preservation (filesystem, memory, processes)
- Volume-based state management: Each Box lives in its own Azure-managed volume
- Instant connect: Azure VMs from ready pool connect to volumes and QEMU resumes the VM
- Zero-configuration setup: No special clients or browser plugins required
- Full SSH feature support: Port forwarding, SCP, and all standard SSH functionality

### User Commands
Users interact with the service entirely through SSH commands:

```bash
# Create a new development box
ssh shellbox.dev spinup <name>

# Connect to an existing box (connects as ubuntu user by default)
ssh <box-name>.shellbox.dev

# Connect to an existing box with specific user
ssh <username>@<box-name>.shellbox.dev

# Check account status and box list
ssh shellbox.dev

# Get help
ssh shellbox.dev help

# Check current user
ssh shellbox.dev whoami

# View version information
ssh shellbox.dev version
```

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

## Go File Reference

### Command Line Tools

#### `cmd/deploy/main.go`
Entry point for infrastructure deployment tool. Creates network infrastructure and deploys bastion host with the shellbox server binary. Requires resource group suffix as command line argument.

#### `cmd/server/server.go` 
Main server application that runs on the bastion host. Initializes Azure clients, maintains box pool, starts SSH server, and logs server start events to Azure Table Storage.

### Infrastructure Management (`internal/infra/`)

#### `bastion.go`
Bastion host deployment functions. Handles VM creation, SSH key setup, server binary compilation/deployment, role assignments, and Table Storage configuration. Contains complete bastion setup workflow with retry logic.

#### `box.go`
Box VM management including creation, networking setup (NSGs, NICs), QEMU initialization scripts, and lifecycle operations (deallocate, status queries). Each box gets isolated networking and nested VM capabilities.

#### `clients.go`
Azure SDK client initialization and credential management. Handles both Azure CLI and managed identity authentication, subscription discovery, and Table Storage client setup.

#### `constants.go`
Configuration constants for networking (VNet/subnet CIDRs), VM settings, SSH ports, key paths, and NSG security rules. Central location for all infrastructure configuration values.

#### `network.go`
Network infrastructure creation including resource groups, VNets, subnets, and NSGs. Handles the foundational networking setup that both bastion and boxes depend on.

#### `pool.go`
Box pool management that maintains a target number of ready boxes. Runs as background goroutine, creates new boxes when needed, and logs box lifecycle events to Table Storage.

#### `resource_naming.go`
Consistent Azure resource naming functions based on suffix. Generates names for all resources (VMs, NICs, NSGs, disks, etc.) following predictable patterns for easy identification.

#### `retry.go`
Generic retry mechanism with timeout and interval controls. Used throughout infrastructure operations where Azure resources may need time to propagate or become available.

#### `tables.go`
Azure Table Storage operations for event logging and resource registry. Provides structured logging of server events, box lifecycle, and user sessions for monitoring and debugging.

### SSH Server (`internal/sshserver/`)

#### `server.go`
Core SSH server that proxies connections to boxes. Handles both interactive shell sessions and command execution, implements SCP support, manages PTY forwarding, and logs all session activity.

#### `commands.go`
Command parsing using Cobra framework for non-interactive SSH commands (spinup, help, version, whoami). Provides structured command handling with proper argument validation and help text.

### SSH Utilities (`internal/sshutil/`)

#### `ssh.go`
SSH key management and remote operation utilities. Handles key pair generation/loading, secure file copying via SCP, and remote command execution with proper error handling.
