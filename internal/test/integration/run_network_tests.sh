#!/bin/bash

# Network Integration Tests Runner
# This script runs network infrastructure tests with proper configuration

set -euo pipefail

# Configuration
export TEST_CATEGORIES="integration"
export TEST_TIMEOUT="30m"
export TEST_CLEANUP_TIMEOUT="15m"
export TEST_PARALLEL_LIMIT="1"  # Run network tests sequentially to avoid conflicts
export TEST_RESOURCE_GROUP_PREFIX="nettest"
export TEST_LOCATION="westus2"

# Azure configuration  
export AZURE_CLIENT_ID="${AZURE_CLIENT_ID:-}"

# Verbose output for debugging
export VERBOSE="${VERBOSE:-true}"

echo "🔧 Network Integration Test Configuration:"
echo "  Categories: $TEST_CATEGORIES"
echo "  Timeout: $TEST_TIMEOUT"
echo "  Cleanup Timeout: $TEST_CLEANUP_TIMEOUT"
echo "  Parallel Limit: $TEST_PARALLEL_LIMIT"
echo "  Resource Group Prefix: $TEST_RESOURCE_GROUP_PREFIX"
echo "  Location: $TEST_LOCATION"
echo "  Azure CLI Mode: $([ -z "$AZURE_CLIENT_ID" ] && echo "yes" || echo "no")"
echo ""

# Azure tests always run - no skip logic

# Verify Azure CLI authentication if needed
if [ -z "$AZURE_CLIENT_ID" ]; then
    echo "🔍 Checking Azure CLI authentication..."
    if ! az account show >/dev/null 2>&1; then
        echo "❌ Azure CLI not authenticated. Please run 'az login' first."
        exit 1
    fi
    
    SUBSCRIPTION_ID=$(az account show --query id -o tsv)
    echo "✅ Azure CLI authenticated (Subscription: $SUBSCRIPTION_ID)"
    echo ""
fi

# Change to the project root
cd "$(dirname "$0")/../../.."

echo "🧪 Running Network Integration Tests..."
echo ""

# Run specific network tests
if [ "${1:-}" = "basic" ]; then
    echo "📡 Running basic network tests..."
    go test -v -tags=integration -timeout="$TEST_TIMEOUT" \
        ./internal/test/integration \
        -run="TestVNetCreation|TestVNetDeletion|TestNSGCreation|TestNSGDeletion"
elif [ "${1:-}" = "advanced" ]; then
    echo "🏗️  Running advanced network tests..."
    go test -v -tags=integration -timeout="$TEST_TIMEOUT" \
        ./internal/test/integration \
        -run="TestSubnetCreationWithinVNet|TestVNetWithNSGIntegration|TestNetworkErrorHandling"
elif [ "${1:-}" = "infrastructure" ]; then
    echo "🏛️  Running network infrastructure tests..."
    go test -v -tags=integration -timeout="$TEST_TIMEOUT" \
        ./internal/test/integration \
        -run="TestCreateNetworkInfrastructure|TestNetworkInfrastructureRetry|TestNetworkResourceDependencies"
elif [ "${1:-}" = "naming" ]; then
    echo "🏷️  Running network naming and configuration tests..."
    go test -v -tags=integration -timeout="$TEST_TIMEOUT" \
        ./internal/test/integration \
        -run="TestNetworkResourceNaming|TestNetworkConfigurationValidation"
else
    echo "🚀 Running all network tests..."
    go test -v -tags=integration -timeout="$TEST_TIMEOUT" \
        ./internal/test/integration \
        -run="TestVNet|TestNSG|TestSubnet|TestNetwork"
fi

echo ""
echo "✅ Network integration tests completed!"
echo ""
echo "💡 Usage examples:"
echo "  $0                    # Run all network tests"
echo "  $0 basic             # Run basic VNet/NSG creation/deletion tests"
echo "  $0 advanced          # Run subnet and integration tests"
echo "  $0 infrastructure    # Run full infrastructure tests"
echo "  $0 naming            # Run naming and configuration tests"
echo ""
echo "🔧 Environment variables:"
echo "  TEST_TIMEOUT=45m          # Set test timeout"
echo "  TEST_PARALLEL_LIMIT=2     # Set parallel test limit"
echo "  VERBOSE=false             # Reduce output verbosity"