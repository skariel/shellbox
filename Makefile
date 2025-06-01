# Shellbox Test Makefile

.PHONY: test test-unit test-client test-integration test-compute test-golden test-pool test-e2e test-all test-fast test-slow clean-test

# Default test configuration
TEST_TIMEOUT ?= 60m
TEST_PARALLEL ?= 4

# Test categories
test-unit:
	@echo "Running unit tests (< 30s)..."
	TEST_CATEGORIES=unit go test -tags=unit -parallel $(TEST_PARALLEL) -timeout 2m ./internal/test/...

test-client:
	@echo "Running client tests (< 2m)..."
	TEST_CATEGORIES=client go test -tags=client -parallel $(TEST_PARALLEL) -timeout 5m ./internal/test/...

test-integration:
	@echo "Running integration tests (< 10m)..."
	TEST_CATEGORIES=integration go test -tags=integration -parallel 2 -timeout 15m ./internal/test/...

test-compute:
	@echo "Running compute tests (< 15m)..."
	TEST_CATEGORIES=compute go test -tags=compute -parallel 2 -timeout 20m ./internal/test/...

test-golden:
	@echo "Running golden snapshot tests (< 30m)..."
	TEST_CATEGORIES=golden go test -tags=golden -parallel 1 -timeout 35m ./internal/test/...

test-pool:
	@echo "Running pool tests (< 30m)..."
	TEST_CATEGORIES=pool go test -tags=pool -parallel 1 -timeout 35m ./internal/test/...

test-e2e:
	@echo "Running end-to-end tests (< 45m)..."
	TEST_CATEGORIES=e2e go test -tags=e2e -parallel 1 -timeout 50m ./internal/test/...

# Combined test targets
test-fast:
	@echo "Running fast tests (unit + client)..."
	TEST_CATEGORIES=unit,client go test -tags="unit,client" -parallel $(TEST_PARALLEL) -timeout 5m ./internal/test/...

test-slow:
	@echo "Running slow tests (compute + golden + pool + e2e)..."
	TEST_CATEGORIES=compute,golden,pool,e2e go test -tags="compute,golden,pool,e2e" -parallel 1 -timeout $(TEST_TIMEOUT) ./internal/test/...

test-all:
	@echo "Running all tests..."
	TEST_CATEGORIES=all go test -tags="unit,client,integration,compute,golden,pool,e2e" -parallel 2 -timeout $(TEST_TIMEOUT) ./internal/test/...

# Development shortcuts
test:
	@make test-unit

test-with-integration:
	@echo "Running unit + client + integration tests..."
	TEST_CATEGORIES=unit,client,integration go test -tags="unit,client,integration" -parallel 2 -timeout 20m ./internal/test/...

# Skip expensive tests
test-no-golden:
	@echo "Running all tests except golden snapshots..."
	SKIP_GOLDEN_SNAPSHOT=true TEST_CATEGORIES=unit,client,integration,compute,pool,e2e go test -tags="unit,client,integration,compute,pool,e2e" -parallel 2 -timeout 45m ./internal/test/...

test-no-slow:
	@echo "Running all tests except slow ones..."
	SKIP_GOLDEN_SNAPSHOT=true SKIP_POOL_TESTS=true SKIP_E2E_TESTS=true TEST_CATEGORIES=unit,client,integration,compute go test -tags="unit,client,integration,compute" -parallel 2 -timeout 25m ./internal/test/...

# Clean up test resources (emergency cleanup)
clean-test:
	@echo "Cleaning up any leftover test resources..."
	@echo "This would run cleanup scripts to remove test-* resource groups"
	# TODO: Implement cleanup script

# Verbose testing
test-verbose:
	TEST_CATEGORIES=unit go test -tags=unit -v -parallel $(TEST_PARALLEL) -timeout 2m ./internal/test/...

# Show test configuration
test-config:
	@echo "Test Configuration:"
	@echo "  Categories: $${TEST_CATEGORIES:-unit}"
	@echo "  Parallel: $(TEST_PARALLEL)"
	@echo "  Timeout: $(TEST_TIMEOUT)"
	@echo "  Skip Golden: $${SKIP_GOLDEN_SNAPSHOT:-false}"
	@echo "  Skip Pool: $${SKIP_POOL_TESTS:-false}"
	@echo "  Skip E2E: $${SKIP_E2E_TESTS:-false}"
	@echo "  Azure CLI: $${AZURE_CLIENT_ID:-not set (will use Azure CLI)}"

# Help
help:
	@echo "Shellbox Test Commands:"
	@echo ""
	@echo "Quick tests:"
	@echo "  make test          - Run unit tests only (< 30s)"
	@echo "  make test-fast     - Run unit + client tests (< 2m)"
	@echo ""
	@echo "Individual categories:"
	@echo "  make test-unit        - Unit tests (< 30s)"
	@echo "  make test-client      - Client tests (< 2m)"
	@echo "  make test-integration - Integration tests (< 10m)"
	@echo "  make test-compute     - Compute tests (< 15m)"
	@echo "  make test-golden      - Golden snapshot tests (< 30m)"
	@echo "  make test-pool        - Pool tests (< 30m)"
	@echo "  make test-e2e         - End-to-end tests (< 45m)"
	@echo ""
	@echo "Combined tests:"
	@echo "  make test-all         - All tests (< 60m)"
	@echo "  make test-slow        - Only slow tests"
	@echo "  make test-no-golden   - All except golden snapshots"
	@echo "  make test-no-slow     - All except slow tests"
	@echo ""
	@echo "Utilities:"
	@echo "  make test-config      - Show current test configuration"
	@echo "  make clean-test       - Clean up leftover test resources"
	@echo ""
	@echo "Environment variables:"
	@echo "  TEST_CATEGORIES       - Comma-separated list of categories to run"
	@echo "  SKIP_GOLDEN_SNAPSHOT  - Skip golden snapshot tests"
	@echo "  SKIP_POOL_TESTS       - Skip pool tests"
	@echo "  SKIP_E2E_TESTS        - Skip end-to-end tests"
	@echo "  SKIP_AZURE_TESTS      - Skip all Azure tests"