package test

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"

	"shellbox/internal/infra"
)

// Environment manages Azure resources for testing
type Environment struct {
	Config            *Config
	Clients           *infra.AzureClients
	Suffix            string
	ResourceGroupName string
	CreatedResources  []string
	t                 *testing.T
	startTime         time.Time
	cleanedUp         bool // Prevent double cleanup
}

// SetupTestEnvironment creates a new test environment with Azure resources
func SetupTestEnvironment(t *testing.T) *Environment {
	t.Helper()

	// Initialize logger with debug level for tests
	setupTestLogger()

	config := LoadConfig()

	// Generate unique suffix for this test's resources
	rndNum, _ := rand.Int(rand.Reader, big.NewInt(100000))
	suffix := fmt.Sprintf("%s-test-%d", config.ResourceGroupPrefix, rndNum.Int64())

	// Use shared resource group for all tests
	sharedRGName := "shellbox-testing"

	// Create Azure clients with the shared resource group
	clients := infra.NewAzureClients(suffix, config.UseAzureCLI)
	// Override the resource group name to use shared one
	clients.ResourceGroupName = sharedRGName

	env := &Environment{
		Config:            config,
		Clients:           clients,
		Suffix:            suffix,
		ResourceGroupName: sharedRGName,
		CreatedResources:  []string{},
		t:                 t,
		startTime:         time.Now(),
		cleanedUp:         false,
	}

	// Create the test resource group
	if err := env.createTestResourceGroup(); err != nil {
		t.Fatalf("Failed to create test resource group: %v", err)
	}

	// Set up cleanup
	t.Cleanup(func() {
		env.Cleanup()
	})

	slog.Debug("Test environment ready",
		"suffix", suffix,
		"sharedResourceGroup", env.ResourceGroupName,
		"useAzureCLI", config.UseAzureCLI)

	return env
}

// SetupMinimalTestEnvironment creates a lightweight test environment for unit tests
func SetupMinimalTestEnvironment(t *testing.T) *Environment {
	t.Helper()

	// Initialize logger with debug level for tests
	setupTestLogger()

	config := LoadConfig()

	// Generate unique suffix but don't create Azure resources
	rndNum, _ := rand.Int(rand.Reader, big.NewInt(100000))
	suffix := fmt.Sprintf("%s-unit-%d", config.ResourceGroupPrefix, rndNum.Int64())

	env := &Environment{
		Config:    config,
		Suffix:    suffix,
		t:         t,
		startTime: time.Now(),
	}

	return env
}

// createTestResourceGroup creates or ensures the shared resource group exists (idempotent)
func (te *Environment) createTestResourceGroup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Check if resource group already exists
	_, err := te.Clients.ResourceClient.Get(ctx, te.ResourceGroupName, nil)
	if err == nil {
		// Resource group already exists, we're good
		slog.Debug("Shared resource group already exists", "name", te.ResourceGroupName)
		return nil
	}

	// Resource group doesn't exist, create it
	slog.Debug("Creating shared resource group", "name", te.ResourceGroupName)

	_, err = te.Clients.ResourceClient.CreateOrUpdate(ctx, te.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(te.Config.Location),
		Tags: map[string]*string{
			"purpose":     to.Ptr("integration-tests"),
			"created":     to.Ptr(time.Now().Format(time.RFC3339)),
			"description": to.Ptr("Shared resource group for all integration tests"),
			"persistent":  to.Ptr("true"),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create shared resource group: %w", err)
	}

	slog.Debug("Successfully created shared resource group", "name", te.ResourceGroupName)
	// Don't add the RG to CreatedResources since we never want to delete it
	return nil
}

// TrackResource adds a resource to the cleanup list
func (te *Environment) TrackResource(resourceName string) {
	te.CreatedResources = append(te.CreatedResources, resourceName)
}

// Cleanup removes all test resources
func (te *Environment) Cleanup() {
	if te.Clients == nil || te.cleanedUp {
		return // Nothing to clean up for minimal environments or already cleaned up
	}
	te.cleanedUp = true // Mark as cleaned up to prevent double cleanup

	elapsed := time.Since(te.startTime)
	slog.Debug("Starting test cleanup",
		"suffix", te.Suffix,
		"resources", len(te.CreatedResources),
		"elapsed", elapsed)

	// Clean up all Azure resources by suffix (VMs, NICs, disks, storage accounts, etc.)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := te.CleanupResourcesBySuffix(ctx); err != nil {
		slog.Warn("Failed to cleanup resources by suffix", "suffix", te.Suffix, "error", err)
	} else {
		slog.Debug("Successfully cleaned up Azure resources by suffix", "suffix", te.Suffix)
	}

	// Clean up table storage (if tables were created with this suffix)
	if te.Clients.TableClient != nil {
		if err := infra.CleanupTestTables(ctx, te.Clients, te.Suffix); err != nil {
			slog.Warn("Failed to cleanup test tables", "suffix", te.Suffix, "error", err)
		} else {
			slog.Debug("Successfully cleaned up test tables", "suffix", te.Suffix)
		}
	}

	// Log tracked resources for reference
	if len(te.CreatedResources) > 0 {
		slog.Debug("Tracked resources for cleanup",
			"count", len(te.CreatedResources),
			"suffix", te.Suffix)

		for _, resourceName := range te.CreatedResources {
			slog.Debug("Resource tracked for cleanup", "resource", resourceName)
		}
	}
}

// WaitForResource waits for a resource to be ready
func (te *Environment) WaitForResource(ctx context.Context, resourceName string, checkFunc func() (bool, error)) error {
	return infra.RetryOperation(ctx, func(_ context.Context) error {
		ready, err := checkFunc()
		if err != nil {
			return err
		}
		if !ready {
			return fmt.Errorf("resource %s not ready yet", resourceName)
		}
		return nil
	}, 5*time.Minute, 10*time.Second, fmt.Sprintf("wait for resource %s", resourceName))
}

// GetResourceNamer returns a resource namer for this test environment
func (te *Environment) GetResourceNamer() *infra.ResourceNamer {
	return infra.NewResourceNamer(te.Suffix)
}

// CleanupResourcesBySuffix deletes all resources in the resource group that match this test's suffix
func (te *Environment) CleanupResourcesBySuffix(ctx context.Context) error {
	if te.Clients == nil {
		return nil // Nothing to clean up for minimal environments
	}

	slog.Debug("Cleaning up resources by suffix", "suffix", te.Suffix, "resourceGroup", te.ResourceGroupName)

	// Clean up resources in dependency order
	te.cleanupVMs(ctx)
	te.cleanupNICs(ctx)
	te.cleanupPublicIPs(ctx)
	te.cleanupVNets(ctx) // Delete VNets before NSGs to remove subnet associations
	te.cleanupNSGs(ctx)
	te.cleanupDisks(ctx)
	te.cleanupStorageAccounts(ctx)

	slog.Debug("Completed resource cleanup by suffix", "suffix", te.Suffix)
	return nil
}

// cleanupVMs deletes VMs with matching suffix
func (te *Environment) cleanupVMs(ctx context.Context) {
	vmPager := te.Clients.ComputeClient.NewListPager(te.ResourceGroupName, nil)
	for vmPager.More() {
		page, err := vmPager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list VMs for cleanup", "error", err)
			break
		}

		for _, vm := range page.Value {
			if vm.Name != nil && contains(*vm.Name, te.Suffix) {
				slog.Debug("Deleting VM", "name", *vm.Name, "suffix", te.Suffix)
				if err := infra.DeleteInstance(ctx, te.Clients, te.ResourceGroupName, *vm.Name); err != nil {
					slog.Warn("Failed to delete VM", "name", *vm.Name, "error", err)
				}
			}
		}
	}
}

// cleanupNICs deletes NICs with matching suffix
func (te *Environment) cleanupNICs(ctx context.Context) {
	nicPager := te.Clients.NICClient.NewListPager(te.ResourceGroupName, nil)
	for nicPager.More() {
		page, err := nicPager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list NICs for cleanup", "error", err)
			break
		}

		for _, nic := range page.Value {
			if nic.Name != nil && contains(*nic.Name, te.Suffix) {
				slog.Debug("Deleting NIC", "name", *nic.Name, "suffix", te.Suffix)
				infra.DeleteNIC(ctx, te.Clients, te.ResourceGroupName, *nic.Name, "")
			}
		}
	}
}

// cleanupPublicIPs deletes Public IPs with matching suffix
func (te *Environment) cleanupPublicIPs(ctx context.Context) {
	pipPager := te.Clients.PublicIPClient.NewListPager(te.ResourceGroupName, nil)
	for pipPager.More() {
		page, err := pipPager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list Public IPs for cleanup", "error", err)
			break
		}

		for _, pip := range page.Value {
			if pip.Name != nil && contains(*pip.Name, te.Suffix) {
				slog.Debug("Deleting Public IP", "name", *pip.Name, "suffix", te.Suffix)
				poller, err := te.Clients.PublicIPClient.BeginDelete(ctx, te.ResourceGroupName, *pip.Name, nil)
				if err != nil {
					slog.Warn("Failed to start Public IP deletion", "name", *pip.Name, "error", err)
					continue
				}
				if _, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions); err != nil {
					slog.Warn("Failed to delete Public IP", "name", *pip.Name, "error", err)
				}
			}
		}
	}
}

// cleanupVNets deletes VNets with matching suffix
func (te *Environment) cleanupVNets(ctx context.Context) {
	vnetPager := te.Clients.NetworkClient.NewListPager(te.ResourceGroupName, nil)
	for vnetPager.More() {
		page, err := vnetPager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list VNets for cleanup", "error", err)
			break
		}

		for _, vnet := range page.Value {
			if vnet.Name != nil && contains(*vnet.Name, te.Suffix) {
				slog.Debug("Deleting VNet", "name", *vnet.Name, "suffix", te.Suffix)
				poller, err := te.Clients.NetworkClient.BeginDelete(ctx, te.ResourceGroupName, *vnet.Name, nil)
				if err != nil {
					slog.Warn("Failed to start VNet deletion", "name", *vnet.Name, "error", err)
					continue
				}
				if _, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions); err != nil {
					slog.Warn("Failed to delete VNet", "name", *vnet.Name, "error", err)
				} else {
					slog.Info("Successfully deleted VNet", "name", *vnet.Name)
				}
			}
		}
	}
}

// cleanupNSGs deletes NSGs with matching suffix
func (te *Environment) cleanupNSGs(ctx context.Context) {
	nsgPager := te.Clients.NSGClient.NewListPager(te.ResourceGroupName, nil)
	for nsgPager.More() {
		page, err := nsgPager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list NSGs for cleanup", "error", err)
			break
		}

		for _, nsg := range page.Value {
			if nsg.Name != nil && contains(*nsg.Name, te.Suffix) {
				slog.Debug("Deleting NSG", "name", *nsg.Name, "suffix", te.Suffix)
				infra.DeleteNSG(ctx, te.Clients, te.ResourceGroupName, *nsg.Name)
			}
		}
	}
}

// cleanupDisks deletes disks with matching suffix
func (te *Environment) cleanupDisks(ctx context.Context) {
	diskPager := te.Clients.DisksClient.NewListByResourceGroupPager(te.ResourceGroupName, nil)
	for diskPager.More() {
		page, err := diskPager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list disks for cleanup", "error", err)
			break
		}

		for _, disk := range page.Value {
			if disk.Name != nil && contains(*disk.Name, te.Suffix) {
				slog.Debug("Deleting disk", "name", *disk.Name, "suffix", te.Suffix)
				if err := infra.DeleteVolume(ctx, te.Clients, te.ResourceGroupName, *disk.Name); err != nil {
					slog.Warn("Failed to delete disk", "name", *disk.Name, "error", err)
				}
			}
		}
	}
}

// cleanupStorageAccounts deletes storage accounts with matching suffix
func (te *Environment) cleanupStorageAccounts(ctx context.Context) {
	storagePager := te.Clients.StorageClient.NewListByResourceGroupPager(te.ResourceGroupName, nil)
	for storagePager.More() {
		page, err := storagePager.NextPage(ctx)
		if err != nil {
			slog.Warn("Failed to list storage accounts for cleanup", "error", err)
			break
		}

		for _, storage := range page.Value {
			if storage.Name != nil && contains(*storage.Name, te.Suffix) {
				slog.Debug("Deleting storage account", "name", *storage.Name, "suffix", te.Suffix)
				if _, err := te.Clients.StorageClient.Delete(ctx, te.ResourceGroupName, *storage.Name, nil); err != nil {
					slog.Warn("Failed to delete storage account", "name", *storage.Name, "error", err)
				}
			}
		}
	}
}

// GetUniqueResourceName generates a unique resource name for testing
func (te *Environment) GetUniqueResourceName(prefix string) string {
	rndNum, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return fmt.Sprintf("%s-%s-%d", prefix, te.Suffix, rndNum.Int64())
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			indexOf(s, substr) >= 0)
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// RequireAzure is deprecated - Azure tests always run
func RequireAzure(t *testing.T) {
	t.Helper()
	// Azure tests always run - no skipping
}

// LogTestProgress logs progress during long-running tests
func LogTestProgress(t *testing.T, operation string, details ...interface{}) {
	t.Helper()

	args := []interface{}{"test", t.Name(), "operation", operation}
	args = append(args, details...)

	slog.Debug("Test progress", args...)
}

// setupTestLogger configures slog to use debug level for test visibility
func setupTestLogger() {
	// Create a text handler for better readability in tests
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	slog.SetDefault(slog.New(handler))
}
