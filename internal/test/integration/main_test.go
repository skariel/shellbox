package integration

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// TestMain controls the test execution order
func TestMain(m *testing.M) {
	// Run initial resource group verification
	if err := verifyResourceGroupIsEmpty("Initial verification - before tests"); err != nil {
		log.Printf("WARNING: %v", err)
		// Don't fail here - just warn that resources exist before tests start
	}

	// Run all tests
	exitCode := m.Run()

	// Run final resource group verification
	if err := verifyResourceGroupIsEmpty("Final verification - after tests"); err != nil {
		log.Printf("ERROR: %v", err)
		// Set exit code to failure if resources remain after tests
		if exitCode == 0 {
			exitCode = 1
		}
	}

	os.Exit(exitCode)
}

// verifyResourceGroupIsEmpty checks that the test resource group is empty
func verifyResourceGroupIsEmpty(phase string) error {
	// Create a minimal environment just for resource checking
	// We don't use the full test setup to avoid side effects
	clients := infra.NewAzureClients("verify-resources", true)
	clients.ResourceGroupName = "shellbox-testing"
	env := &test.Environment{
		Clients:           clients,
		ResourceGroupName: "shellbox-testing",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	resourceGroupName := "shellbox-testing"
	var foundResources []string

	// Check all resource types
	foundResources = append(foundResources, checkVMs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkNICs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkNSGs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkVNets(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkPublicIPs(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkDisks(ctx, env, resourceGroupName)...)
	foundResources = append(foundResources, checkStorageAccounts(ctx, env, resourceGroupName)...)

	// Report results
	if len(foundResources) > 0 {
		errMsg := fmt.Sprintf("%s: Resource group %s contains %d resources:\n", phase, resourceGroupName, len(foundResources))
		for i, resourceID := range foundResources {
			errMsg += fmt.Sprintf("  %d: %s\n", i+1, resourceID)
		}
		return fmt.Errorf("%s", errMsg)
	}

	log.Printf("✅ %s: Resource group %s is clean - no resources found", phase, resourceGroupName)
	return nil
}

// checkPublicIPs lists all Public IPs in the resource group
func checkPublicIPs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	pipPager := env.Clients.PublicIPClient.NewListPager(resourceGroupName, nil)
	for pipPager.More() {
		page, err := pipPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, pip := range page.Value {
			if pip.ID != nil {
				resources = append(resources, "PublicIP: "+*pip.ID)
			}
		}
	}
	return resources
}

// checkDisks lists all Disks in the resource group
func checkDisks(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	diskPager := env.Clients.DisksClient.NewListByResourceGroupPager(resourceGroupName, nil)
	for diskPager.More() {
		page, err := diskPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, disk := range page.Value {
			if disk.ID != nil {
				resources = append(resources, "Disk: "+*disk.ID)
			}
		}
	}
	return resources
}

// checkStorageAccounts lists all Storage Accounts in the resource group
func checkStorageAccounts(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	storagePager := env.Clients.StorageClient.NewListByResourceGroupPager(resourceGroupName, nil)
	for storagePager.More() {
		page, err := storagePager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, account := range page.Value {
			if account.ID != nil {
				resources = append(resources, "StorageAccount: "+*account.ID)
			}
		}
	}
	return resources
}
