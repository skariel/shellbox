package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
	"shellbox/internal/test"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
)

// Helper types and functions to reduce cyclomatic complexity in bastion tests
type bastionComponents struct {
	publicIP *armnetwork.PublicIPAddress
	nic      *armnetwork.Interface
	vm       *armcompute.VirtualMachine
}

func createBastionConfig(t *testing.T) *infra.VMConfig {
	t.Helper()
	config := infra.DefaultBastionConfig()
	_, sshPublicKey, err := sshutil.LoadKeyPair("/home/ubuntu/.ssh/id_ed25519")
	if err != nil {
		t.Fatalf("should load SSH key: %v", err)
	}
	config.SSHPublicKey = sshPublicKey
	return config
}

func verifyBastionConnections(t *testing.T, components bastionComponents, env *test.Environment) {
	t.Helper()

	// Verify VM has correct NIC
	if *components.vm.Properties.NetworkProfile.NetworkInterfaces[0].ID != *components.nic.ID {
		t.Errorf("VM should reference correct NIC")
	}

	// Verify NIC has correct public IP
	if *components.nic.Properties.IPConfigurations[0].Properties.PublicIPAddress.ID != *components.publicIP.ID {
		t.Errorf("NIC should reference correct public IP")
	}

	// Verify NIC is in correct subnet
	if *components.nic.Properties.IPConfigurations[0].Properties.Subnet.ID != env.Clients.BastionSubnetID {
		t.Errorf("NIC should be in bastion subnet")
	}
}

func verifyResourceRetrieval(ctx context.Context, t *testing.T, components bastionComponents, env *test.Environment) {
	t.Helper()
	namer := env.GetResourceNamer()

	retrievedIP, err := env.Clients.PublicIPClient.Get(ctx, env.ResourceGroupName, namer.BastionPublicIPName(), nil)
	if err != nil {
		t.Fatalf("should retrieve public IP: %v", err)
	}
	if *retrievedIP.ID != *components.publicIP.ID {
		t.Errorf("retrieved IP should match original")
	}

	retrievedNIC, err := env.Clients.NICClient.Get(ctx, env.ResourceGroupName, namer.BastionNICName(), nil)
	if err != nil {
		t.Fatalf("should retrieve NIC: %v", err)
	}
	if *retrievedNIC.ID != *components.nic.ID {
		t.Errorf("retrieved NIC should match original")
	}

	retrievedVM, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, namer.BastionVMName(), nil)
	if err != nil {
		t.Fatalf("should retrieve VM: %v", err)
	}
	if *retrievedVM.ID != *components.vm.ID {
		t.Errorf("retrieved VM should match original")
	}
}

func TestBastionComponentIntegration(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing bastion component integration (without SSH operations)")

	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "step 1: creating bastion public IP")
	publicIP, err := infra.CreateBastionPublicIP(ctx, env.Clients)
	if err != nil {
		t.Fatalf("should create public IP: %v", err)
	}
	if *publicIP.Properties.IPAddress == "" {
		t.Error("public IP should have address assigned")
	}

	test.LogTestProgress(t, "step 2: creating bastion NIC", "publicIP", *publicIP.Properties.IPAddress)
	nic, err := infra.CreateBastionNIC(ctx, env.Clients, publicIP.ID)
	if err != nil {
		t.Fatalf("should create NIC: %v", err)
	}
	if !strings.Contains(*nic.Properties.IPConfigurations[0].Properties.Subnet.ID, "bastion-subnet") {
		t.Error("NIC should be in bastion subnet")
	}

	test.LogTestProgress(t, "step 3: generating configuration and init script")
	config := createBastionConfig(t)
	customData, err := infra.GenerateBastionInitScript()
	if err != nil {
		t.Fatalf("should generate init script: %v", err)
	}

	test.LogTestProgress(t, "step 4: creating bastion VM")
	vm, err := infra.CreateBastionVM(ctx, env.Clients, config, *nic.ID, customData)
	if err != nil {
		t.Fatalf("should create VM: %v", err)
	}
	if vm.Identity == nil || vm.Identity.PrincipalID == nil {
		t.Fatal("VM should have managed identity with principal ID")
	}

	test.LogTestProgress(t, "step 5: verifying all components work together")
	components := bastionComponents{publicIP: publicIP, nic: nic, vm: vm}
	verifyBastionConnections(t, components, env)
	verifyResourceRetrieval(ctx, t, components, env)

	test.LogTestProgress(t, "bastion component integration test completed",
		"vmID", *vm.ID,
		"principalID", *vm.Identity.PrincipalID,
		"publicIP", *publicIP.Properties.IPAddress)
}
