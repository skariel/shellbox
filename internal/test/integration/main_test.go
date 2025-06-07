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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// TestMain controls the test execution order
func TestMain(m *testing.M) {
	// Run initial resource group verification
	if err := verifyResourceGroupIsEmpty("Initial verification - before tests"); err != nil {
		log.Printf("WARNING: %v", err)
		// Don't fail here - just warn that resources exist before tests start
	}

	// Create shared storage account for all tests
	log.Println("Creating shared storage account for tests...")
	if err := createSharedStorageAccount(); err != nil {
		log.Printf("ERROR: Failed to create shared storage account: %v", err)
		os.Exit(1)
	}

	// Run all tests
	exitCode := m.Run()

	// Clean up shared storage account
	log.Println("Cleaning up shared storage account...")
	if err := deleteSharedStorageAccount(); err != nil {
		log.Printf("ERROR: Failed to delete shared storage account: %v", err)
		// Don't change exit code for cleanup failure
	}

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

// createSharedStorageAccount creates the shared storage account for all tests
func createSharedStorageAccount() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Create minimal clients for storage account creation
	clients := infra.NewAzureClients("test-setup", true)
	clients.ResourceGroupName = "shellbox-testing"

	// Get the shared storage account name
	namer := infra.NewResourceNamer("test")
	storageAccountName := namer.SharedStorageAccountName()

	log.Printf("Creating shared storage account: %s", storageAccountName)

	// Create storage client
	storageClient, err := armstorage.NewAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	// Check if storage account already exists
	_, err = storageClient.GetProperties(ctx, clients.ResourceGroupName, storageAccountName, nil)
	if err == nil {
		// Storage account already exists, delete it first to ensure clean state
		log.Printf("Storage account %s already exists, deleting it first...", storageAccountName)
		_, err = storageClient.Delete(ctx, clients.ResourceGroupName, storageAccountName, nil)
		if err != nil {
			log.Printf("Warning: Failed to delete existing storage account: %v", err)
		}
		// Wait for deletion to complete
		time.Sleep(30 * time.Second)
	}

	// Create the storage account
	poller, err := storageClient.BeginCreate(ctx, clients.ResourceGroupName, storageAccountName, armstorage.AccountCreateParameters{
		SKU: &armstorage.SKU{
			Name: to.Ptr(armstorage.SKUNameStandardLRS),
		},
		Kind:     to.Ptr(armstorage.KindStorageV2),
		Location: to.Ptr(infra.Location),
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AllowBlobPublicAccess: to.Ptr(false),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage account: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for storage account creation: %w", err)
	}

	log.Printf("Successfully created shared storage account: %s", storageAccountName)
	return nil
}

// deleteSharedStorageAccount deletes the shared storage account
func deleteSharedStorageAccount() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Create minimal clients for storage account deletion
	clients := infra.NewAzureClients("test-cleanup", true)
	clients.ResourceGroupName = "shellbox-testing"

	// Get the shared storage account name
	namer := infra.NewResourceNamer("test")
	storageAccountName := namer.SharedStorageAccountName()

	log.Printf("Deleting shared storage account: %s", storageAccountName)

	// Create storage client
	storageClient, err := armstorage.NewAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	// Delete the storage account
	_, err = storageClient.Delete(ctx, clients.ResourceGroupName, storageAccountName, nil)
	if err != nil {
		return fmt.Errorf("failed to delete storage account: %w", err)
	}

	log.Printf("Successfully deleted shared storage account: %s", storageAccountName)
	return nil
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
