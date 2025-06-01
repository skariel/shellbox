//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestResourceCleanupIsolation(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing resource cleanup and isolation")

	// Test 1: Multiple test environments should have unique suffixes
	env1 := test.SetupTestEnvironment(t, test.CategoryIntegration)
	env2 := test.SetupTestEnvironment(t, test.CategoryIntegration)

	assert.NotEqual(t, env1.Suffix, env2.Suffix, "test environments should have unique suffixes")
	assert.NotEqual(t, env1.ResourceGroupName, env2.ResourceGroupName, "test environments should have unique resource group names")

	// Clean up both environments
	env1.Cleanup()
	env2.Cleanup()
}

func TestResourceGroupCleanupBehavior(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing resource group cleanup behavior")

	// Create a test environment
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	// Verify resource group exists
	rg, err := env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	require.NoError(t, err, "resource group should exist initially")
	assert.Equal(t, env.ResourceGroupName, *rg.Name, "resource group should have correct name")

	test.LogTestProgress(t, "creating test resources in resource group")

	// Create some test resources in the resource group
	volumeID := uuid.New().String()
	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	tags := infra.VolumeTags{
		Role:     infra.ResourceRoleVolume,
		Status:   infra.ResourceStatusFree,
		VolumeID: volumeID,
	}

	_, err = infra.CreateVolume(ctx, env.Clients, env.ResourceGroupName, volumeName, infra.DefaultVolumeSizeGB, tags)
	require.NoError(t, err, "should create test volume")

	// Verify resource exists
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "test volume should exist")
	assert.Equal(t, volumeName, *disk.Name, "test volume should have correct name")

	test.LogTestProgress(t, "performing cleanup")

	// Perform cleanup
	env.Cleanup()

	test.LogTestProgress(t, "verifying cleanup completed")

	// Verify resource group is deleted (which should delete all resources within it)
	_, err = env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	assert.Error(t, err, "resource group should be deleted after cleanup")

	// Verify individual resource is also cleaned up
	_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	assert.Error(t, err, "test volume should be deleted after cleanup")
}

func TestParallelTestIsolation(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing parallel test isolation")

	// Run multiple tests in parallel to verify they don't interfere with each other
	numParallelTests := 3
	resourceTracker := make(chan string, numParallelTests)

	for i := 0; i < numParallelTests; i++ {
		testIndex := i
		t.Run(fmt.Sprintf("ParallelTest_%d", testIndex), func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			env := test.SetupTestEnvironment(t, test.CategoryIntegration)
			defer env.Cleanup()

			test.LogTestProgress(t, "parallel test starting", "testIndex", testIndex, "suffix", env.Suffix)

			// Create a unique resource in each test
			volumeID := uuid.New().String()
			namer := env.GetResourceNamer()
			volumeName := namer.VolumePoolDiskName(volumeID)

			tags := infra.VolumeTags{
				Role:     infra.ResourceRoleVolume,
				Status:   infra.ResourceStatusFree,
				VolumeID: volumeID,
			}

			_, err := infra.CreateVolume(ctx, env.Clients, env.ResourceGroupName, volumeName, infra.DefaultVolumeSizeGB, tags)
			require.NoError(t, err, "should create volume in parallel test %d", testIndex)

			// Verify resource was created
			disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
			require.NoError(t, err, "should retrieve volume in parallel test %d", testIndex)
			assert.Equal(t, volumeName, *disk.Name, "volume should have correct name in parallel test %d", testIndex)

			// Record that this test created a resource
			resourceTracker <- fmt.Sprintf("test-%d:%s", testIndex, volumeName)

			test.LogTestProgress(t, "parallel test completed", "testIndex", testIndex, "volumeName", volumeName)
		})
	}

	// Wait for all parallel tests to complete and verify they all created resources
	close(resourceTracker)
	createdResources := make([]string, 0, numParallelTests)
	for resource := range resourceTracker {
		createdResources = append(createdResources, resource)
	}

	assert.Len(t, createdResources, numParallelTests, "all parallel tests should have created resources")

	// Verify all resource names are unique
	resourceNames := make(map[string]bool)
	for _, resource := range createdResources {
		assert.False(t, resourceNames[resource], "resource names should be unique across parallel tests")
		resourceNames[resource] = true
	}

	test.LogTestProgress(t, "parallel test isolation verified", "uniqueResources", len(resourceNames))
}

func TestCleanupTimeout(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing cleanup timeout behavior")

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	// Verify the test environment has a cleanup timeout configured
	assert.NotZero(t, env.Config.CleanupTimeout, "test environment should have cleanup timeout configured")
	assert.Greater(t, env.Config.CleanupTimeout, 1*time.Minute, "cleanup timeout should be reasonable")

	test.LogTestProgress(t, "cleanup timeout configured", "timeout", env.Config.CleanupTimeout)

	// Clean up the environment
	env.Cleanup()
}

func TestResourceNamingUniqueness(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing resource naming uniqueness")

	// Create multiple test environments
	environments := make([]*test.TestEnvironment, 5)
	for i := 0; i < 5; i++ {
		environments[i] = test.SetupTestEnvironment(t, test.CategoryIntegration)
	}

	// Verify all have unique names
	suffixes := make(map[string]bool)
	resourceGroupNames := make(map[string]bool)
	namers := make([]*infra.ResourceNamer, len(environments))

	for i, env := range environments {
		// Check suffix uniqueness
		assert.False(t, suffixes[env.Suffix], "suffix %s should be unique", env.Suffix)
		suffixes[env.Suffix] = true

		// Check resource group name uniqueness
		assert.False(t, resourceGroupNames[env.ResourceGroupName], "resource group name %s should be unique", env.ResourceGroupName)
		resourceGroupNames[env.ResourceGroupName] = true

		// Create namer for resource name testing
		namers[i] = env.GetResourceNamer()
	}

	// Test that resource names are unique across environments
	vmNames := make(map[string]bool)
	volumeNames := make(map[string]bool)
	nsgNames := make(map[string]bool)

	for i, namer := range namers {
		testInstanceID := uuid.New().String()
		testVolumeID := uuid.New().String()

		vmName := namer.BoxVMName(testInstanceID)
		volumeName := namer.VolumePoolDiskName(testVolumeID)
		nsgName := namer.BastionNSGName()

		assert.False(t, vmNames[vmName], "VM name %s should be unique across environments", vmName)
		vmNames[vmName] = true

		assert.False(t, volumeNames[volumeName], "volume name %s should be unique across environments", volumeName)
		volumeNames[volumeName] = true

		assert.False(t, nsgNames[nsgName], "NSG name %s should be unique across environments", nsgName)
		nsgNames[nsgName] = true

		test.LogTestProgress(t, "verified naming uniqueness", "env", i, "suffix", environments[i].Suffix)
	}

	// Clean up all environments
	for _, env := range environments {
		env.Cleanup()
	}

	test.LogTestProgress(t, "resource naming uniqueness verified", "environments", len(environments))
}

func TestCleanupErrorHandling(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing cleanup error handling")

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	// Test cleanup with valid environment
	originalRGName := env.ResourceGroupName

	// Cleanup should work normally
	env.Cleanup()

	// Test cleanup with already-cleaned environment (should not panic or error)
	env.Cleanup() // Second cleanup should be safe

	// Test cleanup with invalid resource group name
	env.ResourceGroupName = "non-existent-resource-group"
	env.Cleanup() // Should handle gracefully

	// Restore original name for logging
	env.ResourceGroupName = originalRGName

	test.LogTestProgress(t, "cleanup error handling verified")
}

func TestMinimalEnvironmentBehavior(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing minimal environment behavior")

	// Test minimal environment (no Azure resources)
	minimalEnv := test.SetupMinimalTestEnvironment(t)

	// Verify minimal environment properties
	assert.NotEmpty(t, minimalEnv.Suffix, "minimal environment should have suffix")
	assert.Nil(t, minimalEnv.Clients, "minimal environment should not have Azure clients")
	assert.Empty(t, minimalEnv.ResourceGroupName, "minimal environment should not have resource group name")

	// Cleanup should be safe for minimal environment
	minimalEnv.Cleanup() // Should not panic

	test.LogTestProgress(t, "minimal environment behavior verified")
}

func TestResourceTrackingBehavior(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing resource tracking behavior")

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	// Verify initial state
	initialResourceCount := len(env.CreatedResources)
	assert.Greater(t, initialResourceCount, 0, "test environment should track initial resources")

	// Track additional resources
	testResourceName := "test-resource-" + uuid.New().String()
	env.TrackResource(testResourceName)

	// Verify resource was tracked
	assert.Len(t, env.CreatedResources, initialResourceCount+1, "should track additional resource")
	assert.Contains(t, env.CreatedResources, testResourceName, "should contain tracked resource")

	// Track multiple resources
	additionalResources := []string{
		"test-resource-2-" + uuid.New().String(),
		"test-resource-3-" + uuid.New().String(),
	}

	for _, resource := range additionalResources {
		env.TrackResource(resource)
	}

	expectedTotal := initialResourceCount + 1 + len(additionalResources)
	assert.Len(t, env.CreatedResources, expectedTotal, "should track all resources")

	for _, resource := range additionalResources {
		assert.Contains(t, env.CreatedResources, resource, "should contain tracked resource %s", resource)
	}

	test.LogTestProgress(t, "resource tracking verified", "totalTracked", len(env.CreatedResources))
}

func TestUniqueResourceNaming(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing unique resource naming patterns")

	env := test.SetupMinimalTestEnvironment(t)

	// Test that GetUniqueResourceName generates unique names
	names := make(map[string]bool)
	prefix := "test-prefix"

	for i := 0; i < 10; i++ {
		uniqueName := env.GetUniqueResourceName(prefix)
		assert.False(t, names[uniqueName], "generated name %s should be unique", uniqueName)
		assert.Contains(t, uniqueName, prefix, "generated name should contain prefix")
		assert.Contains(t, uniqueName, env.Suffix, "generated name should contain suffix")
		names[uniqueName] = true
	}

	assert.Len(t, names, 10, "should generate 10 unique names")

	test.LogTestProgress(t, "unique resource naming verified", "uniqueNames", len(names))
}

func TestConfigurationCategoryHandling(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing configuration category handling")

	// Test that category is properly detected and handled
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	// Verify test configuration
	assert.NotNil(t, env.Config, "test environment should have configuration")
	assert.True(t, env.Config.ShouldRunCategory(test.CategoryIntegration), "should allow integration category")

	// Verify category-specific behavior
	assert.NotEmpty(t, env.Suffix, "integration test should have suffix")
	assert.Contains(t, env.Suffix, "integration", "integration test suffix should contain category")

	test.LogTestProgress(t, "configuration category handling verified", "category", test.CategoryIntegration)
}
