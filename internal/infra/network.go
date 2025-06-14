package infra

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// createNSGRule helper function to reduce boilerplate
func createNSGRule(name, protocol, srcAddr, dstAddr, dstPort string, access armnetwork.SecurityRuleAccess, priority int32, direction armnetwork.SecurityRuleDirection) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: to.Ptr(name),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocol(protocol)),
			SourceAddressPrefix:      to.Ptr(srcAddr),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr(dstAddr),
			DestinationPortRange:     to.Ptr(dstPort),
			Access:                   to.Ptr(access),
			Priority:                 to.Ptr(priority),
			Direction:                to.Ptr(direction),
		},
	}
}

// NSG Rules configuration
var BastionNSGRules = []*armnetwork.SecurityRule{
	createNSGRule("AllowSSHFromInternet", "Tcp", "Internet", "*", "22", armnetwork.SecurityRuleAccessAllow, 100, armnetwork.SecurityRuleDirectionInbound),
	createNSGRule("AllowCustomSSHFromInternet", "Tcp", "Internet", "*", fmt.Sprintf("%d", BastionSSHPort), armnetwork.SecurityRuleAccessAllow, 110, armnetwork.SecurityRuleDirectionInbound),
	createNSGRule("AllowHTTPSFromInternet", "Tcp", "Internet", "*", "443", armnetwork.SecurityRuleAccessAllow, 120, armnetwork.SecurityRuleDirectionInbound),
	createNSGRule("AllowToBoxesSubnet", "*", "*", boxesSubnetCIDR, "*", armnetwork.SecurityRuleAccessAllow, 100, armnetwork.SecurityRuleDirectionOutbound),
	createNSGRule("AllowToInternet", "*", "*", "Internet", "*", armnetwork.SecurityRuleAccessAllow, 110, armnetwork.SecurityRuleDirectionOutbound),
}

// AzureClients holds all the Azure SDK clients needed for the application
type AzureClients struct {
	Cred                         azcore.TokenCredential
	SubscriptionID               string
	Suffix                       string
	ResourceGroupSuffix          string
	ResourceGroupName            string
	BastionSubnetID              string
	BoxesSubnetID                string
	TableStorageConnectionString string
	ResourceClient               *armresources.ResourceGroupsClient
	NetworkClient                *armnetwork.VirtualNetworksClient
	NSGClient                    *armnetwork.SecurityGroupsClient
	SubnetsClient                *armnetwork.SubnetsClient
	ComputeClient                *armcompute.VirtualMachinesClient
	PublicIPClient               *armnetwork.PublicIPAddressesClient
	NICClient                    *armnetwork.InterfacesClient
	StorageClient                *armstorage.AccountsClient
	RoleClient                   *armauthorization.RoleAssignmentsClient
	TableClient                  *aztables.ServiceClient
	DisksClient                  *armcompute.DisksClient
	SnapshotsClient              *armcompute.SnapshotsClient
	ImagesClient                 *armcompute.ImagesClient
	ResourceGraphClient          *armresourcegraph.Client
}

func createResourceGroup(ctx context.Context, clients *AzureClients) {
	hash, err := GenerateConfigHash(clients.Suffix)
	FatalOnError(err, "failed to generate config hash")

	_, err = clients.ResourceClient.CreateOrUpdate(ctx, clients.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(Location),
		Tags: map[string]*string{
			"config": to.Ptr(fmt.Sprintf("sha256-%s", hash)),
		},
	}, nil)
	FatalOnError(err, "failed to create resource group")
}

func createBastionNSG(ctx context.Context, clients *AzureClients) {
	namer := NewResourceNamer(clients.Suffix)
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(Location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: BastionNSGRules,
		},
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionNSGName(), nsgParams, nil)
	FatalOnError(err, "failed to start bastion NSG creation")

	_, err = poller.PollUntilDone(ctx, &DefaultPollOptions)
	FatalOnError(err, "failed to complete bastion NSG creation")
}

func createVirtualNetwork(ctx context.Context, clients *AzureClients) {
	namer := NewResourceNamer(clients.Suffix)
	nsg, err := clients.NSGClient.Get(ctx, clients.ResourceGroupName, namer.BastionNSGName(), nil)
	FatalOnError(err, "failed to get bastion NSG")

	vnetParams := armnetwork.VirtualNetwork{
		Location: to.Ptr(Location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr(vnetAddressSpace)},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: to.Ptr(namer.BastionSubnetName()),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr(bastionSubnetCIDR),
						NetworkSecurityGroup: &armnetwork.SecurityGroup{
							ID: nsg.ID,
						},
					},
				},
				{
					Name: to.Ptr(namer.BoxesSubnetName()),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr(boxesSubnetCIDR),
					},
				},
			},
		},
	}

	poller, err := clients.NetworkClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.VNetName(), vnetParams, nil)
	FatalOnError(err, "failed to start virtual network creation")

	vnetResult, err := poller.PollUntilDone(ctx, &DefaultPollOptions)
	FatalOnError(err, "failed to complete virtual network creation")

	setSubnetIDsFromVNet(clients, vnetResult)
}

func setSubnetIDsFromVNet(clients *AzureClients, vnetResult armnetwork.VirtualNetworksClientCreateOrUpdateResponse) {
	namer := NewResourceNamer(clients.Suffix)
	for _, subnet := range vnetResult.VirtualNetwork.Properties.Subnets {
		switch *subnet.Name {
		case namer.BastionSubnetName():
			clients.BastionSubnetID = *subnet.ID
		case namer.BoxesSubnetName():
			clients.BoxesSubnetID = *subnet.ID
		}
	}

	if clients.BastionSubnetID == "" || clients.BoxesSubnetID == "" {
		slog.Error("missing subnets in VNet")
		os.Exit(1)
	}
}

// InitializeTableStorage sets up Table Storage resources or reads configuration
func InitializeTableStorage(clients *AzureClients, useAzureCli bool) {
	if useAzureCli {
		namer := NewResourceNamer(clients.Suffix)
		storageAccount := namer.SharedStorageAccountName()
		tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

		result := CreateTableStorageResources(
			context.Background(),
			clients,
			storageAccount,
			tableNames,
		)
		FatalOnError(result.Error, "Table Storage setup error")

		clients.TableStorageConnectionString = result.ConnectionString
	} else {
		err := readTableStorageConfig(clients)
		FatalOnError(err, "Failed to read Table Storage config")
	}

	// Create table client from connection string
	createTableClient(clients)
}

func CreateNetworkInfrastructure(ctx context.Context, clients *AzureClients, useAzureCli bool) {
	// 1. Create resource group first and wait for it to be ready
	createResourceGroup(ctx, clients)

	// Start Table Storage initialization in parallel with NSG and VNet creation
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Initialize Table Storage after resource group is created
		InitializeTableStorage(clients, useAzureCli)
	}()

	// 2. Create NSG first since VNet depends on it
	createBastionNSG(ctx, clients)

	// 3. Create VNet after NSG is ready
	createVirtualNetwork(ctx, clients)

	// Wait for Table Storage initialization to complete
	wg.Wait()
}
