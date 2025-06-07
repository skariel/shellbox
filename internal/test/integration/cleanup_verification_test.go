package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/test"
)

// ZZZCleanupVerificationTestSuite ensures all test resources are properly cleaned up
// Named with ZZZ prefix to ensure this entire suite runs last alphabetically
type ZZZCleanupVerificationTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *ZZZCleanupVerificationTestSuite) SetupSuite() {
	suite.env = test.SetupTestEnvironment(suite.T())
}

// checkVMs lists all VMs in the resource group
func (suite *ZZZCleanupVerificationTestSuite) checkVMs(ctx context.Context, resourceGroupName string) []string {
	var resources []string
	vmPager := suite.env.Clients.ComputeClient.NewListPager(resourceGroupName, nil)
	for vmPager.More() {
		page, err := vmPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, vm := range page.Value {
			if vm.ID != nil {
				resources = append(resources, "VM: "+*vm.ID)
			}
		}
	}
	return resources
}

// checkNICs lists all NICs in the resource group
func (suite *ZZZCleanupVerificationTestSuite) checkNICs(ctx context.Context, resourceGroupName string) []string {
	var resources []string
	nicPager := suite.env.Clients.NICClient.NewListPager(resourceGroupName, nil)
	for nicPager.More() {
		page, err := nicPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, nic := range page.Value {
			if nic.ID != nil {
				resources = append(resources, "NIC: "+*nic.ID)
			}
		}
	}
	return resources
}

// checkNSGs lists all NSGs in the resource group
func (suite *ZZZCleanupVerificationTestSuite) checkNSGs(ctx context.Context, resourceGroupName string) []string {
	var resources []string
	nsgPager := suite.env.Clients.NSGClient.NewListPager(resourceGroupName, nil)
	for nsgPager.More() {
		page, err := nsgPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, nsg := range page.Value {
			if nsg.ID != nil {
				resources = append(resources, "NSG: "+*nsg.ID)
			}
		}
	}
	return resources
}

// checkVNets lists all VNets in the resource group
func (suite *ZZZCleanupVerificationTestSuite) checkVNets(ctx context.Context, resourceGroupName string) []string {
	var resources []string
	vnetPager := suite.env.Clients.NetworkClient.NewListPager(resourceGroupName, nil)
	for vnetPager.More() {
		page, err := vnetPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, vnet := range page.Value {
			if vnet.ID != nil {
				resources = append(resources, "VNet: "+*vnet.ID)
			}
		}
	}
	return resources
}

// TestResourceGroupIsEmpty tests that the shared test resource group is empty after all tests
func (suite *ZZZCleanupVerificationTestSuite) TestResourceGroupIsEmpty() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resourceGroupName := "shellbox-testing"
	var foundResources []string

	// Check all resource types
	foundResources = append(foundResources, suite.checkVMs(ctx, resourceGroupName)...)
	foundResources = append(foundResources, suite.checkNICs(ctx, resourceGroupName)...)
	foundResources = append(foundResources, suite.checkNSGs(ctx, resourceGroupName)...)
	foundResources = append(foundResources, suite.checkVNets(ctx, resourceGroupName)...)

	// Verify no resources remain
	if len(foundResources) > 0 {
		suite.T().Logf("Found %d remaining resources in %s:", len(foundResources), resourceGroupName)
		for i, resourceID := range foundResources {
			suite.T().Logf("  %d: %s", i+1, resourceID)
		}
	}

	assert.Empty(suite.T(), foundResources,
		"Resource group %s should be empty after all tests complete. "+
			"Found %d remaining resources. This indicates test cleanup failures.",
		resourceGroupName, len(foundResources))
}

// TestZZZCleanupVerificationTestSuite runs the cleanup verification suite
// Named with ZZZ prefix to ensure it runs last alphabetically after all other test suites
func TestZZZCleanupVerificationTestSuite(t *testing.T) {
	suite.Run(t, new(ZZZCleanupVerificationTestSuite))
}
