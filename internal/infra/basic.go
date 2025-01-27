package infra

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/cosmos/armcosmos"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"golang.org/x/sync/errgroup"
)

// VMConfig holds common VM configuration fields
type VMConfig struct {
	AdminUsername string
	SSHPublicKey  string
	VMSize        string
}

// AzureClients holds all the Azure SDK clients needed for the application
type AzureClients struct {
	Cred                *azidentity.ManagedIdentityCredential
	SubscriptionID      string
	ResourceGroupSuffix string
	ResourceGroupName   string
	BastionSubnetID     string
	BoxesSubnetID       string
	ResourceClient      *armresources.ResourceGroupsClient
	NetworkClient       *armnetwork.VirtualNetworksClient
	NSGClient           *armnetwork.SecurityGroupsClient
	ComputeClient       *armcompute.VirtualMachinesClient
	PublicIPClient      *armnetwork.PublicIPAddressesClient
	NICClient           *armnetwork.InterfacesClient
	CosmosClient        *armcosmos.DatabaseAccountsClient
	KeyVaultClient      *armkeyvault.VaultsClient
	SecretsClient       *armkeyvault.SecretsClient
	RoleClient          *armauthorization.RoleAssignmentsClient
}

func __createResourceGroupClient(clients *AzureClients) {
	client, err := armresources.NewResourceGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create resource group client: %v", err)
	}
	clients.ResourceClient = client
}

func __createNetworkClient(clients *AzureClients) {
	client, err := armnetwork.NewVirtualNetworksClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create network client: %v", err)
	}
	clients.NetworkClient = client
}

func __createNSGClient(clients *AzureClients) {
	client, err := armnetwork.NewSecurityGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create NSG client: %v", err)
	}
	clients.NSGClient = client
}

func __createPublicIPClient(clients *AzureClients) {
	client, err := armnetwork.NewPublicIPAddressesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create Public IP client: %v", err)
	}
	clients.PublicIPClient = client
}

func __createNICClient(clients *AzureClients) {
	client, err := armnetwork.NewInterfacesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create interfaces client: %v", err)
	}
	clients.NICClient = client
}

func __createComputeClient(clients *AzureClients) {
	client, err := armcompute.NewVirtualMachinesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create compute client: %v", err)
	}
	clients.ComputeClient = client
}

func __createCosmosClient(clients *AzureClients) {
	client, err := armcosmos.NewDatabaseAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos client: %v", err)
	}
	clients.CosmosClient = client
}

func __createKeyVaultClient(clients *AzureClients) {
	client, err := armkeyvault.NewVaultsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create key vault client: %v", err)
	}
	clients.KeyVaultClient = client
}

func __createSecretsClient(clients *AzureClients) {
	client, err := armkeyvault.NewSecretsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create secrets client: %v", err)
	}
	clients.SecretsClient = client
}

func __createRoleClient(clients *AzureClients) {
	client, err := armauthorization.NewRoleAssignmentsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create role assignments client: %v", err)
	}
	clients.RoleClient = client
}

// NewAzureClients creates all Azure clients using credential-based subscription ID discovery
func NewAzureClients(suffix string) *AzureClients {
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		log.Fatalf("failed to create credential: %v", err)
	}

	subsClient, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		log.Fatalf("failed to create subscriptions client: %v", err)
	}

	pager := subsClient.NewListPager(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get first subscription ID (assuming single subscription access)
	var subscriptionID string
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			log.Fatalf("failed to list subscriptions: %v", err)
		}
		if len(page.Value) == 0 {
			log.Fatal("no subscriptions found for managed identity")
		}
		subscriptionID = *page.Value[0].SubscriptionID
	} else {
		log.Fatal("no subscriptions available")
	}

	// Initialize clients with parallel client creation
	clients := &AzureClients{
		Cred:                cred,
		SubscriptionID:      subscriptionID,
		ResourceGroupSuffix: suffix,
		ResourceGroupName:   resourceGroupPrefix + "-" + suffix,
		BastionSubnetID:     "",
		BoxesSubnetID:       "",
	}

	g := new(errgroup.Group)

	g.Go(func() error { __createResourceGroupClient(clients); return nil })
	g.Go(func() error { __createNetworkClient(clients); return nil })
	g.Go(func() error { __createNSGClient(clients); return nil })
	g.Go(func() error { __createPublicIPClient(clients); return nil })
	g.Go(func() error { __createNICClient(clients); return nil })
	g.Go(func() error { __createComputeClient(clients); return nil })
	g.Go(func() error { __createCosmosClient(clients); return nil })
	g.Go(func() error { __createKeyVaultClient(clients); return nil })
	g.Go(func() error { __createSecretsClient(clients); return nil })
	g.Go(func() error { __createRoleClient(clients); return nil })

	_ = g.Wait() // We can ignore the error since the functions use log.Fatal

	return clients
}

func __defaultPollOptions() *runtime.PollUntilDoneOptions {
	return &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}
}

func __createResourceGroup(ctx context.Context, clients *AzureClients) {
	hash, err := generateConfigHash(clients.ResourceGroupName)
	if err != nil {
		log.Fatalf("failed to generate config hash: %v", err)
	}

	_, err = clients.ResourceClient.CreateOrUpdate(ctx, clients.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(location),
		Tags: map[string]*string{
			"config": to.Ptr(fmt.Sprintf("sha256-%s", hash)),
		},
	}, nil)
	if err != nil {
		log.Fatalf("failed to create resource group: %v", err)
	}
}

func __createBastionNSG(ctx context.Context, clients *AzureClients) {
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: BastionNSGRules,
		},
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, bastionNSGName, nsgParams, nil)
	if err != nil {
		log.Fatalf("failed to start bastion NSG creation: %v", err)
	}

	_, err = poller.PollUntilDone(ctx, __defaultPollOptions())
	if err != nil {
		log.Fatalf("failed to complete bastion NSG creation: %v", err)
	}
}

func __createVirtualNetwork(ctx context.Context, clients *AzureClients) {
	vnetParams := armnetwork.VirtualNetwork{
		Location: to.Ptr(location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr(vnetAddressSpace)},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: to.Ptr(bastionSubnetName),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr(bastionSubnetCIDR),
					},
				},
				{
					Name: to.Ptr(boxesSubnetName),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr(boxesSubnetCIDR),
					},
				},
			},
		},
	}

	poller, err := clients.NetworkClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, vnetName, vnetParams, nil)
	if err != nil {
		log.Fatalf("failed to start virtual network creation: %v", err)
	}

	vnetResult, err := poller.PollUntilDone(ctx, __defaultPollOptions())
	if err != nil {
		log.Fatalf("failed to complete virtual network creation: %v", err)
	}

	__setSubnetIDsFromVNet(clients, vnetResult)
}

func __setSubnetIDsFromVNet(clients *AzureClients, vnetResult armnetwork.VirtualNetworksClientCreateOrUpdateResponse) {
	for _, subnet := range vnetResult.VirtualNetwork.Properties.Subnets {
		switch *subnet.Name {
		case bastionSubnetName:
			clients.BastionSubnetID = *subnet.ID
		case boxesSubnetName:
			clients.BoxesSubnetID = *subnet.ID
		}
	}

	if clients.BastionSubnetID == "" || clients.BoxesSubnetID == "" {
		log.Fatal("missing subnets in VNet")
	}
}

func CreateNetworkInfrastructure(ctx context.Context, clients *AzureClients) {
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error { __createResourceGroup(ctx, clients); return nil })
	g.Go(func() error { __createBastionNSG(ctx, clients); return nil })

	if err := g.Wait(); err != nil {
		log.Fatalf("failed to create network infrastructure: %v", err)
	}

	__createVirtualNetwork(ctx, clients)
}
