# Code Style and Conventions

## Go Language Standards
- **Version**: Go 1.24 with modern idioms
- **Formatting**: Use gofumpt (stricter than gofmt)
- **Imports**: Organized with goimports

## Error Handling
- **Deployment code**: Use `log.Fatal()` for simplicity
- **Runtime code**: Return errors gracefully for proper handling
- **Retry operations**: Use centralized `RetryOperation` helper

## Logging
- **Package**: Use `log/slog` for structured logging
- **Production**: JSON format
- **Tests**: Text format
- **Never use**: printf-style logging
- **Format**: Key/value pairs for structured data
```go
slog.Info("message", "key1", value1, "key2", value2)
```

## Code Organization
- **Constants**: Define in constants.go, never hard-code values
- **Resource naming**: Use consistent patterns via ResourceNamer
- **Azure operations**: 
  - Always use pointers
  - Consistent polling with DefaultPollOptions
  - Wait with retry function until visible in resource graph
- **Concurrency**: Use golang.org/x/sync primitives

## Testing & Quality
- **Linting**: golangci-lint with custom config
- **Security**: gosec enabled (with specific exclusions)
- **Complexity**: Max cyclomatic complexity of 15
- **Static analysis**: staticcheck and revive

## Important Principles
- **DON'T ASSUME**: Ask for clarification when in doubt
- **Minimal changes**: Do only what's needed to accomplish tasks
- **Consistency**: Reuse existing functions, maintain naming style
- **Maintenance**: Notify about opportunities to remove unnecessary code
- **LSP usage**: Use the LSP MCP server for discovering types, signatures, references

## Resource Management
- Use Azure Resource Graph instead of local state
- Tag resources for querying
- Implement proper retry and wait logic for Azure operations