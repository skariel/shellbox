# Suggested Commands for Shellbox Development

## Primary Development Commands

### Code Quality & Testing
```bash
./tst.sh    # Run ALL checks - format, lint, test, security
```

This runs:
1. **gofumpt** - Modern Go formatting (stricter than gofmt)
2. **goimports** - Organize imports
3. **golangci-lint** - Static analysis and linting
4. **govulncheck** - Security vulnerability check
5. **staticcheck** - Advanced static analysis

## Running the Applications

### Deploy Infrastructure
```bash
go run cmd/deploy/main.go <suffix>
```
Creates Azure infrastructure including resource groups, networking, and bastion host.

### Run Server
```bash
go run cmd/server/server.go <suffix>
```
Starts the SSH server on the bastion host, manages VM/volume pools.

## Development Tools (via tools.go)

### Individual Tool Commands
```bash
go run github.com/golangci/golangci-lint/cmd/golangci-lint run
go run golang.org/x/tools/cmd/goimports -w .
go run golang.org/x/vuln/cmd/govulncheck ./...
go run honnef.co/go/tools/cmd/staticcheck ./...
go run mvdan.cc/gofumpt -w .
```

## Git Commands
```bash
git status
git diff
git log --oneline -10
```

## System Utilities
- Standard Linux commands: ls, cd, grep, find, rg (ripgrep)
- SSH operations: ssh-keygen, ssh

## Important Notes
- Always run `./tst.sh` after making code changes
- Fix any errors reported by the linters before committing
- Use structured logging with log/slog (not printf)
- For searching code, prefer comby tool over grep/sed for structural operations