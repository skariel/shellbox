package infra

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/google/uuid"
)

// InstanceTags represents searchable metadata for instance VMs.
// These tags are used to track VM status and lifecycle.
type InstanceTags struct {
	Role       string // instance
	Status     string // free, connected
	CreatedAt  string
	LastUsed   string
	InstanceID string
}

// CreateInstance creates a new instance VM with proper networking setup.
// This creates only the compute instance without any volumes or QEMU setup.
// Volumes will be attached separately when users connect.
// It returns the instance ID and any error encountered.
func CreateInstance(ctx context.Context, clients *AzureClients, config *VMConfig) (string, error) {
	instanceID := uuid.New().String()
	namer := NewResourceNamer(clients.Suffix)
	vmName := namer.BoxVMName(instanceID)
	nicName := namer.BoxNICName(instanceID)
	nsgName := namer.BoxNSGName(instanceID)

	// Create NSG for the instance
	nsg, err := createInstanceNSG(ctx, clients, nsgName)
	if err != nil {
		return "", fmt.Errorf("creating instance NSG: %w", err)
	}

	// Create NIC with the NSG
	nic, err := createInstanceNIC(ctx, clients, nicName, nsg.ID)
	if err != nil {
		return "", fmt.Errorf("creating instance NIC: %w", err)
	}

	// Create the VM (instance only, no volumes)
	now := time.Now().UTC()
	tags := InstanceTags{
		Role:       ResourceRoleInstance,
		Status:     ResourceStatusFree,
		CreatedAt:  now.Format(time.RFC3339),
		LastUsed:   now.Format(time.RFC3339),
		InstanceID: instanceID,
	}

	_, err = createInstanceVM(ctx, clients, vmName, *nic.ID, config, tags)
	if err != nil {
		return "", fmt.Errorf("creating instance VM: %w", err)
	}

	return instanceID, nil
}

func createInstanceNSG(ctx context.Context, clients *AzureClients, nsgName string) (*armnetwork.SecurityGroup, error) {

	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(Location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("AllowICMPFromBastion"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolIcmp),
						SourceAddressPrefix:      to.Ptr(bastionSubnetCIDR),
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
						SourceAddressPrefix:      to.Ptr(bastionSubnetCIDR),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr("22"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr(int32(110)),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
				{
					Name: to.Ptr("AllowBoxSSHFromBastion"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						SourceAddressPrefix:      to.Ptr(bastionSubnetCIDR),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr(fmt.Sprintf("%d", BoxSSHPort)),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr(int32(111)),
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
					Name: to.Ptr("DenyBoxesSubnet"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
						SourceAddressPrefix:      to.Ptr("*"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr(boxesSubnetCIDR),
						DestinationPortRange:     to.Ptr("*"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
						Priority:                 to.Ptr(int32(100)),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
				{
					Name: to.Ptr("DenyBastionSubnet"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
						SourceAddressPrefix:      to.Ptr("*"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr(bastionSubnetCIDR),
						DestinationPortRange:     to.Ptr("*"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
						Priority:                 to.Ptr(int32(110)),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
					},
				},
				{
					Name: to.Ptr("AllowInternetOutbound"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
						SourceAddressPrefix:      to.Ptr("*"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("Internet"),
						DestinationPortRange:     to.Ptr("*"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr(int32(4000)),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
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

func createInstanceNIC(ctx context.Context, clients *AzureClients, nicName string, nsgID *string) (*armnetwork.Interface, error) {
	nicParams := armnetwork.Interface{
		Location: to.Ptr(Location),
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

func createInstanceVM(ctx context.Context, clients *AzureClients, vmName string, nicID string, config *VMConfig, tags InstanceTags) (*armcompute.VirtualMachine, error) {
	namer := NewResourceNamer(clients.Suffix)
	tagsMap := map[string]*string{
		TagKeyRole:     to.Ptr(tags.Role),
		TagKeyStatus:   to.Ptr(tags.Status),
		TagKeyCreated:  to.Ptr(tags.CreatedAt),
		TagKeyLastUsed: to.Ptr(tags.LastUsed),
		"instance_id":  to.Ptr(tags.InstanceID),
	}

	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(Location),
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
					Name:         to.Ptr(namer.BoxOSDiskName(tags.InstanceID)),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(namer.BoxComputerName(tags.InstanceID)),
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

// FindInstancesByStatus returns instance IDs matching the given status.
// It filters VMs based on their status tag and returns their resource IDs.
func FindInstancesByStatus(ctx context.Context, clients *AzureClients, status string) ([]string, error) {
	filter := fmt.Sprintf("tagName eq 'status' and tagValue eq '%s'", status)

	pager := clients.ComputeClient.NewListPager(clients.ResourceGroupName, &armcompute.VirtualMachinesClientListOptions{
		Filter: &filter,
	})
	var instances []string

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing VMs: %w", err)
		}

		for _, vm := range page.Value {
			if vm.Tags != nil && vm.Tags[TagKeyStatus] != nil && *vm.Tags[TagKeyStatus] == status {
				instances = append(instances, *vm.ID)
			}
		}
	}

	return instances, nil
}

// instanceResourceInfo holds information about resources associated with an instance
type instanceResourceInfo struct {
	instanceID   string
	nicID        string
	nicName      string
	nsgName      string
	osDiskName   string
	dataDiskName string
}

// DeleteInstance completely removes an instance VM and all its associated resources.
// This includes the VM, its OS disk, data disk (if any), NIC, and NSG.
// This function is used for both temporary cleanup and pool shrinking operations.
func DeleteInstance(ctx context.Context, clients *AzureClients, resourceGroupName, vmName string) error {
	// Get VM and extract resource information
	vm, err := clients.ComputeClient.Get(ctx, resourceGroupName, vmName, nil)
	if err != nil {
		log.Printf("VM %s not found, proceeding with cleanup of other resources: %v", vmName, err)
	}

	// Extract resource information from VM or generate from naming patterns
	resourceInfo := extractInstanceResourceInfo(vm, vmName, resourceGroupName, err == nil)

	log.Printf("Deleting box %s with resources: NIC=%s, NSG=%s, OSDisk=%s, DataDisk=%s",
		vmName, resourceInfo.nicName, resourceInfo.nsgName, resourceInfo.osDiskName, resourceInfo.dataDiskName)

	// Delete resources in order: VM, data disk, OS disk, NIC, NSG
	deleteVM(ctx, clients, resourceGroupName, vmName, err == nil)
	deleteDisk(ctx, clients, resourceGroupName, resourceInfo.dataDiskName, "data disk")
	deleteDisk(ctx, clients, resourceGroupName, resourceInfo.osDiskName, "OS disk")
	deleteNIC(ctx, clients, resourceGroupName, resourceInfo.nicName, resourceInfo.nicID)
	deleteNSG(ctx, clients, resourceGroupName, resourceInfo.nsgName)

	log.Printf("Box deletion completed: %s", vmName)
	return nil
}

// extractInstanceResourceInfo extracts resource information from VM or generates from naming patterns
func extractInstanceResourceInfo(vm armcompute.VirtualMachinesClientGetResponse, vmName, resourceGroupName string, vmExists bool) instanceResourceInfo {
	info := instanceResourceInfo{}

	if vmExists && vm.Properties != nil {
		extractResourcesFromVM(&info, vm)
	}

	// Extract instance ID from VM name if not found in tags
	if info.instanceID == "" {
		info.instanceID = extractInstanceIDFromVMName(vmName)
	}

	// Generate missing resource names using naming patterns
	generateMissingResourceNames(&info, resourceGroupName)

	return info
}

// extractResourcesFromVM extracts resource information from VM properties
func extractResourcesFromVM(info *instanceResourceInfo, vm armcompute.VirtualMachinesClientGetResponse) {
	// Extract instance ID from tags
	if vm.Tags != nil && vm.Tags["instance_id"] != nil {
		info.instanceID = *vm.Tags["instance_id"]
	}

	// Get NIC ID
	if vm.Properties.NetworkProfile != nil && len(vm.Properties.NetworkProfile.NetworkInterfaces) > 0 {
		info.nicID = *vm.Properties.NetworkProfile.NetworkInterfaces[0].ID
	}

	// Get disk names from storage profile
	if vm.Properties.StorageProfile != nil {
		if vm.Properties.StorageProfile.OSDisk != nil {
			info.osDiskName = *vm.Properties.StorageProfile.OSDisk.Name
		}
		if len(vm.Properties.StorageProfile.DataDisks) > 0 {
			info.dataDiskName = *vm.Properties.StorageProfile.DataDisks[0].Name
		}
	}
}

// extractInstanceIDFromVMName extracts instance ID from VM name using naming pattern
func extractInstanceIDFromVMName(vmName string) string {
	parts := strings.Split(vmName, "-")
	if len(parts) >= 4 {
		return parts[len(parts)-2]
	}
	return ""
}

// generateMissingResourceNames generates missing resource names using naming patterns
func generateMissingResourceNames(info *instanceResourceInfo, resourceGroupName string) {
	if info.instanceID == "" {
		return
	}

	namer := NewResourceNamer(extractSuffix(resourceGroupName))
	info.nicName = namer.BoxNICName(info.instanceID)
	info.nsgName = namer.BoxNSGName(info.instanceID)

	if info.osDiskName == "" {
		info.osDiskName = namer.BoxOSDiskName(info.instanceID)
	}
	if info.dataDiskName == "" {
		info.dataDiskName = namer.BoxDataDiskName(info.instanceID)
	}
}

// deleteVM deletes a virtual machine
func deleteVM(ctx context.Context, clients *AzureClients, resourceGroupName, vmName string, vmExists bool) {
	if !vmExists {
		return
	}

	log.Printf("Deleting VM: %s", vmName)
	vmDelete, err := clients.ComputeClient.BeginDelete(ctx, resourceGroupName, vmName, nil)
	if err != nil {
		log.Printf("Failed to start VM deletion %s: %v", vmName, err)
		return
	}

	_, err = vmDelete.PollUntilDone(ctx, nil)
	if err != nil {
		log.Printf("Failed waiting for VM deletion %s: %v", vmName, err)
	} else {
		log.Printf("Successfully deleted VM: %s", vmName)
	}
}

// deleteDisk deletes a disk (OS or data disk)
func deleteDisk(ctx context.Context, clients *AzureClients, resourceGroupName, diskName, diskType string) {
	if diskName == "" {
		return
	}

	log.Printf("Deleting %s: %s", diskType, diskName)
	diskDelete, err := clients.DisksClient.BeginDelete(ctx, resourceGroupName, diskName, nil)
	if err != nil {
		log.Printf("Failed to start %s deletion %s: %v", diskType, diskName, err)
		return
	}

	_, err = diskDelete.PollUntilDone(ctx, nil)
	if err != nil {
		log.Printf("Failed waiting for %s deletion %s: %v", diskType, diskName, err)
	} else {
		log.Printf("Successfully deleted %s: %s", diskType, diskName)
	}
}

// deleteNIC deletes a network interface
func deleteNIC(ctx context.Context, clients *AzureClients, resourceGroupName, nicName, nicID string) {
	targetNICName := nicName
	if targetNICName == "" && nicID != "" {
		parts := strings.Split(nicID, "/")
		targetNICName = parts[len(parts)-1]
	}

	if targetNICName == "" {
		return
	}

	log.Printf("Deleting NIC: %s", targetNICName)
	nicDelete, err := clients.NICClient.BeginDelete(ctx, resourceGroupName, targetNICName, nil)
	if err != nil {
		log.Printf("Failed to start NIC deletion %s: %v", targetNICName, err)
		return
	}

	_, err = nicDelete.PollUntilDone(ctx, nil)
	if err != nil {
		log.Printf("Failed waiting for NIC deletion %s: %v", targetNICName, err)
	} else {
		log.Printf("Successfully deleted NIC: %s", targetNICName)
	}
}

// deleteNSG deletes a network security group
func deleteNSG(ctx context.Context, clients *AzureClients, resourceGroupName, nsgName string) {
	if nsgName == "" {
		return
	}

	log.Printf("Deleting NSG: %s", nsgName)
	nsgDelete, err := clients.NSGClient.BeginDelete(ctx, resourceGroupName, nsgName, nil)
	if err != nil {
		log.Printf("Failed to start NSG deletion %s: %v", nsgName, err)
		return
	}

	_, err = nsgDelete.PollUntilDone(ctx, nil)
	if err != nil {
		log.Printf("Failed waiting for NSG deletion %s: %v", nsgName, err)
	} else {
		log.Printf("Successfully deleted NSG: %s", nsgName)
	}
}
