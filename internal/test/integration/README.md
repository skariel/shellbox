# Network Integration Tests

This directory contains integration tests for Azure network infrastructure components including Virtual Networks (VNets), Network Security Groups (NSGs), and subnets.

## Test Structure

### Basic Network Tests (`network_test.go`)

Tests individual network components in isolation:

- **`TestVNetCreation`**: Creates a VNet with two subnets and verifies properties
- **`TestVNetDeletion`**: Tests VNet deletion and cleanup
- **`TestNSGCreation`**: Creates an NSG with security rules and verifies configuration
- **`TestNSGDeletion`**: Tests NSG deletion and cleanup
- **`TestSubnetCreationWithinVNet`**: Tests subnet creation and deletion within an existing VNet
- **`TestVNetWithNSGIntegration`**: Tests VNet creation with NSG-attached subnets
- **`TestNetworkErrorHandling`**: Tests error conditions and edge cases

### Network Infrastructure Tests (`network_infrastructure_test.go`)

Tests the complete network infrastructure as used by the application:

- **`TestCreateNetworkInfrastructure`**: Tests the full `CreateNetworkInfrastructure()` function
- **`TestNetworkInfrastructureRetry`**: Tests idempotency of infrastructure creation
- **`TestNetworkResourceDependencies`**: Verifies resource dependencies and constraints
- **`TestNetworkResourceNaming`**: Validates resource naming conventions
- **`TestNetworkConfigurationValidation`**: Tests configuration hashing and validation

## Key Components Tested

### Virtual Networks (VNets)
- Address space configuration (`10.0.0.0/8`)
- Subnet creation and management
- Location and naming validation
- Deletion and cleanup procedures

### Network Security Groups (NSGs)
- Security rule creation and validation
- Rule priorities and directions
- Protocol and port configurations
- Association with subnets

### Subnets
- Address prefix allocation
- NSG associations
- Creation within existing VNets
- Dependency management

### Integration Points
- Resource group creation and tagging
- Table storage initialization
- Subnet ID extraction and storage
- Resource naming conventions

## Test Patterns Used

### Resource Management
- Uses `TestEnvironment` for automatic cleanup
- Tracks created resources for proper cleanup order
- Implements retry logic for long-running operations

### Azure SDK Patterns
- `BeginCreateOrUpdate()` for resource creation
- `PollUntilDone()` for operation completion
- Proper error handling and timeout management
- Resource dependency management

### Verification Strategies
- Property validation after creation
- Resource existence verification
- Dependency relationship validation
- Error condition testing

## Running the Tests

### Prerequisites
1. Azure CLI authentication (`az login`) or Service Principal credentials
2. Sufficient Azure permissions for resource creation/deletion
3. Go 1.24+ with test dependencies

### Basic Usage
```bash
# Run all network tests
./run_network_tests.sh

# Run specific test categories
./run_network_tests.sh basic        # VNet/NSG creation/deletion
./run_network_tests.sh advanced     # Subnet and integration tests
./run_network_tests.sh infrastructure # Full infrastructure tests
./run_network_tests.sh naming       # Naming and configuration tests
```

### Direct Go Test Execution
```bash
# Run all network integration tests
go test -v -tags=integration -timeout=30m ./internal/test/integration -run="TestVNet|TestNSG|TestNetwork"

# Run specific test
go test -v -tags=integration -timeout=30m ./internal/test/integration -run="TestCreateNetworkInfrastructure"
```

### Environment Configuration
```bash
export TEST_CATEGORIES="integration"
export TEST_TIMEOUT="30m"
export TEST_CLEANUP_TIMEOUT="15m"
export TEST_PARALLEL_LIMIT="1"
export TEST_RESOURCE_GROUP_PREFIX="nettest"
export TEST_LOCATION="westus2"
export SKIP_AZURE_TESTS="false"
```

## Test Duration and Resources

### Expected Durations
- **Basic tests**: 5-10 minutes each
- **Infrastructure tests**: 10-20 minutes each
- **Full test suite**: 30-45 minutes

### Azure Resources Created
Each test creates a temporary resource group containing:
- 1 Virtual Network
- 1-2 Network Security Groups
- 1-3 Subnets
- Associated security rules and configurations

All resources are automatically cleaned up after test completion.

## Error Handling and Debugging

### Common Issues
1. **Authentication**: Ensure Azure CLI is logged in or service principal is configured
2. **Permissions**: Verify permissions for resource creation in the target subscription
3. **Quotas**: Check Azure subscription limits for VNets and NSGs
4. **Timeouts**: Increase timeout values for slow Azure regions

### Debugging Tips
1. Enable verbose logging: `export VERBOSE=true`
2. Check Azure Activity Log for detailed error messages
3. Verify resource group cleanup in Azure Portal
4. Use shorter resource group prefixes to avoid naming conflicts

### Manual Cleanup
If tests fail to clean up automatically:
```bash
# List test resource groups
az group list --query "[?contains(name, 'nettest')]" -o table

# Delete specific resource group
az group delete --name "shellbox-nettest-integration-12345" --yes --no-wait
```

## Test Coverage

### Functional Coverage
- ✅ VNet creation with custom address spaces
- ✅ NSG creation with multiple security rules
- ✅ Subnet creation and deletion
- ✅ Resource association (NSG to subnet)
- ✅ Infrastructure orchestration
- ✅ Error handling and edge cases

### Integration Coverage
- ✅ End-to-end infrastructure creation
- ✅ Resource dependency management
- ✅ Naming convention validation
- ✅ Configuration hashing and tagging
- ✅ Idempotency testing
- ✅ Cleanup and resource lifecycle

### Performance Coverage
- ✅ Creation and deletion timing
- ✅ Parallel operation handling
- ✅ Retry and timeout behavior
- ✅ Large-scale resource management

## Future Enhancements

### Planned Additions
- [ ] Load testing with multiple concurrent VNets
- [ ] Cross-region networking tests
- [ ] VNet peering integration tests
- [ ] Advanced NSG rule validation
- [ ] Network monitoring and diagnostics tests

### Test Optimization
- [ ] Parallel test execution optimization
- [ ] Resource sharing across related tests
- [ ] Faster cleanup strategies
- [ ] Test result caching for CI/CD

## Contributing

When adding new network tests:

1. Follow existing naming conventions
2. Use appropriate build tags (`//go:build integration`)
3. Implement proper cleanup in test teardown
4. Add verification steps for all created resources
5. Include error condition testing
6. Update this documentation

### Test Naming Pattern
- `TestVNet*`: VNet-specific functionality
- `TestNSG*`: NSG-specific functionality
- `TestSubnet*`: Subnet-specific functionality
- `TestNetwork*`: Cross-component integration
- `Test*Infrastructure`: Full infrastructure testing