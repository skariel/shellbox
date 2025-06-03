package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
	"shellbox/internal/test"
)

func TestBastionComponentIntegration(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

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
	publicIP, err := infra.CreateBastionPublicIP(ctx, env.Clients)
	require.NoError(t, err, "should create public IP")
	assert.NotEmpty(t, *publicIP.Properties.IPAddress, "public IP should have address assigned")

	test.LogTestProgress(t, "step 2: creating bastion NIC", "publicIP", *publicIP.Properties.IPAddress)

	// Step 2: Create NIC
	nic, err := infra.CreateBastionNIC(ctx, env.Clients, publicIP.ID)
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
	vm, err := infra.CreateBastionVM(ctx, env.Clients, config, *nic.ID, customData)
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
