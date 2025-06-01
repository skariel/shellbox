package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

func FatalOnError(err error, message string) {
	if err != nil {
		slog.Error(message, "error", err)
		os.Exit(1)
	}
}

func createAzureClients(clients *AzureClients) {
	var err error

	clients.ResourceClient, err = armresources.NewResourceGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create resource group client")

	clients.NetworkClient, err = armnetwork.NewVirtualNetworksClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create network client")

	clients.NSGClient, err = armnetwork.NewSecurityGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create NSG client")

	clients.SubnetsClient, err = armnetwork.NewSubnetsClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create subnets client")

	clients.PublicIPClient, err = armnetwork.NewPublicIPAddressesClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create Public IP client")

	clients.NICClient, err = armnetwork.NewInterfacesClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create interfaces client")

	clients.ComputeClient, err = armcompute.NewVirtualMachinesClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create compute client")

	clients.StorageClient, err = armstorage.NewAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create storage client")

	clients.RoleClient, err = armauthorization.NewRoleAssignmentsClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create role assignments client")

	clients.DisksClient, err = armcompute.NewDisksClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create disks client")

	clients.SnapshotsClient, err = armcompute.NewSnapshotsClient(clients.SubscriptionID, clients.Cred, nil)
	FatalOnError(err, "failed to create snapshots client")

	clients.ResourceGraphClient, err = armresourcegraph.NewClient(clients.Cred, nil)
	FatalOnError(err, "failed to create resource graph client")
}

func createTableClient(clients *AzureClients) {
	if err := readTableStorageConfig(clients); err != nil {
		slog.Warn("Table Storage config not available", "error", err)
		return
	}

	client, err := aztables.NewServiceClientFromConnectionString(clients.TableStorageConnectionString, nil)
	if err != nil {
		slog.Warn("Failed to create table storage client", "error", err)
		return
	}
	clients.TableClient = client
}

func waitForRoleAssignment(ctx context.Context, cred azcore.TokenCredential) string {
	var subscriptionID string
	err := RetryOperation(ctx, func(ctx context.Context) error {
		client, err := armsubscriptions.NewClient(cred, nil)
		if err != nil {
			return err
		}
		pager := client.NewListPager(nil)
		page, err := pager.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Value) == 0 {
			return fmt.Errorf("no subscriptions found")
		}
		subscriptionID = *page.Value[0].SubscriptionID
		return nil
	}, 5*time.Minute, 5*time.Second, "verify role assignment")
	FatalOnError(err, "role assignment verification failed")
	return subscriptionID
}

// readTableStorageConfig reads Table Storage connection string from the config file
func readTableStorageConfig(clients *AzureClients) error {
	data, err := os.ReadFile(tableStorageConfigFile)
	if err != nil {
		return fmt.Errorf("failed to read Table Storage config file: %w", err)
	}

	var config struct {
		ConnectionString string `json:"connectionString"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse Table Storage config: %w", err)
	}

	clients.TableStorageConnectionString = config.ConnectionString
	return nil
}

// NewAzureClients creates all Azure clients using credential-based subscription ID discovery
func NewAzureClients(suffix string, useAzureCli bool) *AzureClients {
	var cred azcore.TokenCredential
	var err error

	if !useAzureCli {
		cred, err = azidentity.NewManagedIdentityCredential(nil)
		FatalOnError(err, "failed to create managed identity credential")
	} else {
		// Use Azure CLI credentials
		cred, err = azidentity.NewAzureCLICredential(nil)
		FatalOnError(err, "failed to create Azure CLI credential")
	}

	slog.Info("waiting for role assignment to propagate")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	subscriptionID := waitForRoleAssignment(ctx, cred)
	slog.Info("role assignment active")

	// Initialize clients with parallel client creation
	namer := NewResourceNamer(suffix)
	clients := &AzureClients{
		Cred:                cred,
		SubscriptionID:      subscriptionID,
		Suffix:              suffix,
		ResourceGroupSuffix: suffix,
		ResourceGroupName:   namer.ResourceGroup(),
		BastionSubnetID:     "",
		BoxesSubnetID:       "",
	}

	createAzureClients(clients)
	createTableClient(clients)

	return clients
}
