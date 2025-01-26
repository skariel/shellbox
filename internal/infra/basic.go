package infra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
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
	cred              *azidentity.ManagedIdentityCredential
	subscriptionID    string
	ResourceGroupName string
	BastionSubnetID   string
	BoxesSubnetID     string
	ResourceClient    *armresources.ResourceGroupsClient
	NetworkClient     *armnetwork.VirtualNetworksClient
	NSGClient         *armnetwork.SecurityGroupsClient
	ComputeClient     *armcompute.VirtualMachinesClient
	PublicIPClient    *armnetwork.PublicIPAddressesClient
	NICClient         *armnetwork.InterfacesClient
	CosmosClient      *armcosmos.DatabaseAccountsClient
	KeyVaultClient    *armkeyvault.VaultsClient
	SecretsClient     *armkeyvault.SecretsClient
	RoleClient        *armauthorization.RoleAssignmentsClient
}

func __createResourceGroupClient(clients *AzureClients) error {
	client, err := armresources.NewResourceGroupsClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}
	clients.ResourceClient = client
	return nil
}

func __createNetworkClient(clients *AzureClients) error {
	client, err := armnetwork.NewVirtualNetworksClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create network client: %w", err)
	}
	clients.NetworkClient = client
	return nil
}

func __createNSGClient(clients *AzureClients) error {
	client, err := armnetwork.NewSecurityGroupsClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NSG client: %w", err)
	}
	clients.NSGClient = client
	return nil
}

func __createPublicIPClient(clients *AzureClients) error {
	client, err := armnetwork.NewPublicIPAddressesClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create Public IP client: %w", err)
	}
	clients.PublicIPClient = client
	return nil
}

func __createNICClient(clients *AzureClients) error {
	client, err := armnetwork.NewInterfacesClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create interfaces client: %w", err)
	}
	clients.NICClient = client
	return nil
}

func __createComputeClient(clients *AzureClients) error {
	client, err := armcompute.NewVirtualMachinesClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %w", err)
	}
	clients.ComputeClient = client
	return nil
}

func __createCosmosClient(clients *AzureClients) error {
	client, err := armcosmos.NewDatabaseAccountsClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create cosmos client: %w", err)
	}
	clients.CosmosClient = client
	return nil
}

func __createKeyVaultClient(clients *AzureClients) error {
	client, err := armkeyvault.NewVaultsClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create key vault client: %w", err)
	}
	clients.KeyVaultClient = client
	return nil
}

func __createSecretsClient(clients *AzureClients) error {
	client, err := armkeyvault.NewSecretsClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}
	clients.SecretsClient = client
	return nil
}

func __createRoleClient(clients *AzureClients) error {
	client, err := armauthorization.NewRoleAssignmentsClient(clients.subscriptionID, clients.cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignments client: %w", err)
	}
	clients.RoleClient = client
	return nil
}

// NewAzureClients creates all Azure clients using credential-based subscription ID discovery
func NewAzureClients() (*AzureClients, error) {
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
		cred:              cred,
		subscriptionID:    subscriptionID,
		ResourceGroupName: "",
		BastionSubnetID:   "",
		BoxesSubnetID:     "",
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

func serializeNSGRules(rules []*armnetwork.SecurityRule) string {
	var parts []string
	for _, rule := range rules {
		parts = append(parts,
			fmt.Sprintf("%s-%d-%s-%s-%s",
				*rule.Name,
				*rule.Properties.Priority,
				*rule.Properties.Direction,
				*rule.Properties.Access,
				*rule.Properties.DestinationPortRange,
			))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func __getNSGRules() []*armnetwork.SecurityRule {
	return []*armnetwork.SecurityRule{
		{
			Name: to.Ptr("AllowSSHFromInternet"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
				SourceAddressPrefix:      to.Ptr("Internet"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("*"),
				DestinationPortRange:     to.Ptr("22"),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
				Priority:                 to.Ptr(int32(100)),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			},
		},
		{
			Name: to.Ptr("AllowHTTPSFromInternet"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
				SourceAddressPrefix:      to.Ptr("Internet"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("*"),
				DestinationPortRange:     to.Ptr("443"),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
				Priority:                 to.Ptr(int32(110)),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			},
		},
		{
			Name: to.Ptr("DenyAllInbound"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
				SourceAddressPrefix:      to.Ptr("*"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("*"),
				DestinationPortRange:     to.Ptr("*"),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
				Priority:                 to.Ptr(int32(4096)),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			},
		},
		{
			Name: to.Ptr("AllowToBoxesSubnet"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
				SourceAddressPrefix:      to.Ptr("*"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr(boxesSubnetCIDR),
				DestinationPortRange:     to.Ptr("*"),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
				Priority:                 to.Ptr(int32(100)),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			},
		},
		{
			Name: to.Ptr("AllowToInternet"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
				SourceAddressPrefix:      to.Ptr("*"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("Internet"),
				DestinationPortRange:     to.Ptr("*"),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
				Priority:                 to.Ptr(int32(110)),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			},
		},
		{
			Name: to.Ptr("DenyAllOutbound"),
			Properties: &armnetwork.SecurityRulePropertiesFormat{
				Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
				SourceAddressPrefix:      to.Ptr("*"),
				SourcePortRange:          to.Ptr("*"),
				DestinationAddressPrefix: to.Ptr("*"),
				DestinationPortRange:     to.Ptr("*"),
				Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
				Priority:                 to.Ptr(int32(4096)),
				Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
			},
		},
	}
}

func GenerateConfigHash() (string, error) {
	nsgRules := __getNSGRules()
	hashInput := fmt.Sprintf("%s-%s-%s-%s",
		bastionSubnetCIDR,
		boxesSubnetCIDR,
		vnetAddressSpace,
		serializeNSGRules(nsgRules),
	)

	hasher := sha256.New()
	hasher.Write([]byte(hashInput))
	return hex.EncodeToString(hasher.Sum(nil))[:8], nil
}

func __createResourceGroup(ctx context.Context, clients *AzureClients) error {
	hash, err := GenerateConfigHash()
	if err != nil {
		return fmt.Errorf("failed to generate config hash: %w", err)
	}

	_, err = clients.ResourceClient.CreateOrUpdate(ctx, ResourceGroup, armresources.ResourceGroup{
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
			// these securitygroups are duplicated in the function GenerateConfigHas above.
			// can these instead be refactored into a function that returns the security groups and is
			// use in both places? AI?
			SecurityRules: __getNSGRules(),
		},
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, rgName, bastionNSGName, nsgParams, nil)
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

	poller, err := clients.NetworkClient.BeginCreateOrUpdate(ctx, rgName, vnetName, vnetParams, nil)
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
