//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestCreateVirtualNetwork(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	namer := infra.NewResourceNamer(env.Clients.Suffix)

	// Create VNet parameters
	vnetParams := armnetwork.VirtualNetwork{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.0.0.0/8")},
			},
		},
	}

	// Create VNet
	poller, err := env.Clients.NetworkClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, namer.VNetName(), vnetParams, nil)
	require.NoError(t, err, "should start VNet creation without error")

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete VNet creation without error")

	// Verify VNet properties
	assert.NotNil(t, result.VirtualNetwork.ID, "VNet should have an ID")
	assert.Equal(t, namer.VNetName(), *result.VirtualNetwork.Name, "VNet should have correct name")
	assert.Equal(t, infra.Location, *result.VirtualNetwork.Location, "VNet should be in correct location")
	assert.Equal(t, "10.0.0.0/8", *result.VirtualNetwork.Properties.AddressSpace.AddressPrefixes[0], "VNet should have correct address space")

	// Verify VNet can be retrieved
	getResult, err := env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should be able to retrieve created VNet")
	assert.Equal(t, *result.VirtualNetwork.ID, *getResult.VirtualNetwork.ID, "retrieved VNet should have same ID")

	// Test VNet deletion
	deletePoller, err := env.Clients.NetworkClient.BeginDelete(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should start VNet deletion without error")

	_, err = deletePoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete VNet deletion without error")

	// Verify VNet is deleted
	_, err = env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	assert.Error(t, err, "should not be able to retrieve deleted VNet")
}

func TestCreateNetworkSecurityGroup(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	namer := infra.NewResourceNamer(env.Clients.Suffix)

	// Create NSG with bastion security rules
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: infra.BastionNSGRules,
		},
	}

	// Create NSG
	poller, err := env.Clients.NSGClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nsgParams, nil)
	require.NoError(t, err, "should start NSG creation without error")

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete NSG creation without error")

	// Verify NSG properties
	assert.NotNil(t, result.SecurityGroup.ID, "NSG should have an ID")
	assert.Equal(t, namer.BastionNSGName(), *result.SecurityGroup.Name, "NSG should have correct name")
	assert.Equal(t, infra.Location, *result.SecurityGroup.Location, "NSG should be in correct location")
	assert.Len(t, result.SecurityGroup.Properties.SecurityRules, len(infra.BastionNSGRules), "NSG should have correct number of security rules")

	// Verify specific security rules
	foundSSHRule := false
	foundHTTPSRule := false
	for _, rule := range result.SecurityGroup.Properties.SecurityRules {
		switch *rule.Name {
		case "AllowSSHFromInternet":
			foundSSHRule = true
			assert.Equal(t, "22", *rule.Properties.DestinationPortRange, "SSH rule should allow port 22")
			assert.Equal(t, armnetwork.SecurityRuleAccessAllow, *rule.Properties.Access, "SSH rule should allow access")
		case "AllowHTTPSFromInternet":
			foundHTTPSRule = true
			assert.Equal(t, "443", *rule.Properties.DestinationPortRange, "HTTPS rule should allow port 443")
			assert.Equal(t, armnetwork.SecurityRuleAccessAllow, *rule.Properties.Access, "HTTPS rule should allow access")
		}
	}
	assert.True(t, foundSSHRule, "should find SSH security rule")
	assert.True(t, foundHTTPSRule, "should find HTTPS security rule")

	// Verify NSG can be retrieved
	getResult, err := env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should be able to retrieve created NSG")
	assert.Equal(t, *result.SecurityGroup.ID, *getResult.SecurityGroup.ID, "retrieved NSG should have same ID")

	// Test NSG deletion
	deletePoller, err := env.Clients.NSGClient.BeginDelete(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should start NSG deletion without error")

	_, err = deletePoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete NSG deletion without error")

	// Verify NSG is deleted
	_, err = env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	assert.Error(t, err, "should not be able to retrieve deleted NSG")
}

func TestCreateSubnet(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	namer := infra.NewResourceNamer(env.Clients.Suffix)

	// First create a VNet to hold the subnet
	vnetParams := armnetwork.VirtualNetwork{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.0.0.0/8")},
			},
		},
	}

	vnetPoller, err := env.Clients.NetworkClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, namer.VNetName(), vnetParams, nil)
	require.NoError(t, err, "should start VNet creation without error")

	_, err = vnetPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete VNet creation without error")

	// Create a subnet
	subnetParams := armnetwork.Subnet{
		Properties: &armnetwork.SubnetPropertiesFormat{
			AddressPrefix: to.Ptr("10.0.1.0/24"),
		},
	}

	subnetPoller, err := env.Clients.SubnetsClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, namer.VNetName(), "test-subnet", subnetParams, nil)
	require.NoError(t, err, "should start subnet creation without error")

	subnetResult, err := subnetPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete subnet creation without error")

	// Verify subnet properties
	assert.NotNil(t, subnetResult.Subnet.ID, "subnet should have an ID")
	assert.Equal(t, "test-subnet", *subnetResult.Subnet.Name, "subnet should have correct name")
	assert.Equal(t, "10.0.1.0/24", *subnetResult.Subnet.Properties.AddressPrefix, "subnet should have correct address prefix")

	// Verify subnet is part of the VNet
	updatedVNet, err := env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should be able to retrieve VNet")
	assert.Len(t, updatedVNet.VirtualNetwork.Properties.Subnets, 1, "VNet should contain one subnet")
	assert.Equal(t, "test-subnet", *updatedVNet.VirtualNetwork.Properties.Subnets[0].Name, "VNet should contain our subnet")

	// Test subnet deletion
	deletePoller, err := env.Clients.SubnetsClient.BeginDelete(ctx, env.Clients.ResourceGroupName, namer.VNetName(), "test-subnet", nil)
	require.NoError(t, err, "should start subnet deletion without error")

	_, err = deletePoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete subnet deletion without error")

	// Verify subnet is deleted from VNet
	finalVNet, err := env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should be able to retrieve VNet after subnet deletion")
	assert.Len(t, finalVNet.VirtualNetwork.Properties.Subnets, 0, "VNet should not contain any subnets after deletion")

	// Clean up VNet
	vnetDeletePoller, err := env.Clients.NetworkClient.BeginDelete(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should start VNet cleanup without error")

	_, err = vnetDeletePoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete VNet cleanup without error")
}

func TestNetworkSecurityGroupWithSubnet(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	namer := infra.NewResourceNamer(env.Clients.Suffix)

	// Create NSG first
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: infra.BastionNSGRules,
		},
	}

	nsgPoller, err := env.Clients.NSGClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nsgParams, nil)
	require.NoError(t, err, "should start NSG creation without error")

	nsgResult, err := nsgPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete NSG creation without error")

	// Create VNet with subnet that references the NSG
	vnetParams := armnetwork.VirtualNetwork{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{to.Ptr("10.0.0.0/8")},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: to.Ptr(namer.BastionSubnetName()),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: to.Ptr("10.0.0.0/24"),
						NetworkSecurityGroup: &armnetwork.SecurityGroup{
							ID: nsgResult.SecurityGroup.ID,
						},
					},
				},
			},
		},
	}

	vnetPoller, err := env.Clients.NetworkClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, namer.VNetName(), vnetParams, nil)
	require.NoError(t, err, "should start VNet creation without error")

	vnetResult, err := vnetPoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete VNet creation without error")

	// Verify subnet has NSG attached
	assert.Len(t, vnetResult.VirtualNetwork.Properties.Subnets, 1, "VNet should have one subnet")
	subnet := vnetResult.VirtualNetwork.Properties.Subnets[0]
	assert.NotNil(t, subnet.Properties.NetworkSecurityGroup, "subnet should have NSG attached")
	assert.Equal(t, *nsgResult.SecurityGroup.ID, *subnet.Properties.NetworkSecurityGroup.ID, "subnet should reference correct NSG")

	// Verify NSG shows the subnet association
	updatedNSG, err := env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should be able to retrieve NSG")
	assert.Len(t, updatedNSG.SecurityGroup.Properties.Subnets, 1, "NSG should show one associated subnet")
	assert.Equal(t, *subnet.ID, *updatedNSG.SecurityGroup.Properties.Subnets[0].ID, "NSG should reference correct subnet")

	// Test cleanup order: VNet first, then NSG
	vnetDeletePoller, err := env.Clients.NetworkClient.BeginDelete(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should start VNet deletion without error")

	_, err = vnetDeletePoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete VNet deletion without error")

	nsgDeletePoller, err := env.Clients.NSGClient.BeginDelete(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should start NSG deletion without error")

	_, err = nsgDeletePoller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete NSG deletion without error")
}

func TestNetworkInfrastructureIntegration(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// Test the full CreateNetworkInfrastructure function
	infra.CreateNetworkInfrastructure(ctx, env.Clients, true)

	namer := infra.NewResourceNamer(env.Clients.Suffix)

	// Verify resource group exists with correct config hash
	rg, err := env.Clients.ResourceClient.Get(ctx, env.Clients.ResourceGroupName, nil)
	require.NoError(t, err, "should be able to retrieve resource group")
	assert.NotNil(t, rg.ResourceGroup.Tags["config"], "resource group should have config tag")

	// Verify NSG was created with correct rules
	nsg, err := env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should be able to retrieve NSG")
	assert.Len(t, nsg.SecurityGroup.Properties.SecurityRules, len(infra.BastionNSGRules), "NSG should have correct number of security rules")

	// Verify VNet was created with correct subnets
	vnet, err := env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should be able to retrieve VNet")
	assert.Equal(t, "10.0.0.0/8", *vnet.VirtualNetwork.Properties.AddressSpace.AddressPrefixes[0], "VNet should have correct address space")
	assert.Len(t, vnet.VirtualNetwork.Properties.Subnets, 2, "VNet should have two subnets")

	// Verify subnets have correct properties
	bastionSubnetFound := false
	boxesSubnetFound := false
	for _, subnet := range vnet.VirtualNetwork.Properties.Subnets {
		switch *subnet.Name {
		case namer.BastionSubnetName():
			bastionSubnetFound = true
			assert.Equal(t, "10.0.0.0/24", *subnet.Properties.AddressPrefix, "bastion subnet should have correct CIDR")
			assert.NotNil(t, subnet.Properties.NetworkSecurityGroup, "bastion subnet should have NSG attached")
			assert.Equal(t, *nsg.SecurityGroup.ID, *subnet.Properties.NetworkSecurityGroup.ID, "bastion subnet should reference correct NSG")
		case namer.BoxesSubnetName():
			boxesSubnetFound = true
			assert.Equal(t, "10.1.0.0/16", *subnet.Properties.AddressPrefix, "boxes subnet should have correct CIDR")
		}
	}
	assert.True(t, bastionSubnetFound, "should find bastion subnet")
	assert.True(t, boxesSubnetFound, "should find boxes subnet")

	// Verify subnet IDs were set correctly in clients
	assert.NotEmpty(t, env.Clients.BastionSubnetID, "bastion subnet ID should be set")
	assert.NotEmpty(t, env.Clients.BoxesSubnetID, "boxes subnet ID should be set")
	assert.Contains(t, env.Clients.BastionSubnetID, namer.BastionSubnetName(), "bastion subnet ID should contain correct name")
	assert.Contains(t, env.Clients.BoxesSubnetID, namer.BoxesSubnetName(), "boxes subnet ID should contain correct name")

	// Verify Table Storage was initialized
	assert.NotEmpty(t, env.Clients.TableStorageConnectionString, "table storage connection string should be set")
	assert.NotNil(t, env.Clients.TableClient, "table client should be initialized")
}

func TestNetworkInfrastructureIdempotency(t *testing.T) {
	t.Parallel()
	test.RequireCategory(t, test.CategoryIntegration)

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Create network infrastructure first time
	infra.CreateNetworkInfrastructure(ctx, env.Clients, true)

	namer := infra.NewResourceNamer(env.Clients.Suffix)

	// Get initial resource IDs
	initialNSG, err := env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should be able to retrieve initial NSG")

	initialVNet, err := env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should be able to retrieve initial VNet")

	initialBastionSubnetID := env.Clients.BastionSubnetID
	initialBoxesSubnetID := env.Clients.BoxesSubnetID

	// Run network infrastructure creation again (should be idempotent)
	infra.CreateNetworkInfrastructure(ctx, env.Clients, true)

	// Verify resources still exist with same IDs
	finalNSG, err := env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, namer.BastionNSGName(), nil)
	require.NoError(t, err, "should be able to retrieve NSG after second creation")
	assert.Equal(t, *initialNSG.SecurityGroup.ID, *finalNSG.SecurityGroup.ID, "NSG ID should remain same after idempotent operation")

	finalVNet, err := env.Clients.NetworkClient.Get(ctx, env.Clients.ResourceGroupName, namer.VNetName(), nil)
	require.NoError(t, err, "should be able to retrieve VNet after second creation")
	assert.Equal(t, *initialVNet.VirtualNetwork.ID, *finalVNet.VirtualNetwork.ID, "VNet ID should remain same after idempotent operation")

	// Verify subnet IDs are still correct
	assert.Equal(t, initialBastionSubnetID, env.Clients.BastionSubnetID, "bastion subnet ID should remain same")
	assert.Equal(t, initialBoxesSubnetID, env.Clients.BoxesSubnetID, "boxes subnet ID should remain same")
}
