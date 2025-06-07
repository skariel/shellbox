//go:build unit

package unit

import (
	"strings"
	"testing"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// TestResourceNamerBasics tests basic resource naming functionality
func TestResourceNamerBasics(t *testing.T) {
	suffix := "test123"
	namer := infra.NewResourceNamer(suffix)

	// Test resource group naming
	rg := namer.ResourceGroup()
	if !strings.Contains(rg, suffix) {
		t.Errorf("Resource group should contain suffix %q, got %q", suffix, rg)
	}
	if !strings.Contains(rg, "shellbox") {
		t.Errorf("Resource group should contain shellbox, got %q", rg)
	}

	// Test VNet naming
	vnet := namer.VNetName()
	if !strings.Contains(vnet, suffix) {
		t.Errorf("VNet should contain suffix %q, got %q", suffix, vnet)
	}
	if !strings.Contains(vnet, "vnet") {
		t.Errorf("VNet should contain vnet, got %q", vnet)
	}

	// Test subnet naming
	bastionSubnet := namer.BastionSubnetName()
	boxesSubnet := namer.BoxesSubnetName()

	if !strings.Contains(bastionSubnet, "bastion") {
		t.Errorf("Bastion subnet should contain bastion, got %q", bastionSubnet)
	}
	if !strings.Contains(boxesSubnet, "boxes") {
		t.Errorf("Boxes subnet should contain boxes, got %q", boxesSubnet)
	}
	if bastionSubnet == boxesSubnet {
		t.Errorf("Subnets should have different names, both got %q", bastionSubnet)
	}
}

// TestResourceNamerConsistency tests that the same suffix produces consistent names
func TestResourceNamerConsistency(t *testing.T) {
	suffix := "consistency-test"

	namer1 := infra.NewResourceNamer(suffix)
	namer2 := infra.NewResourceNamer(suffix)

	// Names should be identical for the same suffix
	if namer1.ResourceGroup() != namer2.ResourceGroup() {
		t.Errorf("Resource groups should be equal: %q vs %q", namer1.ResourceGroup(), namer2.ResourceGroup())
	}
	if namer1.VNetName() != namer2.VNetName() {
		t.Errorf("VNet names should be equal: %q vs %q", namer1.VNetName(), namer2.VNetName())
	}
	if namer1.BastionSubnetName() != namer2.BastionSubnetName() {
		t.Errorf("Bastion subnet names should be equal: %q vs %q", namer1.BastionSubnetName(), namer2.BastionSubnetName())
	}
	if namer1.BoxesSubnetName() != namer2.BoxesSubnetName() {
		t.Errorf("Boxes subnet names should be equal: %q vs %q", namer1.BoxesSubnetName(), namer2.BoxesSubnetName())
	}
}

// TestResourceNamerUniqueness tests that different suffixes produce different names
func TestResourceNamerUniqueness(t *testing.T) {
	namer1 := infra.NewResourceNamer("suffix1")
	namer2 := infra.NewResourceNamer("suffix2")

	// Names should be different for different suffixes
	if namer1.ResourceGroup() == namer2.ResourceGroup() {
		t.Errorf("Resource groups should be different, both got %q", namer1.ResourceGroup())
	}
	if namer1.VNetName() == namer2.VNetName() {
		t.Errorf("VNet names should be different, both got %q", namer1.VNetName())
	}
	if namer1.BastionSubnetName() == namer2.BastionSubnetName() {
		t.Errorf("Bastion subnet names should be different, both got %q", namer1.BastionSubnetName())
	}
	if namer1.BoxesSubnetName() == namer2.BoxesSubnetName() {
		t.Errorf("Boxes subnet names should be different, both got %q", namer1.BoxesSubnetName())
	}
}

// TestInstanceNaming tests VM instance naming
func TestInstanceNaming(t *testing.T) {
	suffix := "inst-test"
	namer := infra.NewResourceNamer(suffix)

	// Test bastion naming
	bastionName := namer.BastionVMName()
	if !strings.Contains(bastionName, "bastion") {
		t.Errorf("Bastion name should contain bastion, got %q", bastionName)
	}
	if !strings.Contains(bastionName, suffix) {
		t.Errorf("Bastion name should contain suffix %q, got %q", suffix, bastionName)
	}

	// Test box naming with UUID
	uuid := "test-uuid-123"
	boxName := namer.BoxVMName(uuid)
	if !strings.Contains(boxName, "box") {
		t.Errorf("Box name should contain box, got %q", boxName)
	}
	if !strings.Contains(boxName, uuid) {
		t.Errorf("Box name should contain UUID %q, got %q", uuid, boxName)
	}

	// Test computer names (shorter names for Windows compatibility)
	bastionCompName := namer.BastionComputerName()
	if bastionCompName != "shellbox-bastion" {
		t.Errorf("Bastion computer name should be fixed, expected %q, got %q", "shellbox-bastion", bastionCompName)
	}

	boxCompName := namer.BoxComputerName(uuid)
	if !strings.Contains(boxCompName, "shellbox-box") {
		t.Errorf("Box computer name should contain shellbox-box, got %q", boxCompName)
	}

	// Test with long UUID to ensure truncation
	longUUID := "very-long-uuid-that-should-be-truncated"
	boxCompNameLong := namer.BoxComputerName(longUUID)
	if !strings.Contains(boxCompNameLong, "very-lon") {
		t.Errorf("Long UUID should be truncated to 8 chars, expected to contain 'very-lon', got %q", boxCompNameLong)
	}
}

// TestNetworkingNaming tests networking resource naming
func TestNetworkingNaming(t *testing.T) {
	suffix := "net-test"
	namer := infra.NewResourceNamer(suffix)

	// Test NSG naming
	bastionNSG := namer.BastionNSGName()
	if !strings.Contains(bastionNSG, "bastion") {
		t.Errorf("Bastion NSG should contain bastion, got %q", bastionNSG)
	}
	if !strings.Contains(bastionNSG, "nsg") {
		t.Errorf("Bastion NSG should contain nsg, got %q", bastionNSG)
	}

	uuid := "test-uuid-456"
	boxNSG := namer.BoxNSGName(uuid)
	if !strings.Contains(boxNSG, "box") {
		t.Errorf("Box NSG should contain box, got %q", boxNSG)
	}
	if !strings.Contains(boxNSG, "nsg") {
		t.Errorf("Box NSG should contain nsg, got %q", boxNSG)
	}
	if !strings.Contains(boxNSG, uuid) {
		t.Errorf("Box NSG should contain UUID %q, got %q", uuid, boxNSG)
	}

	// Test public IP naming
	bastionPIP := namer.BastionPublicIPName()
	if !strings.Contains(bastionPIP, "bastion") {
		t.Errorf("Bastion public IP should contain bastion, got %q", bastionPIP)
	}
	if !strings.Contains(bastionPIP, "pip") {
		t.Errorf("Bastion public IP should contain pip, got %q", bastionPIP)
	}

	// Test NIC naming
	bastionNIC := namer.BastionNICName()
	if !strings.Contains(bastionNIC, "bastion") {
		t.Errorf("Bastion NIC should contain bastion, got %q", bastionNIC)
	}
	if !strings.Contains(bastionNIC, "nic") {
		t.Errorf("Bastion NIC should contain nic, got %q", bastionNIC)
	}

	boxNIC := namer.BoxNICName(uuid)
	if !strings.Contains(boxNIC, "box") {
		t.Errorf("Box NIC should contain box, got %q", boxNIC)
	}
	if !strings.Contains(boxNIC, "nic") {
		t.Errorf("Box NIC should contain nic, got %q", boxNIC)
	}
	if !strings.Contains(boxNIC, uuid) {
		t.Errorf("Box NIC should contain UUID %q, got %q", uuid, boxNIC)
	}
}

// TestStorageNaming tests storage resource naming
func TestStorageNaming(t *testing.T) {
	suffix := "storage-test"
	namer := infra.NewResourceNamer(suffix)

	uuid := "test-uuid-789"

	// Test volume naming
	volumeName := namer.VolumePoolDiskName(uuid)
	if !strings.Contains(volumeName, "volume") {
		t.Errorf("Volume should contain volume, got %q", volumeName)
	}
	if !strings.Contains(volumeName, uuid) {
		t.Errorf("Volume should contain UUID %q, got %q", uuid, volumeName)
	}

	// Test snapshot naming
	snapshotName := namer.GoldenSnapshotName()
	if !strings.Contains(snapshotName, "snapshot") {
		t.Errorf("Snapshot should contain snapshot, got %q", snapshotName)
	}
	if !strings.Contains(snapshotName, suffix) {
		t.Errorf("Snapshot should contain suffix %q, got %q", suffix, snapshotName)
	}

	// Test storage account naming (Azure storage accounts cannot contain hyphens)
	storageAccount := namer.StorageAccountName()
	if !strings.Contains(storageAccount, "sb") {
		t.Errorf("Storage account should contain sb prefix, got %q", storageAccount)
	}
	if !strings.Contains(storageAccount, "storagetest") {
		t.Errorf("Storage account should contain cleaned suffix 'storagetest', got %q", storageAccount)
	}

	// Test shared storage account naming
	sharedStorageAccount := namer.SharedStorageAccountName()
	if sharedStorageAccount != "shellboxtest536567" {
		t.Errorf("Shared storage account should be fixed name, expected %q, got %q", "shellboxtest536567", sharedStorageAccount)
	}

	// Test table naming (table names have cleaned suffixes with no hyphens)
	eventLogTable := namer.EventLogTableName()
	if !strings.Contains(eventLogTable, "EventLog") {
		t.Errorf("EventLog table should contain EventLog, got %q", eventLogTable)
	}
	if !strings.Contains(eventLogTable, "storagetest") {
		t.Errorf("EventLog table should contain cleaned suffix 'storagetest', got %q", eventLogTable)
	}

	resourceRegistryTable := namer.ResourceRegistryTableName()
	if !strings.Contains(resourceRegistryTable, "ResourceRegistry") {
		t.Errorf("ResourceRegistry table should contain ResourceRegistry, got %q", resourceRegistryTable)
	}
	if !strings.Contains(resourceRegistryTable, "storagetest") {
		t.Errorf("ResourceRegistry table should contain cleaned suffix 'storagetest', got %q", resourceRegistryTable)
	}

	// Test disk naming
	bastionOSDisk := namer.BastionOSDiskName()
	if !strings.Contains(bastionOSDisk, "bastion") {
		t.Errorf("Bastion OS disk should contain bastion, got %q", bastionOSDisk)
	}
	if !strings.Contains(bastionOSDisk, "os-disk") {
		t.Errorf("Bastion OS disk should contain os-disk, got %q", bastionOSDisk)
	}

	boxOSDisk := namer.BoxOSDiskName(uuid)
	if !strings.Contains(boxOSDisk, "box") {
		t.Errorf("Box OS disk should contain box, got %q", boxOSDisk)
	}
	if !strings.Contains(boxOSDisk, "os-disk") {
		t.Errorf("Box OS disk should contain os-disk, got %q", boxOSDisk)
	}
	if !strings.Contains(boxOSDisk, uuid) {
		t.Errorf("Box OS disk should contain UUID %q, got %q", uuid, boxOSDisk)
	}

	boxDataDisk := namer.BoxDataDiskName(uuid)
	if !strings.Contains(boxDataDisk, "box") {
		t.Errorf("Box data disk should contain box, got %q", boxDataDisk)
	}
	if !strings.Contains(boxDataDisk, "data-disk") {
		t.Errorf("Box data disk should contain data-disk, got %q", boxDataDisk)
	}
	if !strings.Contains(boxDataDisk, uuid) {
		t.Errorf("Box data disk should contain UUID %q, got %q", uuid, boxDataDisk)
	}
}

// TestValidResourceNames tests that generated names meet Azure requirements
func TestValidResourceNames(t *testing.T) {
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
		if len(name) > 60 {
			t.Errorf("Resource name should not be too long (>60 chars): %s (len=%d)", name, len(name))
		}

		// Check that name is not empty
		if name == "" {
			t.Error("Resource name should not be empty")
		}

		// Check that name starts with alphanumeric
		if len(name) > 0 && !((name[0] >= 'a' && name[0] <= 'z') || (name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= '0' && name[0] <= '9')) {
			t.Errorf("Resource name should start with alphanumeric: %s", name)
		}
	}
}

// TestFrameworkItself tests that our test framework is working
func TestFrameworkItself(t *testing.T) {
	// Test that our test environment was set up correctly
	env := test.SetupMinimalTestEnvironment(t)
	if env == nil {
		t.Fatal("Test environment should be initialized")
	}
	if env.Suffix == "" {
		t.Fatal("Test environment should have a suffix")
	}

	// Test configuration loading
	config := test.LoadConfig()
	if config == nil {
		t.Fatal("Test config should load successfully")
	}
}
