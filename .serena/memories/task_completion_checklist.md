# Task Completion Checklist

## After Making Code Changes

### 1. Run Quality Checks (MANDATORY)
```bash
./tst.sh
```

This command MUST be run after every session of code changes. It will:
- Format code with gofumpt
- Organize imports with goimports  
- Run static analysis with golangci-lint
- Check for security vulnerabilities with govulncheck
- Perform advanced static analysis with staticcheck

### 2. Fix Any Errors
If `./tst.sh` reports any errors, you MUST fix them before considering the task complete.

### 3. Verify Build
Ensure the code compiles without errors:
```bash
go build ./...
```

### 4. Run Tests (if applicable)
```bash
go test ./...
```

### 5. Review Changes
- Ensure code follows existing patterns
- Check that no hard-coded values were added (use constants.go)
- Verify structured logging is used (log/slog)
- Confirm error handling follows conventions (Fatal for deploy, return for runtime)

## Important Reminders
- NEVER commit unless explicitly asked by the user
- If lint/typecheck commands are unknown, ask the user and suggest adding to CLAUDE.md
- Ensure all Azure operations use retry logic and wait for resource graph visibility
- Keep code changes minimal and focused on the task at hand