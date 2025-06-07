package integration

import (
	"context"
	"testing"
	"time"

	"shellbox/internal/test"
)

// checkVMs lists all VMs in the resource group
func checkVMs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	vmPager := env.Clients.ComputeClient.NewListPager(resourceGroupName, nil)
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
func checkNICs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	nicPager := env.Clients.NICClient.NewListPager(resourceGroupName, nil)
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
func checkNSGs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	nsgPager := env.Clients.NSGClient.NewListPager(resourceGroupName, nil)
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
func checkVNets(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	vnetPager := env.Clients.NetworkClient.NewListPager(resourceGroupName, nil)
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

// TestZZZResourceGroupIsEmpty tests that the shared test resource group is empty after all tests
// Named with ZZZ prefix to ensure this test runs last alphabetically after all other tests
// This test does NOT call t.Parallel() so it runs in the main test goroutine after all parallel tests complete
func TestZZZResourceGroupIsEmpty(t *testing.T) {
	// Set up test environment
	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resourceGroupName := "shellbox-testing"
	var foundResources []string

	// Check all resource types
	foundResources = append(foundResources, checkVMs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkNICs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkNSGs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkVNets(ctx, env, resourceGroupName)...)

	// Verify no resources remain
	if len(foundResources) > 0 {
		t.Logf("Found %d remaining resources in %s:", len(foundResources), resourceGroupName)
		for i, resourceID := range foundResources {
			t.Logf("  %d: %s", i+1, resourceID)
		}
		t.Fatalf("Resource group %s should be empty after all tests complete. "+
			"Found %d remaining resources. This indicates test cleanup failures.",
			resourceGroupName, len(foundResources))
	}

	t.Logf("✅ Resource group %s is clean - no resources remain after tests", resourceGroupName)
}
