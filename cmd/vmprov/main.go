// This file shows an example of provisioning a VM on Azure
// it is not part of the app

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// getSubscriptionID gets the subscription ID from az cli
func getSubscriptionID() (string, error) {
	out, err := exec.Command("az", "account", "show", "--query", "id", "-o", "tsv").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get subscription ID: %w", err)
	}
	// Trim any whitespace, newlines, etc.
	return strings.TrimSpace(string(out)), nil
}

func readSSHKey(path string) (string, error) {
	expandedPath := filepath.Clean(os.ExpandEnv(path))
	key, err := os.ReadFile(expandedPath)
	if err != nil {
		return "", fmt.Errorf("reading SSH key: %w", err)
	}
	return string(key), nil
}

func createVM(ctx context.Context, name, sshKey string) error {
	pollUntilDoneOption := runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second}
	// Get credentials from Azure CLI
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return fmt.Errorf("failed to create credential: %w", err)
	}

	// Get subscription ID
	subscriptionID, err := getSubscriptionID()
	if err != nil {
		return err
	}

	// Initialize clients
	resourceClient, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create resource group client: %w", err)
	}

	vnetClient, err := armnetwork.NewVirtualNetworksClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create vnet client: %w", err)
	}

	nsgClient, err := armnetwork.NewSecurityGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NSG client: %w", err)
	}

	ipClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create public IP client: %w", err)
	}

	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create NIC client: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %w", err)
	}

	location := "westeurope"

	// Create resource group
	_, err = resourceClient.CreateOrUpdate(ctx, name, armresources.ResourceGroup{
		Location: &location,
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start resource group creation: %w", err)
	}

	// Create NSG
	sshRule := armnetwork.SecurityRule{
		Name: to.Ptr("SSH"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Priority:                 to.Ptr(int32(1001)),
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("22"),
		},
	}

	nsgPoller, err := nsgClient.BeginCreateOrUpdate(ctx, name, name+"-nsg", armnetwork.SecurityGroup{
		Location: &location,
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{&sshRule},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start NSG creation: %w", err)
	}
	nsg, err := nsgPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create NSG: %w", err)
	}

	// Create VNet
	vnetPoller, err := vnetClient.BeginCreateOrUpdate(ctx, name, name+"-vnet", armnetwork.VirtualNetwork{
		Location: &location,
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.0.0.0/16")},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: to.Ptr("default"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr("10.0.1.0/24"),
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start VNet creation: %w", err)
	}
	vnet, err := vnetPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create VNet: %w", err)
	}

	// Create Public IP
	ipPoller, err := ipClient.BeginCreateOrUpdate(ctx, name, name+"-ip", armnetwork.PublicIPAddress{
		Location: &location,
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

	// Create NIC
	nicPoller, err := nicClient.BeginCreateOrUpdate(ctx, name, name+"-nic", armnetwork.Interface{
		Location: &location,
		Properties: &armnetwork.InterfacePropertiesFormat{
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsg.ID,
			},
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: vnet.Properties.Subnets[0].ID,
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

	// Read Dockerfile
	dockerfileContent, err := os.ReadFile("./cmd/vmprov/dockerfile")
	if err != nil {
		return fmt.Errorf("reading Dockerfile: %w", err)
	}

	// Create setup script
	setupScript := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Create necessary directories
mkdir -p /home/shellbox/.ssh
mkdir -p /home/shellbox/container-setup

# Set up SSH key
echo '%s' > /home/shellbox/.ssh/authorized_keys
chmod 600 /home/shellbox/.ssh/authorized_keys
chown -R shellbox:shellbox /home/shellbox/.ssh

# Create Dockerfile
cat > /home/shellbox/container-setup/Dockerfile << 'EOF'
%s
EOF

# Install podman
apt-get update
apt-get upgrade -y
apt-get install -y ca-certificates curl gnupg

install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_22.04/Release.key | gpg --dearmor -o /etc/apt/keyrings/libcontainers.gpg

echo \
"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/libcontainers.gpg] \
https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/xUbuntu_22.04/ /" | \
tee /etc/apt/sources.list.d/libcontainers.list

apt-get update
apt-get install -y podman

# Build the container image
cd /home/shellbox/container-setup
podman build -t box-container-image:latest .

# Set ownership
chown -R shellbox:shellbox /home/shellbox/container-setup
`, sshKey, string(dockerfileContent))

	// Create VM
	vmPoller, err := vmClient.BeginCreateOrUpdate(ctx, name, name, armcompute.VirtualMachine{
		Location: &location,
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypesStandardDS2V2),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr("Canonical"),
					Offer:     to.Ptr("0001-com-ubuntu-server-jammy"),
					SKU:       to.Ptr("22_04-lts-gen2"),
					Version:   to.Ptr("latest"),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(name + "-disk"),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr("vm-" + name),
				AdminUsername: to.Ptr("shellbox"),
				CustomData:    to.Ptr(base64.StdEncoding.EncodeToString([]byte(setupScript))),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr("/home/shellbox/.ssh/authorized_keys"),
								KeyData: to.Ptr(sshKey),
							},
						},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: nic.ID,
					},
				},
			},
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to start VM creation: %w", err)
	}
	_, err = vmPoller.PollUntilDone(ctx, &pollUntilDoneOption)
	if err != nil {
		return fmt.Errorf("failed to create VM: %w", err)
	}

	// Get and print the public IP
	ip, err := ipClient.Get(ctx, name, name+"-ip", nil)
	if err != nil {
		return fmt.Errorf("failed to get public IP: %w", err)
	}
	fmt.Printf("IP: %s\n", *ip.Properties.IPAddress)

	return nil
}

func main() {
	if len(os.Args) != 2 {
		log.Fatal("Usage: ./vmcreate <name>")
	}

	keyPath := os.Getenv("SSH_PUBLIC_KEY")
	if keyPath == "" {
		keyPath = "$HOME/.ssh/id_ed25519.pub"
	}

	sshKey, err := readSSHKey(keyPath)
	if err != nil {
		log.Fatal(err)
	}

	name := os.Args[1]
	if err := createVM(context.Background(), name, sshKey); err != nil {
		log.Fatal(err)
	}

	log.Printf("VM '%s' created successfully", name)
}
