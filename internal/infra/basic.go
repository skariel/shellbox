package infra

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
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

// VMConfig holds common VM configuration fields
type VMConfig struct {
	AdminUsername string
	SSHPublicKey  string
	VMSize        string
}

// InfrastructureIDs holds infrastructure resource identifiers
type InfrastructureIDs struct {
	resourceGroupName string
	bastionSubnetID   string
	boxesSubnetID     string
}

// NewInfrastructureIDs creates a new infrastructure IDs instance
func NewInfrastructureIDs() *InfrastructureIDs {
	return &InfrastructureIDs{}
}

// AzureClients holds all the Azure SDK clients needed for the application
type AzureClients struct {
	cred           *azidentity.ManagedIdentityCredential
	subscriptionID string
	infraIDs       *InfrastructureIDs
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
func (c *AzureClients) GetResourceGroupName() string {
	if c.infraIDs.resourceGroupName == "" {
		c.infraIDs.resourceGroupName = fmt.Sprintf("%s-%d", resourceGroupPrefix, time.Now().Unix())
	}
	return c.infraIDs.resourceGroupName
}

func getSubscriptionIDFromMetadata() (string, error) {
	req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/instance/compute/subscriptionId?api-version=2021-02-01&format=text", nil)
	if err != nil {
		return "", fmt.Errorf("creating metadata request: %w", err)
	}
	req.Header.Add("Metadata", "true")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("getting subscription ID from metadata: %w", err)
	}
	defer resp.Body.Close()

	subscriptionIDBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading subscription ID: %w", err)
	}
	if len(subscriptionIDBytes) == 0 {
		return "", fmt.Errorf("empty subscription ID from metadata service")
	}
	return string(subscriptionIDBytes), nil
}

func initializeAzureClients(subscriptionID string, cred *azidentity.ManagedIdentityCredential) (*AzureClients, error) {
	resourceClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource group client: %w", err)
	}

	networkClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create network client: %w", err)
	}

	nsgClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG client: %w", err)
	}

	publicIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Public IP client: %w", err)
	}

	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create interfaces client: %w", err)
	}

	computeClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create compute client: %w", err)
	}

	cosmosClient, err := armcosmos.NewDatabaseAccountsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client: %w", err)
	}

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
		cred:           cred,
		subscriptionID: subscriptionID,
		infraIDs:       NewInfrastructureIDs(),
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

// NewAzureClients creates and returns all necessary Azure clients
func NewAzureClients() (*AzureClients, error) {
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	subscriptionID, err := getSubscriptionIDFromMetadata()
	if err != nil {
		return nil, err
	}

	return initializeAzureClients(subscriptionID, cred)
}

// GetBastionSubnetID returns the ID of the bastion subnet
func (c *AzureClients) GetBastionSubnetID() (string, error) {
	if c.infraIDs.bastionSubnetID != "" {
		return c.infraIDs.bastionSubnetID, nil
	}
	return "", errors.New("could not find BastionSubnetID")
}

// GetBoxesSubnetID returns the ID of the boxes subnet
func (c *AzureClients) GetBoxesSubnetID() (string, error) {
	if c.infraIDs.boxesSubnetID != "" {
		return c.infraIDs.boxesSubnetID, nil
	}
	return "", errors.New("could not find BoxesSubnetID")
}

// CreateNetworkInfrastructure sets up the basic network infrastructure in Azure
func CreateNetworkInfrastructure(ctx context.Context, clients *AzureClients) error {
	pollUntilDoneOption := runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	rgName := clients.GetResourceGroupName()
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
			clients.infraIDs.bastionSubnetID = *subnet.ID
		case boxesSubnetName:
			clients.infraIDs.boxesSubnetID = *subnet.ID
		}
	}
	if clients.infraIDs.bastionSubnetID == "" {
		return fmt.Errorf("bastion subnet not found in VNet")
	}
	if clients.infraIDs.boxesSubnetID == "" {
		return fmt.Errorf("boxes subnet not found in VNet")
	}

	return nil
}

// GetSubscriptionID returns the stored subscription ID
func (c *AzureClients) GetSubscriptionID() string {
	return c.subscriptionID
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
