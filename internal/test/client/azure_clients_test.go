package client

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// Helper function to reduce cyclomatic complexity
func verifyAzureClientFields(t *testing.T, clients *infra.AzureClients, expectedSuffix string) {
	t.Helper()

	// Verify basic fields
	basicFields := map[string]string{
		"SubscriptionID":    clients.SubscriptionID,
		"ResourceGroupName": clients.ResourceGroupName,
	}

	for fieldName, value := range basicFields {
		if value == "" {
			t.Errorf("%s should be set", fieldName)
		}
	}

	if clients.Suffix != expectedSuffix {
		t.Errorf("Suffix should match: expected %q, got %q", expectedSuffix, clients.Suffix)
	}

	if clients.Cred == nil {
		t.Errorf("Credentials should be initialized")
	}

	// Verify all service clients using reflection-like approach
	clientChecks := map[string]interface{}{
		"ResourceClient":      clients.ResourceClient,
		"NetworkClient":       clients.NetworkClient,
		"NSGClient":           clients.NSGClient,
		"ComputeClient":       clients.ComputeClient,
		"PublicIPClient":      clients.PublicIPClient,
		"NICClient":           clients.NICClient,
		"StorageClient":       clients.StorageClient,
		"RoleClient":          clients.RoleClient,
		"DisksClient":         clients.DisksClient,
		"SnapshotsClient":     clients.SnapshotsClient,
		"ResourceGraphClient": clients.ResourceGraphClient,
	}

	for clientName, client := range clientChecks {
		if client == nil {
			t.Errorf("%s should be initialized", clientName)
		}
	}
}

func TestAzureClientInitialization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		useAzureCLI bool
		description string
	}{
		{
			name:        "ManagedIdentity",
			useAzureCLI: false,
			description: "Test client initialization with Managed Identity credentials",
		},
		{
			name:        "AzureCLI",
			useAzureCLI: true,
			description: "Test client initialization with Azure CLI credentials",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := test.SetupMinimalTestEnvironment(t)

			if !tt.useAzureCLI && os.Getenv("CI") == "" {
				t.Skip("Managed Identity test requires Azure environment")
			}

			clients := infra.NewAzureClients(env.Suffix, tt.useAzureCLI)

			if clients == nil {
				t.Fatalf("AzureClients should not be nil")
			}

			verifyAzureClientFields(t, clients, env.Suffix)
		})
	}
}

func TestCredentialCreation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		credFunc      func() (azcore.TokenCredential, error)
		skipCondition string
	}{
		{
			name: "AzureCLICredential",
			credFunc: func() (azcore.TokenCredential, error) {
				return azidentity.NewAzureCLICredential(nil)
			},
			skipCondition: "",
		},
		{
			name: "ManagedIdentityCredential",
			credFunc: func() (azcore.TokenCredential, error) {
				return azidentity.NewManagedIdentityCredential(nil)
			},
			skipCondition: "Managed Identity requires Azure environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.skipCondition != "" && os.Getenv("CI") == "" {
				t.Skip(tt.skipCondition)
			}

			cred, err := tt.credFunc()
			if err != nil {
				t.Fatalf("Credential creation should succeed: %v", err)
			}
			if cred == nil {
				t.Fatalf("Credential should not be nil")
			}
		})
	}
}

func TestSubscriptionDiscovery(t *testing.T) {
	t.Parallel()
	// This test verifies that the Azure client initialization discovers a valid subscription
	env := test.SetupMinimalTestEnvironment(t)
	clients := infra.NewAzureClients(env.Suffix, true)

	// Verify subscription ID was discovered and has the correct format
	if clients.SubscriptionID == "" {
		t.Errorf("Subscription ID should be discovered")
	}
	if len(clients.SubscriptionID) != 36 {
		t.Errorf("Subscription ID should be a valid GUID (36 characters), got %d characters", len(clients.SubscriptionID))
	}
}

func TestClientOperationTimeout(t *testing.T) {
	t.Parallel()
	env := test.SetupMinimalTestEnvironment(t)
	clients := infra.NewAzureClients(env.Suffix, true) // Use Azure CLI for test environment

	// Test basic client operation with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test ResourceClient list operation (lightweight operation that doesn't create resources)
	pager := clients.ResourceClient.NewListPager(nil)
	_, err := pager.NextPage(ctx)
	if err != nil {
		t.Fatalf("Resource group listing should succeed: %v", err)
	}
}

func TestClientValidation(t *testing.T) {
	t.Parallel()
	env := test.SetupMinimalTestEnvironment(t)
	clients := infra.NewAzureClients(env.Suffix, true)

	// Validate that all required clients are properly typed
	tests := []struct {
		name   string
		client interface{}
	}{
		{"ResourceClient", clients.ResourceClient},
		{"NetworkClient", clients.NetworkClient},
		{"NSGClient", clients.NSGClient},
		{"ComputeClient", clients.ComputeClient},
		{"PublicIPClient", clients.PublicIPClient},
		{"NICClient", clients.NICClient},
		{"StorageClient", clients.StorageClient},
		{"RoleClient", clients.RoleClient},
		{"DisksClient", clients.DisksClient},
		{"SnapshotsClient", clients.SnapshotsClient},
		{"ResourceGraphClient", clients.ResourceGraphClient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.client == nil {
				t.Errorf("%s should be initialized", tt.name)
			}
		})
	}

	// Validate that credentials and configuration are properly set
	if clients.Cred == nil {
		t.Errorf("Credentials should be set")
	}
	if clients.SubscriptionID == "" {
		t.Errorf("SubscriptionID should be set")
	}
	if clients.Suffix != env.Suffix {
		t.Errorf("Suffix should match input: expected %q, got %q", env.Suffix, clients.Suffix)
	}
	if clients.ResourceGroupName == "" {
		t.Errorf("ResourceGroupName should be set")
	}
}

func TestResourceGroupNaming(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		suffix   string
		expected string
	}{
		{
			name:     "BasicSuffix",
			suffix:   "test123",
			expected: "shellbox-test123",
		},
		{
			name:     "NumericSuffix",
			suffix:   "456",
			expected: "shellbox-456",
		},
		{
			name:     "AlphanumericSuffix",
			suffix:   "dev99",
			expected: "shellbox-dev99",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			clients := infra.NewAzureClients(tt.suffix, true)
			if clients.ResourceGroupName != tt.expected {
				t.Errorf("Resource group name should follow naming convention: expected %q, got %q", tt.expected, clients.ResourceGroupName)
			}
		})
	}
}
