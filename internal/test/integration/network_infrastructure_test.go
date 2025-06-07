package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestNetworkInfrastructureIdempotency(t *testing.T) {
	t.Parallel()
	env := test.SetupTestEnvironment(t)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "testing network infrastructure idempotency and comprehensive verification")

	// Create infrastructure first time
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "first creation complete, performing comprehensive verification")

	verifyResourceGroup(ctx, t, env)
	nsg := verifyNSG(ctx, t, env, namer)
	vnet := verifyVNet(ctx, t, env, namer)
	verifySubnets(t, namer, &vnet, &nsg)
	verifySubnetIDsInClients(t, env, &vnet, namer)
	verifyTableStorageInitialized(t, env)

	test.LogTestProgress(t, "attempting second creation to test idempotency")

	// Attempt to create again - should be idempotent
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "second creation complete, verifying idempotency")

	verifyIdempotency(ctx, t, env, namer, nsg, vnet)

	test.LogTestProgress(t, "comprehensive infrastructure verification and idempotency test complete")
}

func verifyResourceGroup(ctx context.Context, t *testing.T, env *test.Environment) {
	rg, err := env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	if err != nil {
		t.Fatalf("Resource group should exist: %v", err)
	}
	if *rg.Name != env.ResourceGroupName {
		t.Errorf("Resource group name should match: got %s, want %s", *rg.Name, env.ResourceGroupName)
	}
	if *rg.Location != env.Config.Location {
		t.Errorf("Resource group location should match: got %s, want %s", *rg.Location, env.Config.Location)
	}
}

func verifyNSG(ctx context.Context, t *testing.T, env *test.Environment, namer *infra.ResourceNamer) armnetwork.SecurityGroupsClientGetResponse {
	nsgName := namer.BastionNSGName()
	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, nsgName, nil)
	if err != nil {
		t.Fatalf("Bastion NSG should exist: %v", err)
	}
	if *nsg.Name != nsgName {
		t.Errorf("NSG name should match: got %s, want %s", *nsg.Name, nsgName)
	}

	verifyNSGRules(t, &nsg)
	return nsg
}

func verifyNSGRules(t *testing.T, nsg *armnetwork.SecurityGroupsClientGetResponse) {
	if nsg.Properties == nil {
		t.Fatal("NSG properties should be set")
	}
	if len(nsg.Properties.SecurityRules) == 0 {
		t.Fatal("NSG should have security rules")
	}

	ruleNames := make(map[string]bool)
	for _, rule := range nsg.Properties.SecurityRules {
		ruleNames[*rule.Name] = true
	}

	requiredRules := []string{"AllowSSHFromInternet", "AllowCustomSSHFromInternet", "AllowHTTPSFromInternet"}
	for _, ruleName := range requiredRules {
		if !ruleNames[ruleName] {
			t.Errorf("Should have %s rule", ruleName)
		}
	}
}

func verifyVNet(ctx context.Context, t *testing.T, env *test.Environment, namer *infra.ResourceNamer) armnetwork.VirtualNetworksClientGetResponse {
	vnetName := namer.VNetName()
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, vnetName, nil)
	if err != nil {
		t.Fatalf("VNet should exist: %v", err)
	}
	if *vnet.Name != vnetName {
		t.Errorf("VNet name should match: got %s, want %s", *vnet.Name, vnetName)
	}

	verifyVNetAddressSpace(t, &vnet)
	return vnet
}

func verifyVNetAddressSpace(t *testing.T, vnet *armnetwork.VirtualNetworksClientGetResponse) {
	if vnet.Properties == nil {
		t.Fatal("VNet properties should be set")
	}
	if vnet.Properties.AddressSpace == nil {
		t.Fatal("VNet address space should be set")
	}
	if len(vnet.Properties.AddressSpace.AddressPrefixes) != 1 {
		t.Fatalf("VNet should have one address prefix, got %d", len(vnet.Properties.AddressSpace.AddressPrefixes))
	}
	if *vnet.Properties.AddressSpace.AddressPrefixes[0] != "10.0.0.0/8" {
		t.Errorf("Address space should match: got %s, want 10.0.0.0/8", *vnet.Properties.AddressSpace.AddressPrefixes[0])
	}
}

func verifySubnets(t *testing.T, namer *infra.ResourceNamer, vnet *armnetwork.VirtualNetworksClientGetResponse, nsg *armnetwork.SecurityGroupsClientGetResponse) {
	if len(vnet.Properties.Subnets) != 2 {
		t.Fatalf("VNet should have two subnets, got %d", len(vnet.Properties.Subnets))
	}

	subnetMap := make(map[string]*armnetwork.Subnet)
	for _, subnet := range vnet.Properties.Subnets {
		subnetMap[*subnet.Name] = subnet
	}

	verifyBastionSubnet(t, namer, subnetMap, nsg)
	verifyBoxesSubnet(t, namer, subnetMap)
}

func verifyBastionSubnet(t *testing.T, namer *infra.ResourceNamer, subnetMap map[string]*armnetwork.Subnet, nsg *armnetwork.SecurityGroupsClientGetResponse) {
	bastionSubnet, exists := subnetMap[namer.BastionSubnetName()]
	if !exists {
		t.Fatal("Bastion subnet should exist")
	}
	if *bastionSubnet.Properties.AddressPrefix != "10.0.0.0/24" {
		t.Errorf("Bastion subnet CIDR should match: got %s, want 10.0.0.0/24", *bastionSubnet.Properties.AddressPrefix)
	}
	if bastionSubnet.Properties.NetworkSecurityGroup == nil {
		t.Error("Bastion subnet should have NSG attached")
		return
	}
	if *bastionSubnet.Properties.NetworkSecurityGroup.ID != *nsg.ID {
		t.Errorf("Bastion subnet should reference correct NSG: got %s, want %s", *bastionSubnet.Properties.NetworkSecurityGroup.ID, *nsg.ID)
	}
}

func verifyBoxesSubnet(t *testing.T, namer *infra.ResourceNamer, subnetMap map[string]*armnetwork.Subnet) {
	boxesSubnet, exists := subnetMap[namer.BoxesSubnetName()]
	if !exists {
		t.Fatal("Boxes subnet should exist")
	}
	if *boxesSubnet.Properties.AddressPrefix != "10.1.0.0/16" {
		t.Errorf("Boxes subnet CIDR should match: got %s, want 10.1.0.0/16", *boxesSubnet.Properties.AddressPrefix)
	}
	if boxesSubnet.Properties.NetworkSecurityGroup != nil {
		t.Error("Boxes subnet should not have NSG attached")
	}
}

func verifySubnetIDsInClients(t *testing.T, env *test.Environment, vnet *armnetwork.VirtualNetworksClientGetResponse, namer *infra.ResourceNamer) {
	subnetMap := make(map[string]*armnetwork.Subnet)
	for _, subnet := range vnet.Properties.Subnets {
		subnetMap[*subnet.Name] = subnet
	}

	bastionSubnet := subnetMap[namer.BastionSubnetName()]
	boxesSubnet := subnetMap[namer.BoxesSubnetName()]

	if env.Clients.BastionSubnetID == "" {
		t.Error("Bastion subnet ID should be set")
	}
	if env.Clients.BoxesSubnetID == "" {
		t.Error("Boxes subnet ID should be set")
	}
	if bastionSubnet != nil && env.Clients.BastionSubnetID != *bastionSubnet.ID {
		t.Errorf("Bastion subnet ID should match: got %s, want %s", env.Clients.BastionSubnetID, *bastionSubnet.ID)
	}
	if boxesSubnet != nil && env.Clients.BoxesSubnetID != *boxesSubnet.ID {
		t.Errorf("Boxes subnet ID should match: got %s, want %s", env.Clients.BoxesSubnetID, *boxesSubnet.ID)
	}
}

func verifyTableStorageInitialized(t *testing.T, env *test.Environment) {
	if env.Clients.TableStorageConnectionString == "" {
		t.Error("Table storage connection string should be set")
	}
	if env.Clients.TableClient == nil {
		t.Error("Table client should be initialized")
	}
}

func verifyIdempotency(ctx context.Context, t *testing.T, env *test.Environment, namer *infra.ResourceNamer, originalNSG armnetwork.SecurityGroupsClientGetResponse, originalVNet armnetwork.VirtualNetworksClientGetResponse) {
	nsg2, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, namer.BastionNSGName(), nil)
	if err != nil {
		t.Fatalf("NSG should still exist after second creation: %v", err)
	}
	if *nsg2.ID != *originalNSG.ID {
		t.Errorf("NSG ID should be unchanged: got %s, want %s", *nsg2.ID, *originalNSG.ID)
	}

	vnet2, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, namer.VNetName(), nil)
	if err != nil {
		t.Fatalf("VNet should still exist after second creation: %v", err)
	}
	if *vnet2.ID != *originalVNet.ID {
		t.Errorf("VNet ID should be unchanged: got %s, want %s", *vnet2.ID, *originalVNet.ID)
	}
	if len(vnet2.Properties.Subnets) != 2 {
		t.Errorf("VNet should still have two subnets, got %d", len(vnet2.Properties.Subnets))
	}
}
