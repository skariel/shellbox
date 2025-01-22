package infra

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/google/uuid"
)

const (
	// Role definitions
	contributorRoleID = "/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c"
	readerRoleID      = "/providers/Microsoft.Authorization/roleDefinitions/acdd72a7-3385-48ef-bd42-f606fba81ae7"
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

// DeployBastion creates a bastion host in the bastion subnet
func DeployBastion(ctx context.Context, clients *AzureClients, config *BastionConfig) error {
	const (
		bastionVMName  = "shellbox-bastion"
		bastionNICName = "bastion-nic"
		bastionIPName  = "bastion-ip"
	)

	// Compile server binary
	if err := exec.Command("go", "build", "-o", "/tmp/server", "./cmd/server").Run(); err != nil {
		return fmt.Errorf("failed to compile server binary: %w", err)
	}

	// Get subscription ID early
	subscriptionID, err := getSubscriptionID()
	if err != nil {
		return fmt.Errorf("failed to get subscription ID: %w", err)
	}

	pollUntilDoneOption := runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	// Create public IP for bastion
	ipPoller, err := clients.PublicIPClient.BeginCreateOrUpdate(ctx, resourceGroupName, bastionIPName, armnetwork.PublicIPAddress{
		Location: to.Ptr(location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start public IP creation: %w", err)
	}
	publicIP, err := ipPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create public IP: %w", err)
	}

	// Create NIC for bastion

	bastionSubnetID, err := GetBastionSubnetID()
	if err != nil {
		return err
	}
	nicPoller, err := clients.NICClient.BeginCreateOrUpdate(ctx, resourceGroupName, bastionNICName, armnetwork.Interface{
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
							ID: publicIP.ID,
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start NIC creation: %w", err)
	}
	nic, err := nicPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create NIC: %w", err)
	}

	// Generate cloud-init script
	customData, err := GenerateBastionInitScript(config.SSHPublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate init script: %w", err)
	}

	// Create bastion VM
	vmPoller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, resourceGroupName, bastionVMName, armcompute.VirtualMachine{
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
						ID: nic.ID,
						Properties: &armcompute.NetworkInterfaceReferenceProperties{
							Primary: to.Ptr(true),
						},
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start bastion VM creation: %w", err)
	}
	vm, err := vmPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create bastion VM: %w", err)
	}
	if vm.Identity == nil || vm.Identity.PrincipalID == nil {
		return fmt.Errorf("VM managed identity not found after creation")
	}

	// Create role assignment for the VM's managed identity
	roleDefID := fmt.Sprintf("/subscriptions/%s%s", subscriptionID, contributorRoleID)
	guid := NewGUID()
	_, err = clients.RoleClient.Create(ctx,
		fmt.Sprintf("/subscriptions/%s/resourceGroups/%s", subscriptionID, resourceGroupName),
		guid,
		armauthorization.RoleAssignmentCreateParameters{
			Properties: &armauthorization.RoleAssignmentProperties{
				RoleDefinitionID: to.Ptr(roleDefID),
				PrincipalID:      vm.Identity.PrincipalID,
			},
		}, nil)
	if err != nil {
		return fmt.Errorf("failed to create role assignment: %w", err)
	}

	// Wait for SSH to be ready
	sshAddr := fmt.Sprintf("%s@%s", config.AdminUsername, *publicIP.Properties.IPAddress)
	if err := waitForSSH(sshAddr); err != nil {
		return fmt.Errorf("waiting for SSH: %w", err)
	}
	if err != nil {
		return fmt.Errorf("failed to establish ssh connection to bastion: %w", err)
	}

	// Copy server binary to bastion
	if err := exec.Command("scp", "-o", "StrictHostKeyChecking=no", "/tmp/server",
		fmt.Sprintf("%s@%s:\"/home/%s/server\"", config.AdminUsername, *publicIP.Properties.IPAddress, config.AdminUsername)).Run(); err != nil {
		scpCmd := fmt.Sprintf("scp -o StrictHostKeyChecking=no /tmp/server %s@%s:\"/home/%s/server\"",
			config.AdminUsername, *publicIP.Properties.IPAddress, config.AdminUsername)
		return fmt.Errorf("failed to copy server binary (cmd: %s): %w", scpCmd, err)
	}

	// Start the server via SSH
	if err := exec.Command("ssh",
		fmt.Sprintf("%s@%s", config.AdminUsername, *publicIP.Properties.IPAddress),
		fmt.Sprintf("nohup /home/%s/server > /home/%s/server.log 2>&1 &", config.AdminUsername, config.AdminUsername)).Run(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}

func waitForSSH(addr string) error {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for SSH")
		case <-ticker.C:
			cmd := exec.Command("ssh", addr, "echo test")
			if err := cmd.Run(); err == nil {
				return nil
			}
		}
	}
}
