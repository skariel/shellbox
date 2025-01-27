package infra

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/google/uuid"
)

// BoxTags represents searchable metadata for box VMs.
// These tags are used to track VM status and lifecycle.
type BoxTags struct {
	Status    string // ready, allocated, deallocated
	CreatedAt string
	BoxID     string
}

// CreateBox creates a new box VM with proper networking setup.
// It returns the box ID and any error encountered.
func CreateBox(ctx context.Context, clients *AzureClients, config *VMConfig) (string, error) {
	boxID := uuid.New().String()
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

	_, err = createBoxVM(ctx, clients, vmName, *nic.ID, config, tags)
	if err != nil {
		return "", fmt.Errorf("creating box VM: %w", err)
	}

	return boxID, nil
}

func createBoxNSG(ctx context.Context, clients *AzureClients, nsgName string) (*armnetwork.SecurityGroup, error) {
	bastionSubnet := "10.0.0.0/24"

	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("AllowICMPFromBastion"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolIcmp),
						SourceAddressPrefix:      to.Ptr(bastionSubnet),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr("*"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr(int32(100)),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				{
					Name: to.Ptr("AllowSSHFromBastion"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						SourceAddressPrefix:      to.Ptr(bastionSubnet),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr("22"),
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
			},
		},
	}

	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, nsgName, nsgParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting NSG creation: %w", err)
	}

	nsg, err := poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating NSG: %w", err)
	}

	return &nsg.SecurityGroup, nil
}

func createBoxNIC(ctx context.Context, clients *AzureClients, nicName string, nsgID *string) (*armnetwork.Interface, error) {
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
							ID: to.Ptr(clients.BoxesSubnetID),
						},
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
					},
				},
			},
		},
	}

	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.NICClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, nicName, nicParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting NIC creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating NIC: %w", err)
	}

	return &result.Interface, nil
}

func createBoxVM(ctx context.Context, clients *AzureClients, vmName string, nicID string, config *VMConfig, tags BoxTags) (*armcompute.VirtualMachine, error) {
	tagsMap := map[string]*string{
		"status":     to.Ptr(tags.Status),
		"created_at": to.Ptr(tags.CreatedAt),
		"box_id":     to.Ptr(tags.BoxID),
	}

	// Generate base initialization script
	initScript, err := GenerateBoxInitScript()
	if err != nil {
		return nil, fmt.Errorf("failed to generate box init script: %w", err)
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
					Publisher: to.Ptr(VMPublisher),
					Offer:     to.Ptr(VMOffer),
					SKU:       to.Ptr(VMSku),
					Version:   to.Ptr(VMVersion),
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
				CustomData:    to.Ptr(initScript),
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

	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, vmName, vmParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting VM creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating VM: %w", err)
	}

	return &result.VirtualMachine, nil
}

// DeallocateBox deallocates a box VM.
// It stops the VM and releases compute resources while preserving the VM configuration.
func DeallocateBox(ctx context.Context, clients *AzureClients, vmID string) error {
	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.ComputeClient.BeginDeallocate(ctx, clients.ResourceGroupName, vmID, nil)
	if err != nil {
		return fmt.Errorf("starting VM deallocation: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return fmt.Errorf("deallocating VM: %w", err)
	}
	return nil
}

// FindBoxesByStatus returns box IDs matching the given status.
// It filters VMs based on their status tag and returns their resource IDs.
func FindBoxesByStatus(ctx context.Context, clients *AzureClients, status string) ([]string, error) {
	filter := fmt.Sprintf("tagName eq 'status' and tagValue eq '%s'", status)

	pager := clients.ComputeClient.NewListPager(clients.ResourceGroupName, &armcompute.VirtualMachinesClientListOptions{
		Filter: &filter,
	})
	var boxes []string

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing VMs: %w", err)
		}

		for _, vm := range page.Value {
			if vm.Tags != nil && *vm.Tags["status"] == status {
				boxes = append(boxes, *vm.ID)
			}
		}
	}

	return boxes, nil
}
