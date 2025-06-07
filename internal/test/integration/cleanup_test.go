package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestResourceCleanupIsolation(t *testing.T) {
	t.Parallel()

	test.LogTestProgress(t, "testing resource cleanup and isolation")

	// Test 1: Multiple test environments should have unique suffixes
	env1 := test.SetupTestEnvironment(t)
	env2 := test.SetupTestEnvironment(t)

	if env1.Suffix == env2.Suffix {
		t.Error("test environments should have unique suffixes")
	}
	// Resource groups are shared, but suffixes should be unique
	if env1.ResourceGroupName != env2.ResourceGroupName {
		t.Error("test environments should use the same shared resource group")
	}
	if env1.ResourceGroupName != "shellbox-testing" {
		t.Errorf("should use shared resource group name: got %s, want shellbox-testing", env1.ResourceGroupName)
	}

	// Clean up both environments
	env1.Cleanup()
	env2.Cleanup()
}

func TestResourceGroupCleanupBehavior(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing resource group cleanup behavior")

	// Create a test environment
	env := test.SetupTestEnvironment(t)

	// Verify resource group exists
	rg, err := env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	if err != nil {
		t.Fatalf("resource group should exist initially: %v", err)
	}
	if *rg.Name != env.ResourceGroupName {
		t.Errorf("resource group should have correct name: got %s, want %s", *rg.Name, env.ResourceGroupName)
	}

	test.LogTestProgress(t, "creating test resources in resource group")

	// Create some test resources in the resource group
	config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	if err != nil {
		t.Fatalf("should create test volume: %v", err)
	}

	// Generate volume name from returned volume ID
	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Verify resource exists
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("test volume should exist: %v", err)
	}
	if *disk.Name != volumeName {
		t.Errorf("test volume should have correct name: got %s, want %s", *disk.Name, volumeName)
	}

	test.LogTestProgress(t, "performing cleanup")

	// Perform suffix-based cleanup to delete resources created by this test
	err = env.CleanupResourcesBySuffix(ctx)
	if err != nil {
		t.Fatalf("should clean up resources by suffix: %v", err)
	}

	// Perform standard cleanup
	env.Cleanup()

	test.LogTestProgress(t, "verifying cleanup completed")

	// Shared resource group should still exist (not deleted)
	_, err = env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	if err != nil {
		t.Errorf("shared resource group should still exist after cleanup: %v", err)
	}

	// Individual test resource should be cleaned up
	_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err == nil {
		t.Error("test volume should be deleted after cleanup")
	}
}

func TestCleanupTimeout(t *testing.T) {
	t.Parallel()

	test.LogTestProgress(t, "testing cleanup timeout behavior")

	env := test.SetupTestEnvironment(t)

	// Verify the test environment has a cleanup timeout configured
	if env.Config.CleanupTimeout == 0 {
		t.Error("test environment should have cleanup timeout configured")
	}
	if env.Config.CleanupTimeout <= 1*time.Minute {
		t.Error("cleanup timeout should be reasonable")
	}

	test.LogTestProgress(t, "cleanup timeout configured", "timeout", env.Config.CleanupTimeout)

	// Clean up the environment
	env.Cleanup()
}

func TestComprehensiveResourceNaming(t *testing.T) {
	t.Parallel()

	test.LogTestProgress(t, "testing comprehensive resource naming patterns and uniqueness")

	// Create multiple test environments
	environments := make([]*test.Environment, 5)
	for i := 0; i < 5; i++ {
		environments[i] = test.SetupTestEnvironment(t)
	}

	// Verify all have unique names
	suffixes := make(map[string]bool)
	resourceGroupNames := make(map[string]bool)
	namers := make([]*infra.ResourceNamer, len(environments))

	for i, env := range environments {
		// Check suffix uniqueness
		if suffixes[env.Suffix] {
			t.Errorf("suffix %s should be unique", env.Suffix)
		}
		suffixes[env.Suffix] = true

		// All environments should use the same shared resource group
		if env.ResourceGroupName != "shellbox-testing" {
			t.Errorf("all environments should use shared resource group: got %s, want shellbox-testing", env.ResourceGroupName)
		}
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

		if vmNames[vmName] {
			t.Errorf("VM name %s should be unique across environments", vmName)
		}
		vmNames[vmName] = true

		if volumeNames[volumeName] {
			t.Errorf("volume name %s should be unique across environments", volumeName)
		}
		volumeNames[volumeName] = true

		if nsgNames[nsgName] {
			t.Errorf("NSG name %s should be unique across environments", nsgName)
		}
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
	t.Parallel()

	test.LogTestProgress(t, "testing cleanup error handling")

	env := test.SetupTestEnvironment(t)

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
	t.Parallel()

	test.LogTestProgress(t, "testing minimal environment behavior")

	// Test minimal environment (no Azure resources)
	minimalEnv := test.SetupMinimalTestEnvironment(t)

	// Verify minimal environment properties
	if minimalEnv.Suffix == "" {
		t.Error("minimal environment should have suffix")
	}
	if minimalEnv.Clients != nil {
		t.Error("minimal environment should not have Azure clients")
	}
	if minimalEnv.ResourceGroupName != "" {
		t.Error("minimal environment should not have resource group name")
	}

	// Cleanup should be safe for minimal environment
	minimalEnv.Cleanup() // Should not panic

	test.LogTestProgress(t, "minimal environment behavior verified")
}

func TestResourceTrackingBehavior(t *testing.T) {
	t.Parallel()

	test.LogTestProgress(t, "testing resource tracking behavior")

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	// Verify initial state (environment starts with empty tracking)
	initialResourceCount := len(env.CreatedResources)
	if initialResourceCount != 0 {
		t.Errorf("test environment should start with no tracked resources, got %d", initialResourceCount)
	}

	// Track additional resources
	testResourceName := "test-resource-" + uuid.New().String()
	env.TrackResource(testResourceName)

	// Verify resource was tracked
	if len(env.CreatedResources) != initialResourceCount+1 {
		t.Errorf("should track additional resource: got %d, want %d", len(env.CreatedResources), initialResourceCount+1)
	}
	found := false
	for _, resource := range env.CreatedResources {
		if resource == testResourceName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("should contain tracked resource %s", testResourceName)
	}

	// Track multiple resources
	additionalResources := []string{
		"test-resource-2-" + uuid.New().String(),
		"test-resource-3-" + uuid.New().String(),
	}

	for _, resource := range additionalResources {
		env.TrackResource(resource)
	}

	expectedTotal := initialResourceCount + 1 + len(additionalResources)
	if len(env.CreatedResources) != expectedTotal {
		t.Errorf("should track all resources: got %d, want %d", len(env.CreatedResources), expectedTotal)
	}

	for _, expectedResource := range additionalResources {
		found := false
		for _, resource := range env.CreatedResources {
			if resource == expectedResource {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("should contain tracked resource %s", expectedResource)
		}
	}

	test.LogTestProgress(t, "resource tracking verified", "totalTracked", len(env.CreatedResources))
}

func TestUniqueResourceNaming(t *testing.T) {
	t.Parallel()

	test.LogTestProgress(t, "testing unique resource naming patterns")

	env := test.SetupMinimalTestEnvironment(t)

	// Test that GetUniqueResourceName generates unique names
	names := make(map[string]bool)
	prefix := "test-prefix"

	for i := 0; i < 10; i++ {
		uniqueName := env.GetUniqueResourceName(prefix)
		if names[uniqueName] {
			t.Errorf("generated name %s should be unique", uniqueName)
		}
		if !strings.Contains(uniqueName, prefix) {
			t.Errorf("generated name should contain prefix: %s", uniqueName)
		}
		if !strings.Contains(uniqueName, env.Suffix) {
			t.Errorf("generated name should contain suffix: %s", uniqueName)
		}
		names[uniqueName] = true
	}

	if len(names) != 10 {
		t.Errorf("should generate 10 unique names, got %d", len(names))
	}

	test.LogTestProgress(t, "unique resource naming verified", "uniqueNames", len(names))
}
