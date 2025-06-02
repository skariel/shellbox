//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestCreateNetworkInfrastructure(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "starting full network infrastructure creation")

	// Test the actual CreateNetworkInfrastructure function
	// This creates: resource group, bastion NSG, VNet with subnets, and initializes table storage
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "network infrastructure created, verifying components")

	// Verify resource group exists (already created by test setup, but verify it's accessible)
	rg, err := env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	require.NoError(t, err, "Resource group should exist")
	assert.Equal(t, env.ResourceGroupName, *rg.Name, "Resource group name should match")
	assert.Equal(t, env.Config.Location, *rg.Location, "Resource group location should match")

	// Verify NSG was created with correct rules
	nsgName := namer.BastionNSGName()
	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, nsgName, nil)
	require.NoError(t, err, "Bastion NSG should exist")
	assert.Equal(t, nsgName, *nsg.Name, "NSG name should match")

	// Verify NSG rules match the bastion configuration
	require.NotNil(t, nsg.Properties, "NSG properties should be set")
	require.Greater(t, len(nsg.Properties.SecurityRules), 0, "NSG should have security rules")

	// Check for expected rules (at least SSH and custom SSH)
	ruleNames := make(map[string]bool)
	for _, rule := range nsg.Properties.SecurityRules {
		ruleNames[*rule.Name] = true
	}

	assert.True(t, ruleNames["AllowSSHFromInternet"], "Should have SSH rule")
	assert.True(t, ruleNames["AllowCustomSSHFromInternet"], "Should have custom SSH rule")
	assert.True(t, ruleNames["AllowHTTPSFromInternet"], "Should have HTTPS rule")

	test.LogTestProgress(t, "NSG verification complete", "rules", len(nsg.Properties.SecurityRules))

	// Verify VNet was created with correct configuration
	vnetName := namer.VNetName()
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, vnetName, nil)
	require.NoError(t, err, "VNet should exist")
	assert.Equal(t, vnetName, *vnet.Name, "VNet name should match")

	// Verify VNet address space
	require.NotNil(t, vnet.Properties, "VNet properties should be set")
	require.NotNil(t, vnet.Properties.AddressSpace, "VNet address space should be set")
	require.Len(t, vnet.Properties.AddressSpace.AddressPrefixes, 1, "VNet should have one address prefix")
	assert.Equal(t, "10.0.0.0/8", *vnet.Properties.AddressSpace.AddressPrefixes[0], "Address space should match")

	// Verify subnets
	require.Len(t, vnet.Properties.Subnets, 2, "VNet should have two subnets")

	subnetNames := make(map[string]*struct {
		AddressPrefix string
		HasNSG        bool
		NSGID         string
	})

	for _, subnet := range vnet.Properties.Subnets {
		info := &struct {
			AddressPrefix string
			HasNSG        bool
			NSGID         string
		}{
			AddressPrefix: *subnet.Properties.AddressPrefix,
			HasNSG:        subnet.Properties.NetworkSecurityGroup != nil,
		}

		if info.HasNSG {
			info.NSGID = *subnet.Properties.NetworkSecurityGroup.ID
		}

		subnetNames[*subnet.Name] = info
	}

	// Verify bastion subnet
	bastionSubnet, exists := subnetNames[namer.BastionSubnetName()]
	require.True(t, exists, "Bastion subnet should exist")
	assert.Equal(t, "10.0.0.0/24", bastionSubnet.AddressPrefix, "Bastion subnet should have correct CIDR")
	assert.True(t, bastionSubnet.HasNSG, "Bastion subnet should have NSG attached")
	assert.Equal(t, *nsg.ID, bastionSubnet.NSGID, "Bastion subnet should reference the correct NSG")

	// Verify boxes subnet
	boxesSubnet, exists := subnetNames[namer.BoxesSubnetName()]
	require.True(t, exists, "Boxes subnet should exist")
	assert.Equal(t, "10.1.0.0/16", boxesSubnet.AddressPrefix, "Boxes subnet should have correct CIDR")
	assert.False(t, boxesSubnet.HasNSG, "Boxes subnet should not have NSG attached")

	test.LogTestProgress(t, "VNet verification complete", "subnets", len(vnet.Properties.Subnets))

	// Verify subnet IDs were set in clients
	assert.NotEmpty(t, env.Clients.BastionSubnetID, "Bastion subnet ID should be set")
	assert.NotEmpty(t, env.Clients.BoxesSubnetID, "Boxes subnet ID should be set")

	// Verify the subnet IDs match the actual subnet resources
	bastionSubnetID := ""
	boxesSubnetID := ""
	for _, subnet := range vnet.Properties.Subnets {
		switch *subnet.Name {
		case namer.BastionSubnetName():
			bastionSubnetID = *subnet.ID
		case namer.BoxesSubnetName():
			boxesSubnetID = *subnet.ID
		}
	}

	assert.Equal(t, bastionSubnetID, env.Clients.BastionSubnetID, "Bastion subnet ID should match")
	assert.Equal(t, boxesSubnetID, env.Clients.BoxesSubnetID, "Boxes subnet ID should match")

	// Verify table storage was initialized
	assert.NotEmpty(t, env.Clients.TableStorageConnectionString, "Table storage connection string should be set")
	assert.NotNil(t, env.Clients.TableClient, "Table client should be initialized")

	test.LogTestProgress(t, "full network infrastructure verification complete")
}

func TestNetworkInfrastructureRetry(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "testing network infrastructure creation twice (idempotency)")

	// Create infrastructure first time
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "first creation complete, attempting second creation")

	// Attempt to create again - should be idempotent
	// Note: This tests the CreateOrUpdate behavior of Azure resources
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "second creation complete, verifying final state")

	// Verify everything still exists and is correct
	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "NSG should still exist after second creation")
	assert.Equal(t, namer.BastionNSGName(), *nsg.Name, "NSG name should be correct")

	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "VNet should still exist after second creation")
	assert.Equal(t, namer.VNetName(), *vnet.Name, "VNet name should be correct")
	assert.Len(t, vnet.Properties.Subnets, 2, "VNet should still have two subnets")

	test.LogTestProgress(t, "idempotency verification complete")
}

func TestNetworkResourceDependencies(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "testing network resource dependencies")

	// Create infrastructure
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	// Verify that subnets reference the NSG correctly
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "VNet should exist")

	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "NSG should exist")

	// Find bastion subnet and verify NSG reference
	var bastionSubnet *armnetwork.Subnet
	for _, subnet := range vnet.Properties.Subnets {
		if *subnet.Name == namer.BastionSubnetName() {
			bastionSubnet = subnet
			break
		}
	}

	require.NotNil(t, bastionSubnet, "Bastion subnet should exist")
	require.NotNil(t, bastionSubnet.Properties.NetworkSecurityGroup, "Bastion subnet should have NSG reference")
	assert.Equal(t, *nsg.ID, *bastionSubnet.Properties.NetworkSecurityGroup.ID, "NSG reference should be correct")

	test.LogTestProgress(t, "dependency verification complete")

	// Test dependency ordering by attempting to delete resources in wrong order
	test.LogTestProgress(t, "testing dependency constraints")

	// Try to delete NSG while it's still referenced by subnet - should fail
	_, err = env.Clients.NSGClient.BeginDelete(ctx, env.ResourceGroupName, namer.BastionNSGName(), nil)
	assert.Error(t, err, "Should not be able to delete NSG while it's referenced by subnet")

	test.LogTestProgress(t, "dependency constraint verified - NSG deletion failed as expected")
}

func TestNetworkResourceNaming(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "verifying resource naming patterns")

	// Create infrastructure
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	// Verify all resources follow naming conventions
	expectedNames := map[string]string{
		"VNet":          namer.VNetName(),
		"BastionNSG":    namer.BastionNSGName(),
		"BastionSubnet": namer.BastionSubnetName(),
		"BoxesSubnet":   namer.BoxesSubnetName(),
	}

	// Verify VNet name
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, expectedNames["VNet"], nil)
	require.NoError(t, err, "VNet should exist with expected name")
	assert.Equal(t, expectedNames["VNet"], *vnet.Name, "VNet name should match pattern")

	// Verify NSG name
	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, expectedNames["BastionNSG"], nil)
	require.NoError(t, err, "NSG should exist with expected name")
	assert.Equal(t, expectedNames["BastionNSG"], *nsg.Name, "NSG name should match pattern")

	// Verify subnet names
	subnetNames := make(map[string]bool)
	for _, subnet := range vnet.Properties.Subnets {
		subnetNames[*subnet.Name] = true
	}

	assert.True(t, subnetNames[expectedNames["BastionSubnet"]], "Bastion subnet should exist with expected name")
	assert.True(t, subnetNames[expectedNames["BoxesSubnet"]], "Boxes subnet should exist with expected name")

	test.LogTestProgress(t, "resource naming verification complete", "suffix", env.Suffix)
}

func TestNetworkConfigurationValidation(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing network configuration validation")

	// Create infrastructure
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	// Get configuration hash and verify it matches expectations
	expectedConfig := infra.FormatConfig(env.Suffix)

	// Verify resource group has the correct config tag
	rg, err := env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	require.NoError(t, err, "Resource group should exist")
	require.NotNil(t, rg.Tags, "Resource group should have tags")

	configTag, exists := rg.Tags["config"]
	require.True(t, exists, "Resource group should have config tag")

	// Extract hash from tag (format: "sha256-XXXXXXXX")
	require.True(t, len(*configTag) > 7 && (*configTag)[:7] == "sha256-", "Config tag should have sha256- prefix")
	actualHash := (*configTag)[7:]

	// Verify the hash matches what we would generate for this config
	expectedHash, err := infra.GenerateConfigHash(env.Suffix)
	require.NoError(t, err, "Should be able to generate config hash")
	assert.Equal(t, expectedHash, actualHash, "Config tag should match expected hash")

	test.LogTestProgress(t, "configuration validation complete",
		"hash", actualHash,
		"config_length", len(expectedConfig))
}
