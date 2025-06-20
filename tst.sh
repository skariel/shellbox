#!/bin/bash
set -e

echo "🔧 Running Go modernization and quality checks..."

# Modern formatting with gofumpt (stricter than gofmt)
echo "📝 Formatting code with gofumpt..."
find . -name "*.go" -not -path "./vendor/*" -exec gofumpt -w {} \;

# Import organization  
echo "📦 Organizing imports..."
find . -name "*.go" -not -path "./vendor/*" -exec goimports -w {} \;

# Static analysis and linting
echo "🔍 Running static analysis..."
golangci-lint run --timeout 10m

# Security vulnerability check
echo "🛡️  Checking for security vulnerabilities..."
govulncheck ./...

# Advanced static analysis
echo "🔬 Running staticcheck..."
staticcheck ./...

# Dead code detection
echo "🧹 Checking for dead code..."
go run golang.org/x/tools/cmd/deadcode@latest ./...

echo "✅ All checks completed successfully!"
