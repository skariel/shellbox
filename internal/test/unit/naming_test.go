//go:build unit

package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// NamingTestSuite tests the resource naming functionality
type NamingTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *NamingTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestResourceNamerBasics tests basic resource naming functionality
func (suite *NamingTestSuite) TestResourceNamerBasics() {
	suffix := "test123"
	namer := infra.NewResourceNamer(suffix)

	// Test resource group naming
	rg := namer.ResourceGroup()
	assert.Contains(suite.T(), rg, suffix, "Resource group should contain suffix")
	assert.Contains(suite.T(), rg, "shellbox", "Resource group should contain shellbox")

	// Test VNet naming
	vnet := namer.VNetName()
	assert.Contains(suite.T(), vnet, suffix, "VNet should contain suffix")
	assert.Contains(suite.T(), vnet, "vnet", "VNet should contain vnet")

	// Test subnet naming
	bastionSubnet := namer.BastionSubnetName()
	boxesSubnet := namer.BoxesSubnetName()

	assert.Contains(suite.T(), bastionSubnet, "bastion", "Bastion subnet should contain bastion")
	assert.Contains(suite.T(), boxesSubnet, "boxes", "Boxes subnet should contain boxes")
	assert.NotEqual(suite.T(), bastionSubnet, boxesSubnet, "Subnets should have different names")
}

// TestResourceNamerConsistency tests that the same suffix produces consistent names
func (suite *NamingTestSuite) TestResourceNamerConsistency() {
	suffix := "consistency-test"

	namer1 := infra.NewResourceNamer(suffix)
	namer2 := infra.NewResourceNamer(suffix)

	// Names should be identical for the same suffix
	assert.Equal(suite.T(), namer1.ResourceGroup(), namer2.ResourceGroup())
	assert.Equal(suite.T(), namer1.VNetName(), namer2.VNetName())
	assert.Equal(suite.T(), namer1.BastionSubnetName(), namer2.BastionSubnetName())
	assert.Equal(suite.T(), namer1.BoxesSubnetName(), namer2.BoxesSubnetName())
}

// TestResourceNamerUniqueness tests that different suffixes produce different names
func (suite *NamingTestSuite) TestResourceNamerUniqueness() {
	namer1 := infra.NewResourceNamer("suffix1")
	namer2 := infra.NewResourceNamer("suffix2")

	// Names should be different for different suffixes
	assert.NotEqual(suite.T(), namer1.ResourceGroup(), namer2.ResourceGroup())
	assert.NotEqual(suite.T(), namer1.VNetName(), namer2.VNetName())
	assert.NotEqual(suite.T(), namer1.BastionSubnetName(), namer2.BastionSubnetName())
	assert.NotEqual(suite.T(), namer1.BoxesSubnetName(), namer2.BoxesSubnetName())
}

// TestInstanceNaming tests VM instance naming
func (suite *NamingTestSuite) TestInstanceNaming() {
	suffix := "inst-test"
	namer := infra.NewResourceNamer(suffix)

	// Test bastion naming
	bastionName := namer.BastionVMName()
	assert.Contains(suite.T(), bastionName, "bastion", "Bastion name should contain bastion")
	assert.Contains(suite.T(), bastionName, suffix, "Bastion name should contain suffix")

	// Test box naming with UUID
	uuid := "test-uuid-123"
	boxName := namer.BoxVMName(uuid)
	assert.Contains(suite.T(), boxName, "box", "Box name should contain box")
	assert.Contains(suite.T(), boxName, uuid, "Box name should contain UUID")

	// Test computer names (shorter names for Windows compatibility)
	bastionCompName := namer.BastionComputerName()
	assert.Equal(suite.T(), "shellbox-bastion", bastionCompName, "Bastion computer name should be fixed")

	boxCompName := namer.BoxComputerName(uuid)
	assert.Contains(suite.T(), boxCompName, "shellbox-box", "Box computer name should contain shellbox-box")

	// Test with long UUID to ensure truncation
	longUUID := "very-long-uuid-that-should-be-truncated"
	boxCompNameLong := namer.BoxComputerName(longUUID)
	assert.Contains(suite.T(), boxCompNameLong, "very-lon", "Long UUID should be truncated to 8 chars")
}

// TestNetworkingNaming tests networking resource naming
func (suite *NamingTestSuite) TestNetworkingNaming() {
	suffix := "net-test"
	namer := infra.NewResourceNamer(suffix)

	// Test NSG naming
	bastionNSG := namer.BastionNSGName()
	assert.Contains(suite.T(), bastionNSG, "bastion", "Bastion NSG should contain bastion")
	assert.Contains(suite.T(), bastionNSG, "nsg", "Bastion NSG should contain nsg")

	uuid := "test-uuid-456"
	boxNSG := namer.BoxNSGName(uuid)
	assert.Contains(suite.T(), boxNSG, "box", "Box NSG should contain box")
	assert.Contains(suite.T(), boxNSG, "nsg", "Box NSG should contain nsg")
	assert.Contains(suite.T(), boxNSG, uuid, "Box NSG should contain UUID")

	// Test public IP naming
	bastionPIP := namer.BastionPublicIPName()
	assert.Contains(suite.T(), bastionPIP, "bastion", "Bastion public IP should contain bastion")
	assert.Contains(suite.T(), bastionPIP, "pip", "Bastion public IP should contain pip")

	// Test NIC naming
	bastionNIC := namer.BastionNICName()
	assert.Contains(suite.T(), bastionNIC, "bastion", "Bastion NIC should contain bastion")
	assert.Contains(suite.T(), bastionNIC, "nic", "Bastion NIC should contain nic")

	boxNIC := namer.BoxNICName(uuid)
	assert.Contains(suite.T(), boxNIC, "box", "Box NIC should contain box")
	assert.Contains(suite.T(), boxNIC, "nic", "Box NIC should contain nic")
	assert.Contains(suite.T(), boxNIC, uuid, "Box NIC should contain UUID")
}

// TestStorageNaming tests storage resource naming
func (suite *NamingTestSuite) TestStorageNaming() {
	suffix := "storage-test"
	namer := infra.NewResourceNamer(suffix)

	uuid := "test-uuid-789"

	// Test volume naming
	volumeName := namer.VolumePoolDiskName(uuid)
	assert.Contains(suite.T(), volumeName, "volume", "Volume should contain volume")
	assert.Contains(suite.T(), volumeName, uuid, "Volume should contain UUID")

	// Test snapshot naming
	snapshotName := namer.GoldenSnapshotName()
	assert.Contains(suite.T(), snapshotName, "snapshot", "Snapshot should contain snapshot")
	assert.Contains(suite.T(), snapshotName, suffix, "Snapshot should contain suffix")

	// Test storage account naming (Azure storage accounts cannot contain hyphens)
	storageAccount := namer.StorageAccountName()
	assert.Contains(suite.T(), storageAccount, "sb", "Storage account should contain sb prefix")
	assert.Contains(suite.T(), storageAccount, "storagetest", "Storage account should contain cleaned suffix")

	// Test shared storage account naming
	sharedStorageAccount := namer.SharedStorageAccountName()
	assert.Equal(suite.T(), "shellboxtest536567", sharedStorageAccount, "Shared storage account should be fixed name")

	// Test table naming (table names have cleaned suffixes with no hyphens)
	eventLogTable := namer.EventLogTableName()
	assert.Contains(suite.T(), eventLogTable, "EventLog", "EventLog table should contain EventLog")
	assert.Contains(suite.T(), eventLogTable, "storagetest", "EventLog table should contain cleaned suffix")

	resourceRegistryTable := namer.ResourceRegistryTableName()
	assert.Contains(suite.T(), resourceRegistryTable, "ResourceRegistry", "ResourceRegistry table should contain ResourceRegistry")
	assert.Contains(suite.T(), resourceRegistryTable, "storagetest", "ResourceRegistry table should contain cleaned suffix")

	// Test disk naming
	bastionOSDisk := namer.BastionOSDiskName()
	assert.Contains(suite.T(), bastionOSDisk, "bastion", "Bastion OS disk should contain bastion")
	assert.Contains(suite.T(), bastionOSDisk, "os-disk", "Bastion OS disk should contain os-disk")

	boxOSDisk := namer.BoxOSDiskName(uuid)
	assert.Contains(suite.T(), boxOSDisk, "box", "Box OS disk should contain box")
	assert.Contains(suite.T(), boxOSDisk, "os-disk", "Box OS disk should contain os-disk")
	assert.Contains(suite.T(), boxOSDisk, uuid, "Box OS disk should contain UUID")

	boxDataDisk := namer.BoxDataDiskName(uuid)
	assert.Contains(suite.T(), boxDataDisk, "box", "Box data disk should contain box")
	assert.Contains(suite.T(), boxDataDisk, "data-disk", "Box data disk should contain data-disk")
	assert.Contains(suite.T(), boxDataDisk, uuid, "Box data disk should contain UUID")
}

// TestValidResourceNames tests that generated names meet Azure requirements
func (suite *NamingTestSuite) TestValidResourceNames() {
	suffix := "valid-test"
	namer := infra.NewResourceNamer(suffix)

	// Test that names don't exceed common Azure limits and contain only valid characters
	names := []string{
		namer.ResourceGroup(),
		namer.VNetName(),
		namer.BastionSubnetName(),
		namer.BoxesSubnetName(),
		namer.BastionVMName(),
		namer.BoxVMName("test-uuid"),
		namer.BastionComputerName(),
		namer.BoxComputerName("test-uuid"),
		namer.BastionNSGName(),
		namer.BoxNSGName("test-uuid"),
		namer.BastionPublicIPName(),
		namer.BastionNICName(),
		namer.BoxNICName("test-uuid"),
		namer.BastionOSDiskName(),
		namer.BoxOSDiskName("test-uuid"),
		namer.BoxDataDiskName("test-uuid"),
		namer.VolumePoolDiskName("test-uuid"),
		namer.GoldenSnapshotName(),
		namer.StorageAccountName(),
		namer.SharedStorageAccountName(),
		namer.EventLogTableName(),
		namer.ResourceRegistryTableName(),
	}

	for _, name := range names {
		// Check length (most Azure resources have 80 char limit, but let's be conservative)
		assert.LessOrEqual(suite.T(), len(name), 60, "Resource name should not be too long: %s", name)

		// Check that name is not empty
		assert.NotEmpty(suite.T(), name, "Resource name should not be empty")

		// Check that name starts with alphanumeric
		assert.Regexp(suite.T(), "^[a-zA-Z0-9]", name, "Resource name should start with alphanumeric: %s", name)
	}
}

// TestFrameworkItself tests that our test framework is working
func (suite *NamingTestSuite) TestFrameworkItself() {
	// Test that our test environment was set up correctly
	require.NotNil(suite.T(), suite.env, "Test environment should be initialized")
	require.NotEmpty(suite.T(), suite.env.Suffix, "Test environment should have a suffix")

	// Test configuration loading
	config := test.LoadConfig()
	require.NotNil(suite.T(), config, "Test config should load successfully")
}

// Run the test suite
func TestNamingTestSuite(t *testing.T) {
	suite.Run(t, new(NamingTestSuite))
}
