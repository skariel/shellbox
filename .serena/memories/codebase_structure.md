# Codebase Structure

## Directory Layout

### `/cmd` - Application entry points
- `/cmd/server/server.go` - SSH server that runs on bastion host
  - Manages pool maintenance
  - Handles SSH connections
  - Creates golden snapshots
- `/cmd/deploy/main.go` - Infrastructure deployment tool
  - Creates Azure resource groups
  - Sets up networking
  - Deploys bastion host

### `/internal` - Core implementation
- `/internal/infra/` - Azure infrastructure management
  - `clients.go` - Azure client initialization
  - `constants.go` - All hardcoded values and configuration
  - `instances.go` - VM instance management
  - `volumes.go` - Disk volume management
  - `network.go` - Virtual network setup
  - `pool.go` - Resource pool management
  - `resource_graph_queries.go` - Azure Resource Graph queries
  - `resource_allocator.go` - Resource allocation logic
  - `bastion.go` - Bastion host deployment
  - `golden_snapshot.go` - Golden image creation
  - `qemu_manager.go` - QEMU VM management
  - `tables.go` - Azure Table Storage operations
  - `retry.go` - Retry logic for operations
  - `logger.go` - Logging configuration
  - `resource_naming.go` - Consistent resource naming

- `/internal/sshserver/` - SSH server implementation
  - `server.go` - Main SSH server logic
  - `commands.go` - SSH command handling

- `/internal/sshutil/` - SSH utilities
  - `ssh.go` - SSH key management and operations

### Root Files
- `go.mod`, `go.sum` - Go module definitions
- `tools.go` - Development tool dependencies
- `tst.sh` - Quality check script
- `.golangci.yml` - Linter configuration
- `CLAUDE.md` - Project instructions and guidelines
- `.mcp.json` - MCP server configuration

### Configuration Files
- `.serena/` - Serena MCP configuration
- `.claude/` - Claude-specific settings
- `.gitignore` - Git ignore rules

## Key Architectural Components
1. **Resource Management**: Uses Azure Resource Graph for state (no local state)
2. **Pool System**: Maintains pools of free VMs and volumes for instant allocation
3. **Golden Snapshots**: Base images for quick volume creation
4. **Table Storage**: Event logging and resource registry
5. **SSH-only Interface**: No web UI, pure CLI experience