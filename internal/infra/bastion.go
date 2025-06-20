package infra

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"shellbox/internal/sshutil"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
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

func compileBastionServer() error {
	if err := exec.Command("go", "build", "-o", "/tmp/server", "./cmd/server").Run(); err != nil {
		return fmt.Errorf("failed to compile server binary: %w", err)
	}
	return nil
}

func CreateBastionPublicIP(ctx context.Context, clients *AzureClients) (*armnetwork.PublicIPAddress, error) {
	namer := NewResourceNamer(clients.Suffix)
	ipPoller, err := clients.PublicIPClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionPublicIPName(), armnetwork.PublicIPAddress{
		Location: to.Ptr(Location),
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
	res, err := ipPoller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to poll IP creation: %w", err)
	}
	return &res.PublicIPAddress, nil
}

func CreateBastionNIC(ctx context.Context, clients *AzureClients, publicIPID *string) (*armnetwork.Interface, error) {
	namer := NewResourceNamer(clients.Suffix)
	nicPoller, err := clients.NICClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionNICName(), armnetwork.Interface{
		Location: to.Ptr(Location),
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
	res, err := nicPoller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to poll NIC creation: %w", err)
	}
	return &res.Interface, nil
}

func CreateBastionVM(ctx context.Context, clients *AzureClients, config *VMConfig, nicID string, customData string) (*armcompute.VirtualMachine, error) {
	namer := NewResourceNamer(clients.Suffix)
	vmPoller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionVMName(), armcompute.VirtualMachine{
		Location: to.Ptr(Location),
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
					Name:         to.Ptr(namer.BastionOSDiskName()),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(namer.BastionComputerName()),
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

	vm, err := vmPoller.PollUntilDone(ctx, &DefaultPollOptions)
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
	remotePath := fmt.Sprintf("/home/%s/server", config.AdminUsername)
	return RetryOperation(ctx, func(ctx context.Context) error {
		return sshutil.CopyFile(ctx, "/tmp/server", remotePath, config.AdminUsername, publicIPAddress)
	}, 5*time.Minute, 5*time.Second, "copy server binary to bastion")
}

// copyTableStorageConfig creates a configuration file with Table Storage connection string
// and copies it to the bastion host. This is called after copyServerBinary
// because we don't need to retry - if copyServerBinary succeeded, the bastion
// is already initialized and accepting connections.
func copyTableStorageConfig(ctx context.Context, clients *AzureClients, config *VMConfig, publicIPAddress string) error {
	tableStorageConfigContent := fmt.Sprintf(`{"connectionString": "%s"}`, clients.TableStorageConnectionString)

	// Create temporary local file
	tempFile := TempConfigPath
	if err := os.WriteFile(tempFile, []byte(tableStorageConfigContent), 0o600); err != nil {
		return fmt.Errorf("failed to create temporary Table Storage config file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up when done

	// Copy to bastion
	remoteConfigPath := fmt.Sprintf("/home/%s/%s", config.AdminUsername, tableStorageConfigFile)
	if err := sshutil.CopyFile(ctx, tempFile, remoteConfigPath, config.AdminUsername, publicIPAddress); err != nil {
		return fmt.Errorf("failed to copy Table Storage config to bastion: %w", err)
	}

	return nil
}

// copySSHKeyToBastion copies the SSH key to the bastion host
func copySSHKeyToBastion(ctx context.Context, config *VMConfig, bastionIP, privateKey string) error {
	slog.Info("Copying SSH key to bastion", "ip", bastionIP)

	// Create a temporary file for the private key
	tmpFile, err := os.CreateTemp("", "bastion_ssh_key")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(privateKey); err != nil {
		return fmt.Errorf("failed to write SSH key to temp file: %w", err)
	}
	tmpFile.Close()

	// Set correct permissions on temp file
	if err := os.Chmod(tmpFile.Name(), 0o600); err != nil {
		return fmt.Errorf("failed to set permissions on temp file: %w", err)
	}

	// Create the .ssh directory on bastion
	createDirCmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", config.AdminUsername, bastionIP),
		"mkdir", "-p", "/home/shellbox/.ssh")

	if output, err := createDirCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w: %s", err, string(output))
	}

	// Copy the SSH key to bastion
	scpCmd := exec.CommandContext(ctx, "scp",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		tmpFile.Name(),
		fmt.Sprintf("%s@%s:/home/shellbox/.ssh/id_rsa", config.AdminUsername, bastionIP))

	if output, err := scpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to copy SSH key: %w: %s", err, string(output))
	}

	// Set correct permissions on the key file
	chmodCmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", config.AdminUsername, bastionIP),
		"chmod", "600", "/home/shellbox/.ssh/id_rsa")

	if output, err := chmodCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set SSH key permissions: %w: %s", err, string(output))
	}

	// Set ownership to shellbox user
	chownCmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", config.AdminUsername, bastionIP),
		"sudo", "chown", "shellbox:shellbox", "/home/shellbox/.ssh/id_rsa")

	if output, err := chownCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set SSH key ownership: %w: %s", err, string(output))
	}

	slog.Info("SSH key copied to bastion successfully")
	return nil
}

func startServerOnBastion(ctx context.Context, config *VMConfig, publicIPAddress string, resourceGroupSuffix string) error {
	command := fmt.Sprintf("nohup /home/%s/server %s > /home/%s/server.log 2>&1 &", config.AdminUsername, resourceGroupSuffix, config.AdminUsername)
	return RetryOperation(ctx, func(ctx context.Context) error {
		return sshutil.ExecuteCommand(ctx, command, config.AdminUsername, publicIPAddress)
	}, 2*time.Minute, 5*time.Second, "start server on bastion")
}

func getBastionRoleID(subscriptionID string) string {
	// Use built-in Contributor role which can manage all resources including disks
	return fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c", subscriptionID)
}

func assignRoleToVM(ctx context.Context, clients *AzureClients, principalID *string) error {
	roleDefID := getBastionRoleID(clients.SubscriptionID)
	guid := uuid.New().String()
	scope := fmt.Sprintf("/subscriptions/%s", clients.SubscriptionID)

	return RetryOperation(ctx, func(ctx context.Context) error {
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
			// Treat PrincipalNotFound as retryable without logging error
			if strings.Contains(err.Error(), "PrincipalNotFound") {
				return fmt.Errorf("principal not found, retrying")
			}
			return fmt.Errorf("creating role assignment: %w", err)
		}
		return nil
	}, 2*time.Minute, 10*time.Second, "assign role to bastion VM")
}

// DeployBastion creates a bastion host in the bastion subnet and returns its public IP
func DeployBastion(ctx context.Context, clients *AzureClients, config *VMConfig) string {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := compileBastionServer(); err != nil {
		logger.Error("failed to compile server binary", "error", err)
		os.Exit(1)
	}

	publicIP, err := CreateBastionPublicIP(ctx, clients)
	if err != nil {
		logger.Error("failed to create public IP", "error", err)
		os.Exit(1)
	}

	nic, err := CreateBastionNIC(ctx, clients, publicIP.ID)
	if err != nil {
		logger.Error("failed to create NIC", "error", err)
		os.Exit(1)
	}

	customData, err := GenerateBastionInitScript()
	if err != nil {
		logger.Error("failed to generate init script", "error", err)
		os.Exit(1)
	}

	vm, err := CreateBastionVM(ctx, clients, config, *nic.ID, customData)
	if err != nil {
		logger.Error("failed to create bastion VM", "error", err)
		os.Exit(1)
	}

	if err := assignRoleToVM(ctx, clients, vm.Identity.PrincipalID); err != nil {
		logger.Error("failed to assign role to VM", "error", err)
		os.Exit(1)
	}

	// Ensure SSH key exists in Key Vault and copy to bastion
	privateKey, _, err := ensureBastionSSHKey(ctx, clients)
	if err != nil {
		logger.Error("failed to ensure SSH key in Key Vault", "error", err)
		os.Exit(1)
	}

	if err := copySSHKeyToBastion(ctx, config, *publicIP.Properties.IPAddress, privateKey); err != nil {
		logger.Error("failed to copy SSH key to bastion", "error", err)
		os.Exit(1)
	}

	if err := copyServerBinary(ctx, config, *publicIP.Properties.IPAddress); err != nil {
		logger.Error("failed to copy server binary", "error", err)
		os.Exit(1)
	}

	if err := copyTableStorageConfig(ctx, clients, config, *publicIP.Properties.IPAddress); err != nil {
		logger.Error("failed to copy Table Storage config", "error", err)
		os.Exit(1)
	}

	if err := startServerOnBastion(ctx, config, *publicIP.Properties.IPAddress, clients.ResourceGroupSuffix); err != nil {
		logger.Error("failed to start server on bastion", "error", err)
		os.Exit(1)
	}

	return *publicIP.Properties.IPAddress
}
