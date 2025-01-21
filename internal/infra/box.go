package infra

import (
	"context"
	"fmt"
	"time"
	
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
)

// BoxConfig holds the configuration for a box VM
type BoxConfig struct {
	VMSize        string
	AdminUsername string
	SSHPublicKey  string
}

// BoxTags represents searchable metadata for box VMs
type BoxTags struct {
	Status    string // ready, allocated, deallocated
	CreatedAt string
	BoxID     string
}

// CreateBox creates a new box VM with proper networking setup
func CreateBox(ctx context.Context, clients *AzureClients, config *BoxConfig) (string, error) {
	boxID := NewGUID()
	vmName := fmt.Sprintf("box-%s", boxID)
	nicName := fmt.Sprintf("box-nic-%s", boxID)
	nsgName := fmt.Sprintf("box-nsg-%s", boxID)

	// Create NSG for the box
	nsg, err := createBoxNSG(ctx, clients, nsgName)
	if err != nil {
		return "", fmt.Errorf("creating box NSG: %w", err)
	}

	// Create NIC with the NSG
	nic, err := createBoxNIC(ctx, clients, nicName, nsg.ID)
	if err != nil {
		return "", fmt.Errorf("creating box NIC: %w", err)
	}

	// Create the VM
	tags := BoxTags{
		Status:    "ready",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		BoxID:     boxID,
	}

	vm, err := createBoxVM(ctx, clients, vmName, *nic.ID, config, tags)
	if err != nil {
		return "", fmt.Errorf("creating box VM: %w", err)
	}

	return *vm.ID, nil
}

func createBoxNSG(ctx context.Context, clients *AzureClients, nsgName string) (*armnetwork.SecurityGroup, error) {
	bastionSubnet := "10.0.0.0/24"
	
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("AllowSSHFromBastion"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						SourceAddressPrefix:      to.Ptr(bastionSubnet),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr("22"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr(int32(100)),
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
			},
		},
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, resourceGroupName, nsgName, nsgParams, nil)
	if err != nil {
		return nil, err
	}

	nsg, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &nsg.SecurityGroup, nil
}

func createBoxNIC(ctx context.Context, clients *AzureClients, nicName string, nsgID *string) (*armnetwork.Interface, error) {
	boxesSubnetID := "" // TODO: implement GetBoxesSubnetID() similar to GetBastionSubnetID()
	
	nicParams := armnetwork.Interface{
		Location: to.Ptr(location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsgID,
			},
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(boxesSubnetID),
						},
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
					},
				},
			},
		},
	}

	poller, err := clients.NICClient.BeginCreateOrUpdate(ctx, resourceGroupName, nicName, nicParams, nil)
	if err != nil {
		return nil, err
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &result.Interface, nil
}

func createBoxVM(ctx context.Context, clients *AzureClients, vmName string, nicID string, config *BoxConfig, tags BoxTags) (*armcompute.VirtualMachine, error) {
	tagsMap := map[string]*string{
		"status":     to.Ptr(tags.Status),
		"created_at": to.Ptr(tags.CreatedAt),
		"box_id":     to.Ptr(tags.BoxID),
	}

	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Tags:     tagsMap,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(config.VMSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr("Canonical"),
					Offer:     to.Ptr("UbuntuServer"),
					Sku:       to.Ptr("18.04-LTS"),
					Version:   to.Ptr("latest"),
				},
				OSDisk: &armcompute.OSDisk{
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesStandardLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(vmName),
				AdminUsername: to.Ptr(config.AdminUsername),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", config.AdminUsername)),
								KeyData: to.Ptr(config.SSHPublicKey),
							},
						},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(nicID),
					},
				},
			},
		},
	}

	poller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, vmParams, nil)
	if err != nil {
		return nil, err
	}

	result, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}

	return &result.VirtualMachine, nil
}

// DeallocateBox deallocates a box VM
func DeallocateBox(ctx context.Context, clients *AzureClients, vmID string) error {
	poller, err := clients.ComputeClient.BeginDeallocate(ctx, resourceGroupName, vmID, nil)
	if err != nil {
		return err
	}

	_, err = poller.PollUntilDone(ctx, nil)
	return err
}

// FindBoxesByStatus returns box IDs matching the given status
func FindBoxesByStatus(ctx context.Context, clients *AzureClients, status string) ([]string, error) {
	filter := fmt.Sprintf("tagName eq 'status' and tagValue eq '%s'", status)
	
	pager := clients.ComputeClient.NewListPager(resourceGroupName, nil)
	var boxes []string

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, vm := range page.Value {
			if vm.Tags != nil && *vm.Tags["status"] == status {
				boxes = append(boxes, *vm.ID)
			}
		}
	}

	return boxes, nil
}
