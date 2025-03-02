package infra

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
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

func createResourceGroupClient(clients *AzureClients) {
	client, err := armresources.NewResourceGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create resource group client: %v", err)
	}
	clients.ResourceClient = client
}

func createNetworkClient(clients *AzureClients) {
	client, err := armnetwork.NewVirtualNetworksClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create network client: %v", err)
	}
	clients.NetworkClient = client
}

func createNSGClient(clients *AzureClients) {
	client, err := armnetwork.NewSecurityGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create NSG client: %v", err)
	}
	clients.NSGClient = client
}

func createPublicIPClient(clients *AzureClients) {
	client, err := armnetwork.NewPublicIPAddressesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create Public IP client: %v", err)
	}
	clients.PublicIPClient = client
}

func createNICClient(clients *AzureClients) {
	client, err := armnetwork.NewInterfacesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create interfaces client: %v", err)
	}
	clients.NICClient = client
}

func createComputeClient(clients *AzureClients) {
	client, err := armcompute.NewVirtualMachinesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create compute client: %v", err)
	}
	clients.ComputeClient = client
}

func createCosmosClient(clients *AzureClients) {
	client, err := armcosmos.NewDatabaseAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create cosmos client: %v", err)
	}
	clients.CosmosClient = client
}

func createKeyVaultClient(clients *AzureClients) {
	client, err := armkeyvault.NewVaultsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create key vault client: %v", err)
	}
	clients.KeyVaultClient = client
}

func createSecretsClient(clients *AzureClients) {
	client, err := armkeyvault.NewSecretsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create secrets client: %v", err)
	}
	clients.SecretsClient = client
}

func createRoleClient(clients *AzureClients) {
	client, err := armauthorization.NewRoleAssignmentsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		log.Fatalf("failed to create role assignments client: %v", err)
	}
	clients.RoleClient = client
}

func waitForRoleAssignment(ctx context.Context, cred azcore.TokenCredential) string {
	opts := DefaultRetryOptions()
	opts.Operation = "verify role assignment"
	opts.Timeout = 5 * time.Minute
	opts.Interval = 5 * time.Second

	var subscriptionID string
	_, err := RetryWithTimeout(ctx, opts, func(ctx context.Context) (bool, error) {
		client, err := armsubscriptions.NewClient(cred, nil)
		if err != nil {
			return false, err // retry with error
		}
		pager := client.NewListPager(nil)
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, err // retry with error
		}
		if len(page.Value) == 0 {
			return false, fmt.Errorf("no subscriptions found") // retry with specific error
		}
		subscriptionID = *page.Value[0].SubscriptionID
		return true, nil
	})
	if err != nil {
		log.Fatalf("role assignment verification failed: %v", err)
	}
	return subscriptionID
}

// NewAzureClients creates all Azure clients using credential-based subscription ID discovery
func NewAzureClients(suffix string, use_az_cli bool) *AzureClients {
	var cred azcore.TokenCredential
	var err error

	var subscriptionID string

	if !use_az_cli {
		cred, err = azidentity.NewManagedIdentityCredential(nil)
		if err != nil {
			log.Fatalf("failed to create managed identity credential: %v", err)
		}
		log.Println("waiting for role assignment to propagate...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		subscriptionID = waitForRoleAssignment(ctx, cred)
		log.Println("role assignment active")
	} else {
		// Use Azure CLI credentials
		cred, err = azidentity.NewAzureCLICredential(nil)
		if err != nil {
			log.Fatalf("failed to create Azure CLI credential: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		subscriptionID = waitForRoleAssignment(ctx, cred)
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

	g.Go(func() error { createResourceGroupClient(clients); return nil })
	g.Go(func() error { createNetworkClient(clients); return nil })
	g.Go(func() error { createNSGClient(clients); return nil })
	g.Go(func() error { createPublicIPClient(clients); return nil })
	g.Go(func() error { createNICClient(clients); return nil })
	g.Go(func() error { createComputeClient(clients); return nil })
	g.Go(func() error { createCosmosClient(clients); return nil })
	g.Go(func() error { createKeyVaultClient(clients); return nil })
	g.Go(func() error { createSecretsClient(clients); return nil })
	g.Go(func() error { createRoleClient(clients); return nil })

	_ = g.Wait() // We can ignore the error since the functions use log.Fatal

	return clients
}

func createResourceGroup(ctx context.Context, clients *AzureClients) {
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

func createBastionNSG(ctx context.Context, clients *AzureClients) {
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

	_, err = poller.PollUntilDone(ctx, &defaultPollOptions)
	if err != nil {
		log.Fatalf("failed to complete bastion NSG creation: %v", err)
	}
}

func createVirtualNetwork(ctx context.Context, clients *AzureClients) {
	nsg, err := clients.NSGClient.Get(ctx, clients.ResourceGroupName, bastionNSGName, nil)
	if err != nil {
		log.Fatalf("failed to get bastion NSG: %v", err)
	}

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
						NetworkSecurityGroup: &armnetwork.SecurityGroup{
							ID: nsg.ID,
						},
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

	vnetResult, err := poller.PollUntilDone(ctx, &defaultPollOptions)
	if err != nil {
		log.Fatalf("failed to complete virtual network creation: %v", err)
	}

	setSubnetIDsFromVNet(clients, vnetResult)
}

func setSubnetIDsFromVNet(clients *AzureClients, vnetResult armnetwork.VirtualNetworksClientCreateOrUpdateResponse) {
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
	// 1. Create resource group first and wait for it to be ready
	createResourceGroup(ctx, clients)

	// 2. Create NSG first since VNet depends on it
	createBastionNSG(ctx, clients)

	// 3. Create VNet after NSG is ready
	createVirtualNetwork(ctx, clients)
}
