package infra

import (
	"context"
	"fmt"
	"os/exec"
	"shellbox/internal/ssh"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/google/uuid"
)

// NewGUID generates a new GUID string
func NewGUID() string {
	return uuid.New().String()
}

// BastionConfig holds configuration for bastion deployment
type BastionConfig struct {
	AdminUsername string
	SSHPublicKey  string
	VMSize        string
}

// DefaultBastionConfig returns a default configuration for bastion deployment
func DefaultBastionConfig() *BastionConfig {
	return &BastionConfig{
		AdminUsername: "shellboxadmin",
		VMSize:        string(armcompute.VirtualMachineSizeTypesStandardD2SV3),
	}
}

var defaultPollOptions = runtime.PollUntilDoneOptions{
	Frequency: 2 * time.Second,
}

func compileBastionServer() error {
	if err := exec.Command("go", "build", "-o", "/tmp/server", "./cmd/server").Run(); err != nil {
		return fmt.Errorf("failed to compile server binary: %w", err)
	}
	return nil
}

func createBastionPublicIP(ctx context.Context, clients *AzureClients) (*armnetwork.PublicIPAddress, error) {
	ipPoller, err := clients.PublicIPClient.BeginCreateOrUpdate(ctx, clients.GetResourceGroupName(), bastionIPName, armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start public IP creation: %w", err)
	}
	res, err := ipPoller.PollUntilDone(ctx, &defaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to poll IP creation: %w", err)
	}
	return &res.PublicIPAddress, nil

}

func createBastionNIC(ctx context.Context, clients *AzureClients, publicIPID *string) (*armnetwork.Interface, error) {
	bastionSubnetID, err := clients.GetBastionSubnetID()
	if err != nil {
		return nil, err
	}

	nicPoller, err := clients.NICClient.BeginCreateOrUpdate(ctx, clients.GetResourceGroupName(), bastionNICName, armnetwork.Interface{
		Location: to.Ptr(location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(bastionSubnetID),
						},
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
						PublicIPAddress: &armnetwork.PublicIPAddress{
							ID: publicIPID,
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start NIC creation: %w", err)
	}
	res, err := nicPoller.PollUntilDone(ctx, &defaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to poll NIC creation: %w", err)
	}
	return &res.Interface, nil
}

func createBastionVM(ctx context.Context, clients *AzureClients, config *BastionConfig, nicID string, customData string) (*armcompute.VirtualMachine, error) {
	vmPoller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, clients.GetResourceGroupName(), bastionVMName, armcompute.VirtualMachine{
		Location: to.Ptr(location),
		Identity: &armcompute.VirtualMachineIdentity{
			Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssigned),
		},
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(config.VMSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr("Canonical"),
					Offer:     to.Ptr("0001-com-ubuntu-server-jammy"),
					SKU:       to.Ptr("22_04-lts-gen2"),
					Version:   to.Ptr("latest"),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr("bastion-os-disk"),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr("shellbox-bastion"),
				AdminUsername: to.Ptr(config.AdminUsername),
				CustomData:    to.Ptr(customData),
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
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							Primary: to.Ptr(true),
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start bastion VM creation: %w", err)
	}

	vm, err := vmPoller.PollUntilDone(ctx, &defaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create bastion VM: %w", err)
	}

	if vm.Identity == nil || vm.Identity.PrincipalID == nil {
		return nil, fmt.Errorf("VM managed identity not found after creation")
	}

	return &vm.VirtualMachine, nil
}

func copyServerBinary(config *BastionConfig, publicIPAddress string) error {
	remotePath := fmt.Sprintf("/home/%s/server", config.AdminUsername)
	return ssh.CopyFile("/tmp/server", remotePath, config.AdminUsername, publicIPAddress)
}

func startServerOnBastion(config *BastionConfig, publicIPAddress string) error {
	command := fmt.Sprintf("nohup /home/%s/server > /home/%s/server.log 2>&1 &", config.AdminUsername, config.AdminUsername)
	return ssh.ExecuteCommand(command, config.AdminUsername, publicIPAddress)
}

func getBastionRoleID(subscriptionID string) string {
	// Use built-in Contributor role which can manage all resources including disks
	return fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c", subscriptionID)
}

func assignRoleToVM(ctx context.Context, clients *AzureClients, principalID *string) error {
	subscriptionID := clients.GetSubscriptionID()

	roleDefID := getBastionRoleID(subscriptionID)

	retryTimeout := time.After(2 * time.Minute)
	retryTicker := time.NewTicker(10 * time.Second)
	defer retryTicker.Stop()

	var lastErr error
	for {
		select {
		case <-retryTimeout:
			if lastErr != nil {
				return fmt.Errorf("timeout waiting for role assignment: %w", lastErr)
			}
			return fmt.Errorf("timeout waiting for role assignment")
		case <-retryTicker.C:
			guid := NewGUID()
			_, err := clients.RoleClient.Create(ctx,
				fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", subscriptionID, clients.GetResourceGroupName()),
				guid,
				armauthorization.RoleAssignmentCreateParameters{
					Properties: &armauthorization.RoleAssignmentProperties{
						PrincipalID:      principalID,
						RoleDefinitionID: to.Ptr(roleDefID),
					},
				}, nil)
			if err == nil {
				return nil
			}
			lastErr = err
		}
	}
}

// DeployBastion creates a bastion host in the bastion subnet
func DeployBastion(ctx context.Context, clients *AzureClients, config *BastionConfig) error {
	if err := compileBastionServer(); err != nil {
		return err
	}

	publicIP, err := createBastionPublicIP(ctx, clients)
	if err != nil {
		return fmt.Errorf("failed to create public IP: %w", err)
	}

	nic, err := createBastionNIC(ctx, clients, publicIP.ID)
	if err != nil {
		return fmt.Errorf("failed to create NIC: %w", err)
	}

	customData, err := GenerateBastionInitScript(config.SSHPublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate init script: %w", err)
	}

	vm, err := createBastionVM(ctx, clients, config, *nic.ID, customData)
	if err != nil {
		return err
	}

	if err := copyServerBinary(config, *publicIP.Properties.IPAddress); err != nil {
		return err
	}

	if err := startServerOnBastion(config, *publicIP.Properties.IPAddress); err != nil {
		return err
	}

	return assignRoleToVM(ctx, clients, vm.Identity.PrincipalID)
}
