package infra

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"shellbox/internal/sshutil"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/google/uuid"
)

const bastionSetupScript = `#!/bin/bash
sudo apt-get update -y
sudo apt-get upgrade -y

# Security hardening
ufw allow OpenSSH
ufw --force enable

# Bastion-specific setup
mkdir -p /etc/ssh/sshd_config.d/
echo "PermitUserEnvironment yes" > /etc/ssh/sshd_config.d/shellbox.conf
systemctl reload sshd

# Create shellbox directory
mkdir -p /home/\${admin_user}`

func GenerateBastionInitScript() (string, error) {
	fullScript := fmt.Sprintf(`#cloud-config
runcmd:
- %s`, bastionSetupScript)
	return base64.StdEncoding.EncodeToString([]byte(fullScript)), nil
}

// DefaultBastionConfig returns a default configuration for bastion deployment
func DefaultBastionConfig() *VMConfig {
	return &VMConfig{
		AdminUsername: "shellbox",
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
	ipPoller, err := clients.PublicIPClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, bastionIPName, armnetwork.PublicIPAddress{
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
	nicPoller, err := clients.NICClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, bastionNICName, armnetwork.Interface{
		Location: to.Ptr(location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(clients.BastionSubnetID),
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

func createBastionVM(ctx context.Context, clients *AzureClients, config *VMConfig, nicID string, customData string) (*armcompute.VirtualMachine, error) {
	vmPoller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, bastionVMName, armcompute.VirtualMachine{
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
					Publisher: to.Ptr(VMPublisher),
					Offer:     to.Ptr(VMOffer),
					SKU:       to.Ptr(VMSku),
					Version:   to.Ptr(VMVersion),
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

// copyServerBinary compiles and copies the server binary to the bastion host.
// Uses retry with timeout because the bastion may still be initializing and
// might not be ready to accept SSH connections immediately.
func copyServerBinary(ctx context.Context, config *VMConfig, publicIPAddress string) error {
	opts := DefaultRetryOptions()
	opts.Operation = "copy server binary to bastion"
	opts.Timeout = 5 * time.Minute // Longer timeout for file transfer

	// Copy file to bastion
	remotePath := fmt.Sprintf("/home/%s/server", config.AdminUsername)

	_, err := RetryWithTimeout(ctx, opts, func(ctx context.Context) (bool, error) {
		if err := sshutil.CopyFile(ctx, "/tmp/server", remotePath, config.AdminUsername, publicIPAddress); err != nil {
			return false, err
		}
		return true, nil
	})
	return err
}

// copyCosmosDBConfig creates a configuration file with CosmosDB connection string
// and copies it to the bastion host. This is called after copyServerBinary
// because we don't need to retry - if copyServerBinary succeeded, the bastion
// is already initialized and accepting connections.
func copyCosmosDBConfig(ctx context.Context, clients *AzureClients, config *VMConfig, publicIPAddress string) error {
	cosmosConfigContent := fmt.Sprintf(`{"connectionString": "%s"}`, clients.CosmosDBConnectionString)

	// Create temporary local file
	tempFile := "/tmp/cosmosdb.json"
	if err := os.WriteFile(tempFile, []byte(cosmosConfigContent), 0600); err != nil {
		return fmt.Errorf("failed to create temporary CosmosDB config file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up when done

	// Copy to bastion
	remoteConfigPath := fmt.Sprintf("/home/%s/%s", config.AdminUsername, cosmosdbConfigFile)
	if err := sshutil.CopyFile(ctx, tempFile, remoteConfigPath, config.AdminUsername, publicIPAddress); err != nil {
		return fmt.Errorf("failed to copy CosmosDB config to bastion: %w", err)
	}

	return nil
}

func startServerOnBastion(ctx context.Context, config *VMConfig, publicIPAddress string, resourceGroupSuffix string) error {
	opts := DefaultRetryOptions()
	opts.Operation = "start server on bastion"
	command := fmt.Sprintf("nohup /home/%s/server %s > /home/%s/server.log 2>&1 &", config.AdminUsername, resourceGroupSuffix, config.AdminUsername)
	_, err := RetryWithTimeout(ctx, opts, func(ctx context.Context) (bool, error) {
		if err := sshutil.ExecuteCommand(ctx, command, config.AdminUsername, publicIPAddress); err != nil {
			return false, err
		}
		return true, nil
	})
	return err
}

func getBastionRoleID(subscriptionID string) string {
	// Use built-in Contributor role which can manage all resources including disks
	return fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c", subscriptionID)
}

func assignRoleToVM(ctx context.Context, clients *AzureClients, principalID *string) error {
	roleDefID := getBastionRoleID(clients.SubscriptionID)
	guid := uuid.New().String()
	// Assign at subscription level to allow resource creation in any resource group
	scope := fmt.Sprintf("/subscriptions/%s", clients.SubscriptionID)

	opts := DefaultRetryOptions()
	opts.Operation = "assign role to bastion VM"
	opts.Timeout = 2 * time.Minute   // Longer timeout for AAD propagation
	opts.Interval = 10 * time.Second // Longer interval between retries

	_, err := RetryWithTimeout(ctx, opts, func(ctx context.Context) (bool, error) {
		_, err := clients.RoleClient.Create(ctx,
			scope,
			guid,
			armauthorization.RoleAssignmentCreateParameters{
				Properties: &armauthorization.RoleAssignmentProperties{
					PrincipalID:      principalID,
					RoleDefinitionID: to.Ptr(roleDefID),
				},
			}, nil)
		if err != nil {
			// Return false without error to trigger retry for PrincipalNotFound
			if strings.Contains(err.Error(), "PrincipalNotFound") {
				return false, nil
			}
			// Return actual error for other cases
			return false, fmt.Errorf("creating role assignment: %w", err)
		}
		return true, nil
	})

	return err
}

// DeployBastion creates a bastion host in the bastion subnet and returns its public IP
func DeployBastion(ctx context.Context, clients *AzureClients, config *VMConfig) string {
	if err := compileBastionServer(); err != nil {
		log.Fatalf("failed to compile server binary: %v", err)
	}

	publicIP, err := createBastionPublicIP(ctx, clients)
	if err != nil {
		log.Fatalf("failed to create public IP: %v", err)
	}

	nic, err := createBastionNIC(ctx, clients, publicIP.ID)
	if err != nil {
		log.Fatalf("failed to create NIC: %v", err)
	}

	customData, err := GenerateBastionInitScript()
	if err != nil {
		log.Fatalf("failed to generate init script: %v", err)
	}

	vm, err := createBastionVM(ctx, clients, config, *nic.ID, customData)
	if err != nil {
		log.Fatalf("failed to create bastion VM: %v", err)
	}

	if err := assignRoleToVM(ctx, clients, vm.Identity.PrincipalID); err != nil {
		log.Fatalf("failed to assign role to VM: %v", err)
	}
	if err := copyServerBinary(ctx, config, *publicIP.Properties.IPAddress); err != nil {
		log.Fatalf("failed to copy server binary: %v", err)
	}

	if err := copyCosmosDBConfig(ctx, clients, config, *publicIP.Properties.IPAddress); err != nil {
		log.Fatalf("failed to copy CosmosDB config: %v", err)
	}

	if err := startServerOnBastion(ctx, config, *publicIP.Properties.IPAddress, clients.ResourceGroupSuffix); err != nil {
		log.Fatalf("failed to start server on bastion: %v", err)
	}

	return *publicIP.Properties.IPAddress

}
