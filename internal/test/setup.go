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
}

// SetupTestEnvironment creates a new test environment with Azure resources
func SetupTestEnvironment(t *testing.T, category Category) *Environment {
	t.Helper()

	config := LoadConfig()

	// Skip if category is not enabled
	if !config.ShouldRunCategory(category) {
		t.Skipf("Category %s is not enabled in test configuration", category)
	}

	// Skip Azure tests if requested
	if os.Getenv("SKIP_AZURE_TESTS") == "true" {
		t.Skip("Skipping Azure integration tests (SKIP_AZURE_TESTS=true)")
	}

	// Generate unique suffix for this test
	rndNum, _ := rand.Int(rand.Reader, big.NewInt(100000))
	suffix := fmt.Sprintf("%s-%s-%d", config.ResourceGroupPrefix, category, rndNum.Int64())

	// Create Azure clients
	clients := infra.NewAzureClients(suffix, config.UseAzureCLI)

	env := &Environment{
		Config:            config,
		Clients:           clients,
		Suffix:            suffix,
		ResourceGroupName: clients.ResourceGroupName,
		CreatedResources:  []string{},
		t:                 t,
		startTime:         time.Now(),
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
		"resourceGroup", env.ResourceGroupName,
		"useAzureCLI", config.UseAzureCLI)

	return env
}

// SetupMinimalTestEnvironment creates a lightweight test environment for unit tests
func SetupMinimalTestEnvironment(t *testing.T) *Environment {
	t.Helper()

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

// createTestResourceGroup creates a resource group for testing
func (te *Environment) createTestResourceGroup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	slog.Info("Creating test resource group", "name", te.ResourceGroupName)

	_, err := te.Clients.ResourceClient.CreateOrUpdate(ctx, te.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(te.Config.Location),
		Tags: map[string]*string{
			"purpose":     to.Ptr("integration-test"),
			"created":     to.Ptr(time.Now().Format(time.RFC3339)),
			"test-suffix": to.Ptr(te.Suffix),
			"category":    to.Ptr(string(te.getCurrentCategory())),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create test resource group: %w", err)
	}

	te.CreatedResources = append(te.CreatedResources, te.ResourceGroupName)
	return nil
}

// TrackResource adds a resource to the cleanup list
func (te *Environment) TrackResource(resourceName string) {
	te.CreatedResources = append(te.CreatedResources, resourceName)
}

// Cleanup removes all test resources
func (te *Environment) Cleanup() {
	if te.Clients == nil {
		return // Nothing to clean up for minimal environments
	}

	elapsed := time.Since(te.startTime)
	category := te.getCurrentCategory()

	ctx, cancel := context.WithTimeout(context.Background(), te.Config.CleanupTimeout)
	defer cancel()

	slog.Info("Starting test cleanup",
		"suffix", te.Suffix,
		"resources", len(te.CreatedResources),
		"elapsed", elapsed,
		"category", category)

	// Delete the entire resource group - this will delete all resources within it
	if te.ResourceGroupName != "" {
		slog.Info("Deleting test resource group", "name", te.ResourceGroupName)

		poller, err := te.Clients.ResourceClient.BeginDelete(ctx, te.ResourceGroupName, nil)
		if err != nil {
			te.t.Errorf("Failed to start resource group deletion: %v", err)
			return
		}

		_, err = poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
		if err != nil {
			te.t.Errorf("Failed to delete test resource group: %v", err)
		} else {
			slog.Info("Successfully deleted test resource group",
				"name", te.ResourceGroupName,
				"elapsed", time.Since(te.startTime))
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

// RequireAzure skips the test if Azure tests are disabled
func RequireAzure(t *testing.T) {
	t.Helper()

	if os.Getenv("SKIP_AZURE_TESTS") == "true" {
		t.Skip("Skipping Azure integration tests (SKIP_AZURE_TESTS=true)")
	}
}

// LogTestProgress logs progress during long-running tests
func LogTestProgress(t *testing.T, operation string, details ...interface{}) {
	t.Helper()

	args := []interface{}{"test", t.Name(), "operation", operation}
	args = append(args, details...)

	slog.Info("Test progress", args...)
}
