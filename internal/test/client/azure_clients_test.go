//go:build client

package client

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestAzureClientInitialization(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

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
			env := test.SetupMinimalTestEnvironment(t)

			// Skip managed identity test if not in Azure environment
			if !tt.useAzureCLI && os.Getenv("CI") == "" {
				t.Skip("Managed Identity test requires Azure environment")
			}

			clients := infra.NewAzureClients(env.Suffix, tt.useAzureCLI)

			// Verify clients struct is properly initialized
			require.NotNil(t, clients, "AzureClients should not be nil")
			assert.NotEmpty(t, clients.SubscriptionID, "SubscriptionID should be set")
			assert.Equal(t, env.Suffix, clients.Suffix, "Suffix should match")
			assert.NotEmpty(t, clients.ResourceGroupName, "ResourceGroupName should be set")
			assert.NotNil(t, clients.Cred, "Credentials should be initialized")

			// Verify individual clients are initialized
			assert.NotNil(t, clients.ResourceClient, "ResourceClient should be initialized")
			assert.NotNil(t, clients.NetworkClient, "NetworkClient should be initialized")
			assert.NotNil(t, clients.NSGClient, "NSGClient should be initialized")
			assert.NotNil(t, clients.ComputeClient, "ComputeClient should be initialized")
			assert.NotNil(t, clients.PublicIPClient, "PublicIPClient should be initialized")
			assert.NotNil(t, clients.NICClient, "NICClient should be initialized")
			assert.NotNil(t, clients.StorageClient, "StorageClient should be initialized")
			assert.NotNil(t, clients.RoleClient, "RoleClient should be initialized")
			assert.NotNil(t, clients.DisksClient, "DisksClient should be initialized")
			assert.NotNil(t, clients.SnapshotsClient, "SnapshotsClient should be initialized")
			assert.NotNil(t, clients.ResourceGraphClient, "ResourceGraphClient should be initialized")
		})
	}
}

func TestCredentialCreation(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

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
			if tt.skipCondition != "" && os.Getenv("CI") == "" {
				t.Skip(tt.skipCondition)
			}

			cred, err := tt.credFunc()
			require.NoError(t, err, "Credential creation should succeed")
			require.NotNil(t, cred, "Credential should not be nil")
		})
	}
}

func TestSubscriptionDiscovery(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	// This test verifies that the Azure client initialization discovers a valid subscription
	env := test.SetupMinimalTestEnvironment(t)
	clients := infra.NewAzureClients(env.Suffix, true)

	// Verify subscription ID was discovered and has the correct format
	assert.NotEmpty(t, clients.SubscriptionID, "Subscription ID should be discovered")
	assert.Len(t, clients.SubscriptionID, 36, "Subscription ID should be a valid GUID (36 characters)")
}

func TestClientOperationTimeout(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	env := test.SetupMinimalTestEnvironment(t)
	clients := infra.NewAzureClients(env.Suffix, true) // Use Azure CLI for test environment

	// Test basic client operation with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Test ResourceClient list operation (lightweight operation that doesn't create resources)
	pager := clients.ResourceClient.NewListPager(nil)
	_, err := pager.NextPage(ctx)
	require.NoError(t, err, "Resource group listing should succeed")
}

func TestClientValidation(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

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
			assert.NotNil(t, tt.client, "%s should be initialized", tt.name)
		})
	}

	// Validate that credentials and configuration are properly set
	assert.NotNil(t, clients.Cred, "Credentials should be set")
	assert.NotEmpty(t, clients.SubscriptionID, "SubscriptionID should be set")
	assert.Equal(t, env.Suffix, clients.Suffix, "Suffix should match input")
	assert.NotEmpty(t, clients.ResourceGroupName, "ResourceGroupName should be set")
}

func TestResourceGroupNaming(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

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
			clients := infra.NewAzureClients(tt.suffix, true)
			assert.Equal(t, tt.expected, clients.ResourceGroupName, "Resource group name should follow naming convention")
		})
	}
}
