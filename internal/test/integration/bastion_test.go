//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
	"shellbox/internal/test"
)

func TestBastionComponentIntegration(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing bastion component integration (without SSH operations)")

	// Create network infrastructure first
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "step 1: creating bastion public IP")

	// Step 1: Create public IP
	publicIP, err := createBastionPublicIP(ctx, env.Clients)
	require.NoError(t, err, "should create public IP")
	assert.NotEmpty(t, *publicIP.Properties.IPAddress, "public IP should have address assigned")

	test.LogTestProgress(t, "step 2: creating bastion NIC", "publicIP", *publicIP.Properties.IPAddress)

	// Step 2: Create NIC
	nic, err := createBastionNIC(ctx, env.Clients, publicIP.ID)
	require.NoError(t, err, "should create NIC")
	assert.Contains(t, *nic.Properties.IPConfigurations[0].Properties.Subnet.ID, "bastion-subnet", "NIC should be in bastion subnet")

	test.LogTestProgress(t, "step 3: generating configuration and init script")

	// Step 3: Generate configuration
	config := infra.DefaultBastionConfig()
	// Load SSH public key using the same function as production
	_, sshPublicKey, err := sshutil.LoadKeyPair("/home/ubuntu/.ssh/id_ed25519")
	require.NoError(t, err, "should load SSH key")
	config.SSHPublicKey = sshPublicKey

	customData, err := infra.GenerateBastionInitScript()
	require.NoError(t, err, "should generate init script")

	test.LogTestProgress(t, "step 4: creating bastion VM")

	// Step 4: Create VM
	vm, err := createBastionVM(ctx, env.Clients, config, *nic.ID, customData)
	require.NoError(t, err, "should create VM")
	require.NotNil(t, vm.Identity, "VM should have managed identity")
	require.NotNil(t, vm.Identity.PrincipalID, "VM should have principal ID")

	test.LogTestProgress(t, "step 5: verifying all components work together")

	// Step 5: Verify all components work together
	// Verify VM has correct NIC
	assert.Equal(t, *nic.ID, *vm.Properties.NetworkProfile.NetworkInterfaces[0].ID, "VM should reference correct NIC")

	// Verify NIC has correct public IP
	assert.Equal(t, *publicIP.ID, *nic.Properties.IPConfigurations[0].Properties.PublicIPAddress.ID, "NIC should reference correct public IP")

	// Verify NIC is in correct subnet
	assert.Equal(t, env.Clients.BastionSubnetID, *nic.Properties.IPConfigurations[0].Properties.Subnet.ID, "NIC should be in bastion subnet")

	// Verify all resources can be retrieved
	retrievedIP, err := env.Clients.PublicIPClient.Get(ctx, env.ResourceGroupName, namer.BastionPublicIPName(), nil)
	require.NoError(t, err, "should retrieve public IP")
	assert.Equal(t, *publicIP.ID, *retrievedIP.ID, "retrieved IP should match")

	retrievedNIC, err := env.Clients.NICClient.Get(ctx, env.ResourceGroupName, namer.BastionNICName(), nil)
	require.NoError(t, err, "should retrieve NIC")
	assert.Equal(t, *nic.ID, *retrievedNIC.ID, "retrieved NIC should match")

	retrievedVM, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, namer.BastionVMName(), nil)
	require.NoError(t, err, "should retrieve VM")
	assert.Equal(t, *vm.ID, *retrievedVM.ID, "retrieved VM should match")

	test.LogTestProgress(t, "bastion component integration test completed",
		"vmID", *vm.ID,
		"principalID", *vm.Identity.PrincipalID,
		"publicIP", *publicIP.Properties.IPAddress)
}

// Helper functions (these are internal functions that we need to access for testing)

func createBastionPublicIP(ctx context.Context, clients *infra.AzureClients) (*armnetwork.PublicIPAddress, error) {
	// This is a copy of the internal function for testing purposes
	// In a real implementation, we might want to make these functions public or create a test interface
	namer := infra.NewResourceNamer(clients.Suffix)
	ipPoller, err := clients.PublicIPClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionPublicIPName(), armnetwork.PublicIPAddress{
		Location: to.Ptr(infra.Location),
		SKU: &armnetwork.PublicIPAddressSKU{
			Name: to.Ptr(armnetwork.PublicIPAddressSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPAddressPropertiesFormat{
			PublicIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodStatic),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	res, err := ipPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, err
	}
	return &res.PublicIPAddress, nil
}

func createBastionNIC(ctx context.Context, clients *infra.AzureClients, publicIPID *string) (*armnetwork.Interface, error) {
	// This is a copy of the internal function for testing purposes
	namer := infra.NewResourceNamer(clients.Suffix)
	nicPoller, err := clients.NICClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionNICName(), armnetwork.Interface{
		Location: to.Ptr(infra.Location),
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
		return nil, err
	}
	res, err := nicPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, err
	}
	return &res.Interface, nil
}

func createBastionVM(ctx context.Context, clients *infra.AzureClients, config *infra.VMConfig, nicID string, customData string) (*armcompute.VirtualMachine, error) {
	// This is a copy of the internal function for testing purposes
	namer := infra.NewResourceNamer(clients.Suffix)
	vmPoller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, namer.BastionVMName(), armcompute.VirtualMachine{
		Location: to.Ptr(infra.Location),
		Identity: &armcompute.VirtualMachineIdentity{
			Type: to.Ptr(armcompute.ResourceIdentityTypeSystemAssigned),
		},
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(config.VMSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr(infra.VMPublisher),
					Offer:     to.Ptr(infra.VMOffer),
					SKU:       to.Ptr(infra.VMSku),
					Version:   to.Ptr(infra.VMVersion),
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
								Path:    to.Ptr("/home/" + config.AdminUsername + "/.ssh/authorized_keys"),
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
		return nil, err
	}

	vm, err := vmPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, err
	}

	return &vm.VirtualMachine, nil
}
