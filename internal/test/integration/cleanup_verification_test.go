//go:build integration || e2e

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestResourceGroupIsEmpty tests that the shared test resource group is empty after all tests
func (suite *ZZZCleanupVerificationTestSuite) TestResourceGroupIsEmpty() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// List all resources in the shared test resource group
	resourceGroupName := "shellbox-testing"

	resources, err := suite.env.Clients.ResourceClient.ListByResourceGroup(ctx, resourceGroupName, nil)
	require.NoError(suite.T(), err, "Failed to list resources in %s", resourceGroupName)

	// Collect all resource information for debugging
	var foundResources []string
	for resources.More() {
		page, err := resources.NextPage(ctx)
		require.NoError(suite.T(), err, "Failed to get next page of resources")

		for _, resource := range page.Value {
			if resource.ID != nil {
				foundResources = append(foundResources, *resource.ID)
			}
		}
	}

	// Verify no resources remain
	if len(foundResources) > 0 {
		suite.T().Logf("Found %d remaining resources in %s:", len(foundResources), resourceGroupName)
		for i, resourceID := range foundResources {
			suite.T().Logf("  %d: %s", i+1, resourceID)
		}
	}

	assert.Empty(suite.T(), foundResources,
		"Resource group %s should be empty after all tests complete. "+
			"Found %d remaining resources. This indicates test cleanup failures. "+
			"Run 'make clean-test' to manually clean up resources.",
		resourceGroupName, len(foundResources))
}

// TestZZZCleanupVerificationTestSuite runs the cleanup verification suite
// Named with ZZZ prefix to ensure it runs last alphabetically after all other test suites
func TestZZZCleanupVerificationTestSuite(t *testing.T) {
	suite.Run(t, new(ZZZCleanupVerificationTestSuite))
}
