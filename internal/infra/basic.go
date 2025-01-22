package infra

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
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
)

var (
	_resourceGroupName string
)

// Configuration constants
const (
	// Resource group configuration
	resourceGroupPrefix = "shellbox-infra"
	location            = "westus2"

	// Network configuration
	vnetName         = "shellbox-network"
	vnetAddressSpace = "10.0.0.0/8"

	// Bastion subnet configuration
	bastionSubnetName = "bastion-subnet"
	bastionSubnetCIDR = "10.0.0.0/24"
	bastionNSGName    = "bastion-nsg"

	// Boxes subnet configuration
	boxesSubnetName = "boxes-subnet"
	boxesSubnetCIDR = "10.1.0.0/16"
)

// AzureClients holds all the Azure SDK clients needed for the application
type AzureClients struct {
	ResourceClient *armresources.ResourceGroupsClient
	NetworkClient  *armnetwork.VirtualNetworksClient
	NSGClient      *armnetwork.SecurityGroupsClient
	ComputeClient  *armcompute.VirtualMachinesClient
	PublicIPClient *armnetwork.PublicIPAddressesClient
	NICClient      *armnetwork.InterfacesClient
	CosmosClient   *armcosmos.DatabaseAccountsClient
	KeyVaultClient *armkeyvault.VaultsClient
	SecretsClient  *armkeyvault.SecretsClient
	RoleClient     *armauthorization.RoleAssignmentsClient
}

// GetResourceGroupName returns a resource group name with timestamp
func GetResourceGroupName() string {
	if _resourceGroupName == "" {
		_resourceGroupName = fmt.Sprintf("%s-%d", resourceGroupPrefix, time.Now().Unix())
	}
	return _resourceGroupName
}

// getSubscriptionID gets the subscription ID from az cli
func getSubscriptionID() (string, error) {
	out, err := exec.Command("az", "account", "show", "--query", "id", "-o", "tsv").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get subscription ID: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// newAzureClients creates and returns all necessary Azure clients
func NewAzureClients() (*AzureClients, error) {
	// Get credentials from Azure CLI
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	// Get subscription ID
	subscriptionID, err := getSubscriptionID()
	if err != nil {
		return nil, err
	}

	// Initialize resource client
	resourceClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource group client: %w", err)
	}

	// Initialize network client
	networkClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network client: %w", err)
	}

	// Initialize NSG client
	nsgClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG client: %w", err)
	}

	// Initialize Public IP client
	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Public IP client: %w", err)
	}

	// Initialize NIC client
	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create interfaces client: %w", err)
	}

	// Initialize compute client
	computeClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}

	// Initialize Cosmos DB client
	cosmosClient, err := armcosmos.NewDatabaseAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client: %w", err)
	}

	// Initialize Key Vault clients
	keyVaultClient, err := armkeyvault.NewVaultsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create key vault client: %w", err)
	}

	secretsClient, err := armkeyvault.NewSecretsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create secrets client: %w", err)
	}

	roleClient, err := armauthorization.NewRoleAssignmentsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create role assignments client: %w", err)
	}

	return &AzureClients{
		ResourceClient: resourceClient,
		NetworkClient:  networkClient,
		NSGClient:      nsgClient,
		ComputeClient:  computeClient,
		PublicIPClient: publicIPClient,
		NICClient:      nicClient,
		CosmosClient:   cosmosClient,
		KeyVaultClient: keyVaultClient,
		SecretsClient:  secretsClient,
		RoleClient:     roleClient,
	}, nil
}

var (
	_bastionSubnetID string
	_boxesSubnetID   string
)

// GetBastionSubnetID returns the ID of the bastion subnet
func GetBastionSubnetID() (string, error) {
	if _bastionSubnetID != "" {
		return _bastionSubnetID, nil
	}
	return "", errors.New("could not find BastionSubnetID")
}

// GetBoxesSubnetID returns the ID of the boxes subnet
func GetBoxesSubnetID() (string, error) {
	if _boxesSubnetID != "" {
		return _boxesSubnetID, nil
	}
	return "", errors.New("could not find BoxesSubnetID")
}

// CreateNetworkInfrastructure sets up the basic network infrastructure in Azure
func CreateNetworkInfrastructure(ctx context.Context, clients *AzureClients) error {
	pollUntilDoneOption := runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	rgName := GetResourceGroupName()
	// Create resource group
	_, err := clients.ResourceClient.CreateOrUpdate(ctx, rgName, armresources.ResourceGroup{
		Location: to.Ptr(location),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group: %w", err)
	}

	// Create Bastion NSG
	bastionNSGPoller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, rgName, bastionNSGName, armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
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
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start bastion NSG creation: %w", err)
	}
	bastionNSG, err := bastionNSGPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create bastion NSG: %w", err)
	}

	// Create Virtual Network with subnets
	vnetPoller, err := clients.NetworkClient.BeginCreateOrUpdate(ctx, rgName, vnetName, armnetwork.VirtualNetwork{
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
							ID: bastionNSG.ID,
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
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start virtual network creation: %w", err)
	}
	vnetResult, err := vnetPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create virtual network: %w", err)
	}

	// Find bastion and boxes subnets
	for _, subnet := range vnetResult.VirtualNetwork.Properties.Subnets {
		switch *subnet.Name {
		case bastionSubnetName:
			_bastionSubnetID = *subnet.ID
		case boxesSubnetName:
			_boxesSubnetID = *subnet.ID
		}
	}
	if _bastionSubnetID == "" {
		return fmt.Errorf("bastion subnet not found in VNet")
	}
	if _boxesSubnetID == "" {
		return fmt.Errorf("boxes subnet not found in VNet")
	}

	return nil
}

// CleanupOldResourceGroups deletes resource groups older than 5 minutes
func CleanupOldResourceGroups(ctx context.Context, clients *AzureClients) error {
	pager := clients.ResourceClient.NewListPager(nil)
	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing resource groups: %w", err)
		}

		for _, group := range page.Value {
			// Only process groups with our prefix
			if !strings.HasPrefix(*group.Name, resourceGroupPrefix) {
				continue
			}

			// Parse timestamp from group name
			parts := strings.Split(*group.Name, "-")
			if len(parts) != 3 {
				continue
			}

			timestamp, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				log.Printf("Invalid timestamp in resource group name %s: %v", *group.Name, err)
				continue
			}

			createTime := time.Unix(timestamp, 0)
			if createTime.Before(cutoff) {
				log.Printf("Deleting old resource group: %s", *group.Name)
				poller, err := clients.ResourceClient.BeginDelete(ctx, *group.Name, nil)
				if err != nil {
					log.Printf("Failed to delete resource group %s: %v", *group.Name, err)
					continue
				}
				_, err = poller.PollUntilDone(ctx, nil)
				if err != nil {
					log.Printf("Failed to complete deletion of resource group %s: %v", *group.Name, err)
				}
			}
		}
	}
	return nil
}
