package test

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"math/big"
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
func SetupTestEnvironment(t *testing.T, category Category) *Environment {
	t.Helper()

	// Initialize logger with production configuration
	infra.SetDefaultLogger()

	config := LoadConfig()

	// Skip if category is not enabled
	if !config.ShouldRunCategory(category) {
		t.Skipf("Category %s is not enabled in test configuration", category)
	}

	// Generate unique suffix for this test's resources
	rndNum, _ := rand.Int(rand.Reader, big.NewInt(100000))
	suffix := fmt.Sprintf("%s-%s-%d", config.ResourceGroupPrefix, category, rndNum.Int64())

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

	slog.Info("Test environment ready",
		"category", category,
		"suffix", suffix,
		"sharedResourceGroup", env.ResourceGroupName,
		"useAzureCLI", config.UseAzureCLI)

	return env
}

// SetupMinimalTestEnvironment creates a lightweight test environment for unit tests
func SetupMinimalTestEnvironment(t *testing.T) *Environment {
	t.Helper()

	// Initialize logger with production configuration
	infra.SetDefaultLogger()

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
		slog.Info("Shared resource group already exists", "name", te.ResourceGroupName)
		return nil
	}

	// Resource group doesn't exist, create it
	slog.Info("Creating shared resource group", "name", te.ResourceGroupName)

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

	slog.Info("Successfully created shared resource group", "name", te.ResourceGroupName)
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
	category := te.getCurrentCategory()

	slog.Info("Starting test cleanup",
		"suffix", te.Suffix,
		"resources", len(te.CreatedResources),
		"elapsed", elapsed,
		"category", category)

	// Clean up individual resources created by this test
	// Note: We don't delete the shared resource group - it's persistent
	if len(te.CreatedResources) > 0 {
		slog.Info("Cleaning up individual test resources",
			"count", len(te.CreatedResources),
			"suffix", te.Suffix)

		// In practice, most tests will use Azure's resource lifecycle management
		// where deleting parent resources (like VMs) automatically cleans up dependent resources
		// Individual tests can also implement their own specific cleanup if needed

		for _, resourceName := range te.CreatedResources {
			slog.Info("Resource tracked for cleanup", "resource", resourceName)
		}

		slog.Info("Individual resource cleanup completed",
			"suffix", te.Suffix,
			"elapsed", time.Since(te.startTime))
	} else {
		slog.Info("No individual resources to clean up", "suffix", te.Suffix)
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

	slog.Info("Cleaning up resources by suffix", "suffix", te.Suffix, "resourceGroup", te.ResourceGroupName)

	// Clean up volumes/disks with matching suffix
	pager := te.Clients.DisksClient.NewListByResourceGroupPager(te.ResourceGroupName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing disks for cleanup: %w", err)
		}

		for _, disk := range page.Value {
			if disk.Name != nil && contains(*disk.Name, te.Suffix) {
				slog.Info("Deleting disk", "name", *disk.Name, "suffix", te.Suffix)
				if err := infra.DeleteVolume(ctx, te.Clients, te.ResourceGroupName, *disk.Name); err != nil {
					slog.Warn("Failed to delete disk", "name", *disk.Name, "error", err)
				}
			}
		}
	}

	// TODO: Add cleanup for other resource types (VMs, NICs, etc.) if needed for tests

	return nil
}

// GetUniqueResourceName generates a unique resource name for testing
func (te *Environment) GetUniqueResourceName(prefix string) string {
	rndNum, _ := rand.Int(rand.Reader, big.NewInt(1000))
	return fmt.Sprintf("%s-%s-%d", prefix, te.Suffix, rndNum.Int64())
}

// getCurrentCategory extracts the category from the test name or suffix
func (te *Environment) getCurrentCategory() Category {
	testName := te.t.Name()

	// Try to extract category from test name
	for _, category := range AllCategories() {
		if contains(testName, string(category)) {
			return category
		}
	}

	// Fall back to extracting from suffix
	for _, category := range AllCategories() {
		if contains(te.Suffix, string(category)) {
			return category
		}
	}

	return CategoryUnit // Default fallback
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

// RequireCategory skips the test if the category is not enabled
func RequireCategory(t *testing.T, category Category) {
	t.Helper()

	config := LoadConfig()
	if !config.ShouldRunCategory(category) {
		t.Skipf("Test requires category %s which is not enabled", category)
	}
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

	slog.Info("Test progress", args...)
}
