#!/bin/bash
set -e
echo "🔧 Running Go modernization and quality checks..."

# Check go.mod tidiness
echo "📋 Checking go.mod tidiness..."
go mod tidy -v

# Format code (since golangci-lint only checks, doesn't fix)
echo "📝 Formatting code..."
gofumpt -w .
goimports -w .

# Clear golangci-lint cache to ensure fresh results
echo "🧹 Clearing linter cache..."
golangci-lint cache clean

# Static analysis and linting (only check files changed since last commit)
echo "🔍 Running static analysis..."
if git rev-parse HEAD~1 >/dev/null 2>&1; then
    golangci-lint run --timeout 10m --new-from-rev=HEAD~1 --fix
else
    golangci-lint run --timeout 10m --fix
fi

# Security vulnerability check
echo "🛡️  Checking for security vulnerabilities..."
govulncheck ./...

echo "✅ All checks completed successfully!"
