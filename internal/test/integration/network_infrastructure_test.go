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

func TestNetworkInfrastructureIdempotency(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)
	env := test.SetupTestEnvironment(t, test.CategoryIntegration)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "testing network infrastructure idempotency and comprehensive verification")

	// Create infrastructure first time
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "first creation complete, performing comprehensive verification")

	// Verify resource group exists
	rg, err := env.Clients.ResourceClient.Get(ctx, env.ResourceGroupName, nil)
	require.NoError(t, err, "Resource group should exist")
	assert.Equal(t, env.ResourceGroupName, *rg.Name, "Resource group name should match")
	assert.Equal(t, env.Config.Location, *rg.Location, "Resource group location should match")

	// Verify NSG was created with correct rules
	nsgName := namer.BastionNSGName()
	nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, nsgName, nil)
	require.NoError(t, err, "Bastion NSG should exist")
	assert.Equal(t, nsgName, *nsg.Name, "NSG name should match")

	// Verify NSG rules
	require.NotNil(t, nsg.Properties, "NSG properties should be set")
	require.Greater(t, len(nsg.Properties.SecurityRules), 0, "NSG should have security rules")

	ruleNames := make(map[string]bool)
	for _, rule := range nsg.Properties.SecurityRules {
		ruleNames[*rule.Name] = true
	}

	assert.True(t, ruleNames["AllowSSHFromInternet"], "Should have SSH rule")
	assert.True(t, ruleNames["AllowCustomSSHFromInternet"], "Should have custom SSH rule")
	assert.True(t, ruleNames["AllowHTTPSFromInternet"], "Should have HTTPS rule")

	// Verify VNet configuration
	vnetName := namer.VNetName()
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, vnetName, nil)
	require.NoError(t, err, "VNet should exist")
	assert.Equal(t, vnetName, *vnet.Name, "VNet name should match")

	require.NotNil(t, vnet.Properties, "VNet properties should be set")
	require.NotNil(t, vnet.Properties.AddressSpace, "VNet address space should be set")
	require.Len(t, vnet.Properties.AddressSpace.AddressPrefixes, 1, "VNet should have one address prefix")
	assert.Equal(t, "10.0.0.0/8", *vnet.Properties.AddressSpace.AddressPrefixes[0], "Address space should match")

	// Verify subnets
	require.Len(t, vnet.Properties.Subnets, 2, "VNet should have two subnets")

	subnetMap := make(map[string]*armnetwork.Subnet)
	for _, subnet := range vnet.Properties.Subnets {
		subnetMap[*subnet.Name] = subnet
	}

	bastionSubnet, exists := subnetMap[namer.BastionSubnetName()]
	require.True(t, exists, "Bastion subnet should exist")
	assert.Equal(t, "10.0.0.0/24", *bastionSubnet.Properties.AddressPrefix, "Bastion subnet CIDR should match")
	assert.NotNil(t, bastionSubnet.Properties.NetworkSecurityGroup, "Bastion subnet should have NSG attached")
	assert.Equal(t, *nsg.ID, *bastionSubnet.Properties.NetworkSecurityGroup.ID, "Bastion subnet should reference correct NSG")

	boxesSubnet, exists := subnetMap[namer.BoxesSubnetName()]
	require.True(t, exists, "Boxes subnet should exist")
	assert.Equal(t, "10.1.0.0/16", *boxesSubnet.Properties.AddressPrefix, "Boxes subnet CIDR should match")
	assert.Nil(t, boxesSubnet.Properties.NetworkSecurityGroup, "Boxes subnet should not have NSG attached")

	// Verify subnet IDs were set in clients
	assert.NotEmpty(t, env.Clients.BastionSubnetID, "Bastion subnet ID should be set")
	assert.NotEmpty(t, env.Clients.BoxesSubnetID, "Boxes subnet ID should be set")
	assert.Equal(t, *bastionSubnet.ID, env.Clients.BastionSubnetID, "Bastion subnet ID should match")
	assert.Equal(t, *boxesSubnet.ID, env.Clients.BoxesSubnetID, "Boxes subnet ID should match")

	// Verify table storage was initialized
	assert.NotEmpty(t, env.Clients.TableStorageConnectionString, "Table storage connection string should be set")
	assert.NotNil(t, env.Clients.TableClient, "Table client should be initialized")

	test.LogTestProgress(t, "attempting second creation to test idempotency")

	// Attempt to create again - should be idempotent
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "second creation complete, verifying idempotency")

	// Verify everything still exists and is correct after second creation
	nsg2, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "NSG should still exist after second creation")
	assert.Equal(t, *nsg.ID, *nsg2.ID, "NSG ID should be unchanged")

	vnet2, err := env.Clients.NetworkClient.Get(ctx, env.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "VNet should still exist after second creation")
	assert.Equal(t, *vnet.ID, *vnet2.ID, "VNet ID should be unchanged")
	assert.Len(t, vnet2.Properties.Subnets, 2, "VNet should still have two subnets")

	test.LogTestProgress(t, "comprehensive infrastructure verification and idempotency test complete")
}
