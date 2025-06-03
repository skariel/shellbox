# Shellbox Test Makefile

.PHONY: test test-unit test-integration test-e2e test-all clean-test help

# Test configuration
TEST_TIMEOUT ?= 60m
TEST_PARALLEL ?= 4

# Individual test categories (all generate coverage)
test-unit:
	@echo "Running unit tests with coverage..."
	TEST_CATEGORIES=unit go test -v -coverprofile=coverage.out -parallel $(TEST_PARALLEL) -timeout 5m ./internal/test/...
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage percentage:"
	@go tool cover -func=coverage.out | grep total

test-integration:
	@echo "Running integration tests with coverage (includes cleanup verification)..."
	TEST_CATEGORIES=integration go test -v -coverprofile=coverage.out -parallel $(TEST_PARALLEL) -timeout 20m ./internal/test/...
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage percentage:"
	@go tool cover -func=coverage.out | grep total

test-e2e:
	@echo "Running end-to-end tests with coverage (includes cleanup verification)..."
	TEST_CATEGORIES=e2e go test -v -coverprofile=coverage.out -parallel $(TEST_PARALLEL) -timeout 45m ./internal/test/...
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage percentage:"
	@go tool cover -func=coverage.out | grep total

# Combined test targets
test-all:
	@echo "Running all tests sequentially with coverage..."
	TEST_CATEGORIES=unit,integration,e2e go test -v -coverprofile=coverage.out -parallel $(TEST_PARALLEL) -timeout $(TEST_TIMEOUT) ./internal/test/...
	@echo "Generating HTML coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage percentage:"
	@go tool cover -func=coverage.out | grep total

# Default target
test:
	@make test-unit

# Clean up test resources
clean-test:
	@echo "Cleaning up all resources in shellbox-testing resource group..."
	az resource list --resource-group shellbox-testing --query "[].id" -o tsv | xargs -r -I {} az resource delete --ids {} --verbose
	@echo "Cleanup complete"

# Help
help:
	@echo "Shellbox Test Commands:"
	@echo ""
	@echo "Test Categories (all generate coverage.out and coverage.html):"
	@echo "  make test              - Run unit tests (default)"
	@echo "  make test-unit         - Run unit tests only"
	@echo "  make test-integration  - Run integration tests only"
	@echo "  make test-e2e          - Run end-to-end tests only"
	@echo "  make test-all          - Run all tests sequentially"
	@echo ""
	@echo "Utilities:"
	@echo "  make clean-test        - Clean up test resources"
	@echo "  make help              - Show this help"
	@echo ""
	@echo "Configuration:"
	@echo "  TEST_PARALLEL=$(TEST_PARALLEL)  - Parallel test execution within categories"
	@echo "  TEST_TIMEOUT=$(TEST_TIMEOUT)   - Overall test timeout"
	@echo ""
	@echo "Output: Each test run generates coverage.out and coverage.html"