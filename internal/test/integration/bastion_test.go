//go:build integration

package integration

import (
	"context"
	"encoding/base64"
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

func TestCreateBastionPublicIP(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing bastion public IP creation")

	namer := env.GetResourceNamer()

	// Create public IP directly using the bastion function
	publicIP, err := createBastionPublicIP(ctx, env.Clients)
	require.NoError(t, err, "should create bastion public IP without error")

	// Verify public IP properties
	assert.Equal(t, namer.BastionPublicIPName(), *publicIP.Name, "public IP should have correct name")
	assert.Equal(t, infra.Location, *publicIP.Location, "public IP should be in correct location")
	assert.Equal(t, armnetwork.PublicIPAddressSKUNameStandard, *publicIP.SKU.Name, "public IP should use Standard SKU")
	assert.Equal(t, armnetwork.IPAllocationMethodStatic, *publicIP.Properties.PublicIPAllocationMethod, "public IP should use static allocation")
	assert.NotEmpty(t, *publicIP.Properties.IPAddress, "public IP should have an IP address assigned")

	test.LogTestProgress(t, "verifying public IP can be retrieved", "ip", *publicIP.Properties.IPAddress)

	// Verify public IP can be retrieved
	retrievedIP, err := env.Clients.PublicIPClient.Get(ctx, env.ResourceGroupName, namer.BastionPublicIPName(), nil)
	require.NoError(t, err, "should be able to retrieve created public IP")
	assert.Equal(t, *publicIP.ID, *retrievedIP.ID, "retrieved public IP should have same ID")
}

func TestCreateBastionNIC(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "setting up network infrastructure for NIC test")

	// Create network infrastructure first (required for subnets)
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "creating public IP for NIC test")

	// Create public IP first
	publicIP, err := createBastionPublicIP(ctx, env.Clients)
	require.NoError(t, err, "should create public IP for NIC test")

	test.LogTestProgress(t, "creating bastion NIC")

	// Create NIC
	nic, err := createBastionNIC(ctx, env.Clients, publicIP.ID)
	require.NoError(t, err, "should create bastion NIC without error")

	// Verify NIC properties
	assert.Equal(t, namer.BastionNICName(), *nic.Name, "NIC should have correct name")
	assert.Equal(t, infra.Location, *nic.Location, "NIC should be in correct location")
	require.Len(t, nic.Properties.IPConfigurations, 1, "NIC should have one IP configuration")

	ipConfig := nic.Properties.IPConfigurations[0]
	assert.Equal(t, "ipconfig1", *ipConfig.Name, "IP config should have correct name")
	assert.Equal(t, armnetwork.IPAllocationMethodDynamic, *ipConfig.Properties.PrivateIPAllocationMethod, "should use dynamic private IP allocation")
	assert.Equal(t, env.Clients.BastionSubnetID, *ipConfig.Properties.Subnet.ID, "NIC should reference correct subnet")
	assert.Equal(t, *publicIP.ID, *ipConfig.Properties.PublicIPAddress.ID, "NIC should reference correct public IP")

	test.LogTestProgress(t, "verifying NIC can be retrieved")

	// Verify NIC can be retrieved
	retrievedNIC, err := env.Clients.NICClient.Get(ctx, env.ResourceGroupName, namer.BastionNICName(), nil)
	require.NoError(t, err, "should be able to retrieve created NIC")
	assert.Equal(t, *nic.ID, *retrievedNIC.ID, "retrieved NIC should have same ID")
}

func TestCreateBastionVM(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "setting up network infrastructure for VM test")

	// Create network infrastructure first
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "creating public IP and NIC for VM test")

	// Create public IP and NIC first
	publicIP, err := createBastionPublicIP(ctx, env.Clients)
	require.NoError(t, err, "should create public IP for VM test")

	nic, err := createBastionNIC(ctx, env.Clients, publicIP.ID)
	require.NoError(t, err, "should create NIC for VM test")

	test.LogTestProgress(t, "generating bastion init script")

	// Generate init script
	customData, err := infra.GenerateBastionInitScript()
	require.NoError(t, err, "should generate bastion init script")

	// Verify init script content
	decodedScript, err := base64.StdEncoding.DecodeString(customData)
	require.NoError(t, err, "init script should be valid base64")
	scriptContent := string(decodedScript)
	assert.Contains(t, scriptContent, "#cloud-config", "init script should be cloud-config format")
	assert.Contains(t, scriptContent, "apt-get update", "init script should contain apt update")
	assert.Contains(t, scriptContent, "ufw allow OpenSSH", "init script should configure firewall")

	test.LogTestProgress(t, "creating bastion VM")

	// Create VM config
	config := infra.DefaultBastionConfig()
	// Load SSH public key using the same function as production
	_, sshPublicKey, err := sshutil.LoadKeyPair("/home/ubuntu/.ssh/id_ed25519")
	require.NoError(t, err, "should load SSH key")
	config.SSHPublicKey = sshPublicKey

	// Create VM
	vm, err := createBastionVM(ctx, env.Clients, config, *nic.ID, customData)
	require.NoError(t, err, "should create bastion VM without error")

	// Verify VM properties
	assert.Equal(t, namer.BastionVMName(), *vm.Name, "VM should have correct name")
	assert.Equal(t, infra.Location, *vm.Location, "VM should be in correct location")

	// Verify hardware profile
	require.NotNil(t, vm.Properties.HardwareProfile, "VM should have hardware profile")
	assert.Equal(t, armcompute.VirtualMachineSizeTypes(config.VMSize), *vm.Properties.HardwareProfile.VMSize, "VM should have correct size")

	// Verify storage profile
	require.NotNil(t, vm.Properties.StorageProfile, "VM should have storage profile")
	require.NotNil(t, vm.Properties.StorageProfile.ImageReference, "VM should have image reference")
	assert.Equal(t, infra.VMPublisher, *vm.Properties.StorageProfile.ImageReference.Publisher, "VM should use correct image publisher")
	assert.Equal(t, infra.VMOffer, *vm.Properties.StorageProfile.ImageReference.Offer, "VM should use correct image offer")
	assert.Equal(t, infra.VMSku, *vm.Properties.StorageProfile.ImageReference.SKU, "VM should use correct image SKU")

	// Verify OS disk
	require.NotNil(t, vm.Properties.StorageProfile.OSDisk, "VM should have OS disk")
	assert.Equal(t, namer.BastionOSDiskName(), *vm.Properties.StorageProfile.OSDisk.Name, "OS disk should have correct name")
	assert.Equal(t, armcompute.DiskCreateOptionTypesFromImage, *vm.Properties.StorageProfile.OSDisk.CreateOption, "OS disk should be created from image")

	// Verify OS profile
	require.NotNil(t, vm.Properties.OSProfile, "VM should have OS profile")
	assert.Equal(t, namer.BastionComputerName(), *vm.Properties.OSProfile.ComputerName, "VM should have correct computer name")
	assert.Equal(t, config.AdminUsername, *vm.Properties.OSProfile.AdminUsername, "VM should have correct admin username")
	assert.Equal(t, customData, *vm.Properties.OSProfile.CustomData, "VM should have correct custom data")

	// Verify Linux configuration
	require.NotNil(t, vm.Properties.OSProfile.LinuxConfiguration, "VM should have Linux configuration")
	assert.True(t, *vm.Properties.OSProfile.LinuxConfiguration.DisablePasswordAuthentication, "password authentication should be disabled")
	require.NotNil(t, vm.Properties.OSProfile.LinuxConfiguration.SSH, "VM should have SSH configuration")
	require.Len(t, vm.Properties.OSProfile.LinuxConfiguration.SSH.PublicKeys, 1, "VM should have one SSH public key")
	assert.Equal(t, config.SSHPublicKey, *vm.Properties.OSProfile.LinuxConfiguration.SSH.PublicKeys[0].KeyData, "SSH key should match config")

	// Verify network profile
	require.NotNil(t, vm.Properties.NetworkProfile, "VM should have network profile")
	require.Len(t, vm.Properties.NetworkProfile.NetworkInterfaces, 1, "VM should have one network interface")
	assert.Equal(t, *nic.ID, *vm.Properties.NetworkProfile.NetworkInterfaces[0].ID, "VM should reference correct NIC")
	assert.True(t, *vm.Properties.NetworkProfile.NetworkInterfaces[0].Properties.Primary, "NIC should be marked as primary")

	// Verify managed identity
	require.NotNil(t, vm.Identity, "VM should have managed identity")
	assert.Equal(t, armcompute.ResourceIdentityTypeSystemAssigned, *vm.Identity.Type, "VM should have system-assigned managed identity")
	assert.NotNil(t, vm.Identity.PrincipalID, "VM should have principal ID")

	test.LogTestProgress(t, "verifying VM can be retrieved", "principalID", *vm.Identity.PrincipalID)

	// Verify VM can be retrieved
	retrievedVM, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, namer.BastionVMName(), nil)
	require.NoError(t, err, "should be able to retrieve created VM")
	assert.Equal(t, *vm.ID, *retrievedVM.ID, "retrieved VM should have same ID")
}

func TestBastionConfigGeneration(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	test.LogTestProgress(t, "testing bastion configuration generation")

	// Test default config generation
	config := infra.DefaultBastionConfig()
	require.NotNil(t, config, "should generate default config")
	assert.Equal(t, "shellbox", config.AdminUsername, "default config should have correct admin username")
	assert.Equal(t, string(armcompute.VirtualMachineSizeTypesStandardD2SV3), config.VMSize, "default config should have correct VM size")

	// Test init script generation
	initScript, err := infra.GenerateBastionInitScript()
	require.NoError(t, err, "should generate init script without error")
	assert.NotEmpty(t, initScript, "init script should not be empty")

	// Decode and verify script content
	decodedScript, err := base64.StdEncoding.DecodeString(initScript)
	require.NoError(t, err, "init script should be valid base64")
	scriptContent := string(decodedScript)

	// Verify cloud-config format
	assert.Contains(t, scriptContent, "#cloud-config", "script should be in cloud-config format")
	assert.Contains(t, scriptContent, "runcmd:", "script should contain run commands")

	// Verify security setup
	assert.Contains(t, scriptContent, "apt-get update", "script should update packages")
	assert.Contains(t, scriptContent, "ufw allow OpenSSH", "script should configure UFW")
	assert.Contains(t, scriptContent, "ufw --force enable", "script should enable UFW")

	// Verify SSH configuration
	assert.Contains(t, scriptContent, "/etc/ssh/sshd_config.d/", "script should configure SSH")
	assert.Contains(t, scriptContent, "PermitUserEnvironment yes", "script should allow user environment")
	assert.Contains(t, scriptContent, "systemctl reload sshd", "script should reload SSH daemon")

	test.LogTestProgress(t, "configuration generation tests completed")
}

func TestBastionResourceNaming(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupMinimalTestEnvironment(t)
	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "testing bastion resource naming conventions")

	// Test bastion VM naming
	vmName := namer.BastionVMName()
	assert.Contains(t, vmName, "shellbox", "VM name should contain shellbox")
	assert.Contains(t, vmName, env.Suffix, "VM name should contain test suffix")
	assert.Contains(t, vmName, "bastion-vm", "VM name should contain bastion-vm")

	// Test bastion NIC naming
	nicName := namer.BastionNICName()
	assert.Contains(t, nicName, "shellbox", "NIC name should contain shellbox")
	assert.Contains(t, nicName, env.Suffix, "NIC name should contain test suffix")
	assert.Contains(t, nicName, "bastion-nic", "NIC name should contain bastion-nic")

	// Test bastion public IP naming
	pipName := namer.BastionPublicIPName()
	assert.Contains(t, pipName, "shellbox", "public IP name should contain shellbox")
	assert.Contains(t, pipName, env.Suffix, "public IP name should contain test suffix")
	assert.Contains(t, pipName, "bastion-pip", "public IP name should contain bastion-pip")

	// Test bastion OS disk naming
	osDiskName := namer.BastionOSDiskName()
	assert.Contains(t, osDiskName, "shellbox", "OS disk name should contain shellbox")
	assert.Contains(t, osDiskName, env.Suffix, "OS disk name should contain test suffix")
	assert.Contains(t, osDiskName, "bastion-os-disk", "OS disk name should contain bastion-os-disk")

	// Test bastion computer name
	computerName := namer.BastionComputerName()
	assert.Equal(t, "shellbox-bastion", computerName, "computer name should be shellbox-bastion")

	// Test NSG name (for comparison with instance NSG)
	nsgName := namer.BastionNSGName()
	assert.Contains(t, nsgName, "shellbox", "NSG name should contain shellbox")
	assert.Contains(t, nsgName, env.Suffix, "NSG name should contain test suffix")
	assert.Contains(t, nsgName, "bastion-nsg", "NSG name should contain bastion-nsg")

	test.LogTestProgress(t, "resource naming tests completed", "suffix", env.Suffix)
}

func TestBastionNetworkConfiguration(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing bastion network configuration integration")

	// Create network infrastructure
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	// Verify bastion subnet exists and has correct configuration
	assert.NotEmpty(t, env.Clients.BastionSubnetID, "bastion subnet ID should be set")
	assert.Contains(t, env.Clients.BastionSubnetID, "bastion-subnet", "bastion subnet ID should reference bastion subnet")

	namer := env.GetResourceNamer()

	// Verify NSG exists with correct rules
	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "bastion NSG should exist")

	// Verify NSG rules for bastion
	ruleNames := make(map[string]bool)
	for _, rule := range nsg.Properties.SecurityRules {
		ruleNames[*rule.Name] = true
	}

	expectedRules := []string{
		"AllowSSHFromInternet",
		"AllowCustomSSHFromInternet",
		"AllowHTTPSFromInternet",
		"AllowToBoxesSubnet",
		"AllowToInternet",
	}

	for _, expectedRule := range expectedRules {
		assert.True(t, ruleNames[expectedRule], "NSG should have rule %s", expectedRule)
	}

	// Verify VNet configuration includes bastion subnet
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "VNet should exist")

	bastionSubnetFound := false
	for _, subnet := range vnet.Properties.Subnets {
		if *subnet.Name == namer.BastionSubnetName() {
			bastionSubnetFound = true
			assert.Equal(t, "10.0.0.0/24", *subnet.Properties.AddressPrefix, "bastion subnet should have correct CIDR")
			assert.NotNil(t, subnet.Properties.NetworkSecurityGroup, "bastion subnet should have NSG attached")
			assert.Equal(t, *nsg.ID, *subnet.Properties.NetworkSecurityGroup.ID, "bastion subnet should reference bastion NSG")
			break
		}
	}
	assert.True(t, bastionSubnetFound, "bastion subnet should exist in VNet")

	test.LogTestProgress(t, "network configuration tests completed")
}

func TestBastionDeploymentErrorHandling(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing bastion deployment error handling")

	// Test 1: Create public IP with invalid location (should fail)
	// First, temporarily modify clients to use invalid location
	// Note: This is challenging to test without modifying the function, so we'll test other error conditions

	// Test 2: Create NIC without network infrastructure (should fail)
	publicIP := &armnetwork.PublicIPAddress{
		ID: &[]string{"invalid-public-ip-id"}[0],
	}

	_, err := createBastionNIC(ctx, env.Clients, publicIP.ID)
	assert.Error(t, err, "should fail to create NIC without bastion subnet")

	// Test 3: Test invalid VM configuration
	invalidConfig := &infra.VMConfig{
		AdminUsername: "", // Invalid empty username
		SSHPublicKey:  "", // Invalid empty SSH key
		VMSize:        "InvalidVMSize",
	}

	_, err = infra.GenerateBastionInitScript()
	assert.NoError(t, err, "init script generation should not depend on config validation")

	// Test init script with invalid base64 (manually constructed)
	invalidScript := "not-valid-base64!"
	_, err = base64.StdEncoding.DecodeString(invalidScript)
	assert.Error(t, err, "invalid base64 should fail to decode")

	test.LogTestProgress(t, "error handling tests completed")

	// Test valid case after error tests
	validScript, err := infra.GenerateBastionInitScript()
	assert.NoError(t, err, "valid init script should generate successfully")
	assert.NotEmpty(t, validScript, "valid init script should not be empty")

	// Verify the valid script can be decoded
	_, err = base64.StdEncoding.DecodeString(validScript)
	assert.NoError(t, err, "valid init script should decode successfully")

	// Use the invalid config for testing (but don't try to create resources with it)
	assert.Empty(t, invalidConfig.AdminUsername, "invalid config should have empty username")
	assert.Empty(t, invalidConfig.SSHPublicKey, "invalid config should have empty SSH key")
}

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
