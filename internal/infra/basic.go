package infra

import (
	"context"
	"fmt"
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

func __createResourceGroupClient(clients *AzureClients) error {
	client, err := armresources.NewResourceGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}
	clients.ResourceClient = client
	return nil
}

func __createNetworkClient(clients *AzureClients) error {
	client, err := armnetwork.NewVirtualNetworksClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create network client: %w", err)
	}
	clients.NetworkClient = client
	return nil
}

func __createNSGClient(clients *AzureClients) error {
	client, err := armnetwork.NewSecurityGroupsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NSG client: %w", err)
	}
	clients.NSGClient = client
	return nil
}

func __createPublicIPClient(clients *AzureClients) error {
	client, err := armnetwork.NewPublicIPAddressesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create Public IP client: %w", err)
	}
	clients.PublicIPClient = client
	return nil
}

func __createNICClient(clients *AzureClients) error {
	client, err := armnetwork.NewInterfacesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create interfaces client: %w", err)
	}
	clients.NICClient = client
	return nil
}

func __createComputeClient(clients *AzureClients) error {
	client, err := armcompute.NewVirtualMachinesClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	clients.ComputeClient = client
	return nil
}

func __createCosmosClient(clients *AzureClients) error {
	client, err := armcosmos.NewDatabaseAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create cosmos client: %w", err)
	}
	clients.CosmosClient = client
	return nil
}

func __createKeyVaultClient(clients *AzureClients) error {
	client, err := armkeyvault.NewVaultsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vault client: %w", err)
	}
	clients.KeyVaultClient = client
	return nil
}

func __createSecretsClient(clients *AzureClients) error {
	client, err := armkeyvault.NewSecretsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}
	clients.SecretsClient = client
	return nil
}

func __createRoleClient(clients *AzureClients) error {
	client, err := armauthorization.NewRoleAssignmentsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}
	clients.RoleClient = client
	return nil
}

// NewAzureClients creates all Azure clients using credential-based subscription ID discovery
func NewAzureClients(suffix string) (*AzureClients, error) {
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	subsClient, err := armsubscriptions.NewClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create subscriptions client: %w", err)
	}

	pager := subsClient.NewListPager(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get first subscription ID (assuming single subscription access)
	var subscriptionID string
	if pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list subscriptions: %w", err)
		}
		if len(page.Value) == 0 {
			return nil, fmt.Errorf("no subscriptions found for managed identity")
		}
		subscriptionID = *page.Value[0].SubscriptionID
	} else {
		return nil, fmt.Errorf("no subscriptions available")
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

	g, _ := errgroup.WithContext(context.Background())

	g.Go(func() error { return __createResourceGroupClient(clients) })
	g.Go(func() error { return __createNetworkClient(clients) })
	g.Go(func() error { return __createNSGClient(clients) })
	g.Go(func() error { return __createPublicIPClient(clients) })
	g.Go(func() error { return __createNICClient(clients) })
	g.Go(func() error { return __createComputeClient(clients) })
	g.Go(func() error { return __createCosmosClient(clients) })
	g.Go(func() error { return __createKeyVaultClient(clients) })
	g.Go(func() error { return __createSecretsClient(clients) })
	g.Go(func() error { return __createRoleClient(clients) })

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return clients, nil
}

func __defaultPollOptions() *runtime.PollUntilDoneOptions {
	return &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}
}

func __createResourceGroup(ctx context.Context, clients *AzureClients) error {
	hash, err := generateConfigHash(clients.ResourceGroupName)
	if err != nil {
		return fmt.Errorf("failed to generate config hash: %w", err)
	}

	_, err = clients.ResourceClient.CreateOrUpdate(ctx, clients.ResourceGroupName, armresources.ResourceGroup{
		Location: to.Ptr(location),
		Tags: map[string]*string{
			"config": to.Ptr(fmt.Sprintf("sha256-%s", hash)),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}
	return nil
}

func __createBastionNSG(ctx context.Context, clients *AzureClients) error {
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: BastionNSGRules,
		},
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, bastionNSGName, nsgParams, nil)
	if err != nil {
		return fmt.Errorf("failed to start bastion NSG creation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, __defaultPollOptions())
	return err
}

func __createVirtualNetwork(ctx context.Context, clients *AzureClients) error {
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
		return fmt.Errorf("failed to start virtual network creation: %w", err)
	}

	vnetResult, err := poller.PollUntilDone(ctx, __defaultPollOptions())
	if err != nil {
		return err
	}

	return __setSubnetIDsFromVNet(clients, vnetResult)
}

func __setSubnetIDsFromVNet(clients *AzureClients, vnetResult armnetwork.VirtualNetworksClientCreateOrUpdateResponse) error {
	for _, subnet := range vnetResult.VirtualNetwork.Properties.Subnets {
		switch *subnet.Name {
		case bastionSubnetName:
			clients.BastionSubnetID = *subnet.ID
		case boxesSubnetName:
			clients.BoxesSubnetID = *subnet.ID
		}
	}

	if clients.BastionSubnetID == "" || clients.BoxesSubnetID == "" {
		return fmt.Errorf("missing subnets in VNet")
	}
	return nil
}

func CreateNetworkInfrastructure(ctx context.Context, clients *AzureClients) error {
	g, _ := errgroup.WithContext(ctx)

	g.Go(func() error { return __createResourceGroup(ctx, clients) })
	g.Go(func() error { return __createBastionNSG(ctx, clients) })

	if err := g.Wait(); err != nil {
		return err
	}

	if err := __createVirtualNetwork(ctx, clients); err != nil {
		return err
	}

	return nil
}
