#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}=== Running Test Coverage Analysis ===${NC}"

# Clean up any existing coverage files
rm -f unit_coverage.out integration_coverage.out combined_coverage.out

echo -e "\n${YELLOW}Running unit tests with coverage...${NC}"
TEST_CATEGORIES=unit go test -tags=unit -coverprofile=unit_coverage.out -coverpkg=./internal/... ./internal/test/unit/... -timeout=10m
unit_exit_code=$?

echo -e "\n${YELLOW}Running integration tests with coverage...${NC}"
TEST_CATEGORIES=integration go test -tags=integration -coverprofile=integration_coverage.out -coverpkg=./internal/... ./internal/test/integration/... -timeout=10m
integration_exit_code=$?

# Check if tests passed
if [ $unit_exit_code -ne 0 ]; then
    echo -e "${RED}Unit tests failed with exit code $unit_exit_code${NC}"
fi

if [ $integration_exit_code -ne 0 ]; then
    echo -e "${RED}Integration tests failed with exit code $integration_exit_code${NC}"
fi

# Show coverage results
echo -e "\n${BLUE}=== Coverage Results ===${NC}"

if [ -f unit_coverage.out ]; then
    echo -e "\n${GREEN}Unit Test Coverage:${NC}"
    unit_total=$(go tool cover -func=unit_coverage.out | tail -1 | awk '{print $3}')
    echo "Total: $unit_total"
    
    echo -e "\n${YELLOW}Top covered functions (unit tests):${NC}"
    go tool cover -func=unit_coverage.out | grep -v "0.0%" | grep -E "(infra|sshserver|sshutil)" | grep -v "test" | head -10
fi

if [ -f integration_coverage.out ]; then
    echo -e "\n${GREEN}Integration Test Coverage:${NC}"
    integration_total=$(go tool cover -func=integration_coverage.out | tail -1 | awk '{print $3}')
    echo "Total: $integration_total"
    
    echo -e "\n${YELLOW}Top covered functions (integration tests):${NC}"
    go tool cover -func=integration_coverage.out | grep -v "0.0%" | grep -E "(infra|sshserver|sshutil)" | grep -v "test" | head -10
fi

# Try to merge coverage files if both exist
if [ -f unit_coverage.out ] && [ -f integration_coverage.out ]; then
    echo -e "\n${YELLOW}Attempting to merge coverage files...${NC}"
    
    # Simple merge - this may not be perfect but gives an approximation
    (
        echo "mode: set"
        cat unit_coverage.out integration_coverage.out | grep -v "mode:" | sort -u
    ) > combined_coverage.out
    
    if [ -f combined_coverage.out ]; then
        echo -e "\n${GREEN}Combined Coverage:${NC}"
        combined_total=$(go tool cover -func=combined_coverage.out | tail -1 | awk '{print $3}')
        echo "Total: $combined_total"
        
        echo -e "\n${YELLOW}Functions with no coverage:${NC}"
        go tool cover -func=combined_coverage.out | grep "0.0%" | grep -E "(infra|sshserver|sshutil)" | grep -v "test" | head -20
    fi
fi

# Generate HTML reports
echo -e "\n${BLUE}Generating HTML reports...${NC}"
if [ -f unit_coverage.out ]; then
    go tool cover -html=unit_coverage.out -o unit_coverage.html
    echo "Unit coverage HTML: unit_coverage.html"
fi

if [ -f integration_coverage.out ]; then
    go tool cover -html=integration_coverage.out -o integration_coverage.html
    echo "Integration coverage HTML: integration_coverage.html"
fi

if [ -f combined_coverage.out ]; then
    go tool cover -html=combined_coverage.out -o combined_coverage.html
    echo "Combined coverage HTML: combined_coverage.html"
fi

echo -e "\n${BLUE}=== Coverage Analysis Complete ===${NC}"

# Exit with non-zero if any tests failed
if [ $unit_exit_code -ne 0 ] || [ $integration_exit_code -ne 0 ]; then
    exit 1
fi