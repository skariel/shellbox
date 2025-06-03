package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shellbox/internal/infra"
)

func TestResourceNamerTestSuite(t *testing.T) {
	suffix := "test123"
	namer := infra.NewResourceNamer(suffix)

	t.Run("TestBasicNaming", func(t *testing.T) {
		assert.Equal(t, "shellbox-test123", namer.ResourceGroup())
		assert.Equal(t, "shellbox-test123-vnet", namer.VNetName())
		assert.Equal(t, "shellbox-test123-bastion-subnet", namer.BastionSubnetName())
		assert.Equal(t, "shellbox-test123-boxes-subnet", namer.BoxesSubnetName())
		assert.Equal(t, "shellbox-test123-bastion-nsg", namer.BastionNSGName())
		assert.Equal(t, "shellbox-test123-bastion-vm", namer.BastionVMName())
		assert.Equal(t, "shellbox-bastion", namer.BastionComputerName())
		assert.Equal(t, "shellbox-test123-bastion-nic", namer.BastionNICName())
		assert.Equal(t, "shellbox-test123-bastion-pip", namer.BastionPublicIPName())
		assert.Equal(t, "shellbox-test123-bastion-os-disk", namer.BastionOSDiskName())
	})

	t.Run("TestBoxNaming", func(t *testing.T) {
		boxID := "abc123"
		assert.Equal(t, "shellbox-test123-box-abc123-nsg", namer.BoxNSGName(boxID))
		assert.Equal(t, "shellbox-test123-box-abc123-vm", namer.BoxVMName(boxID))
		assert.Equal(t, "shellbox-box-abc123", namer.BoxComputerName(boxID))
		assert.Equal(t, "shellbox-test123-box-abc123-nic", namer.BoxNICName(boxID))
		assert.Equal(t, "shellbox-test123-box-abc123-os-disk", namer.BoxOSDiskName(boxID))
		assert.Equal(t, "shellbox-test123-box-abc123-data-disk", namer.BoxDataDiskName(boxID))
	})

	t.Run("TestVolumeNaming", func(t *testing.T) {
		volumeID := "vol456"
		assert.Equal(t, "shellbox-test123-volume-vol456", namer.VolumePoolDiskName(volumeID))
	})

	t.Run("TestStorageAccountNaming", func(t *testing.T) {
		// Storage account names must be lowercase letters and numbers only
		namer := infra.NewResourceNamer("test123")
		assert.Equal(t, "sbtest123", namer.StorageAccountName())

		// Test with hyphens and uppercase (should be cleaned - uppercase gets filtered out)
		namer2 := infra.NewResourceNamer("Test-456")
		assert.Equal(t, "sbest456", namer2.StorageAccountName())

		// Test length truncation (24 char limit)
		namer3 := infra.NewResourceNamer("verylongsuffixthatwillbetruncated")
		result := namer3.StorageAccountName()
		assert.LessOrEqual(t, len(result), 24)
		assert.True(t, len(result) >= 2 && result[:2] == "sb")
	})

	t.Run("TestTableNaming", func(t *testing.T) {
		namer := infra.NewResourceNamer("test123")
		assert.Equal(t, "EventLogtest123", namer.EventLogTableName())
		assert.Equal(t, "ResourceRegistrytest123", namer.ResourceRegistryTableName())

		// Test with special characters (should be cleaned)
		namer2 := infra.NewResourceNamer("test-456")
		assert.Equal(t, "EventLogtest456", namer2.EventLogTableName())
		assert.Equal(t, "ResourceRegistrytest456", namer2.ResourceRegistryTableName())
	})

	t.Run("TestGoldenSnapshotNaming", func(t *testing.T) {
		assert.Equal(t, "shellbox-test123-golden-snapshot", namer.GoldenSnapshotName())
	})

	t.Run("TestSharedStorageAccountName", func(t *testing.T) {
		assert.Equal(t, "shellboxtest536567", namer.SharedStorageAccountName())
	})

	t.Run("TestBoxComputerNameTruncation", func(t *testing.T) {
		// Test long box ID truncation (8 char limit)
		longBoxID := "verylongboxid123456789"
		result := namer.BoxComputerName(longBoxID)
		assert.Equal(t, "shellbox-box-verylong", result)

		// Test short box ID (no truncation)
		shortBoxID := "short"
		result2 := namer.BoxComputerName(shortBoxID)
		assert.Equal(t, "shellbox-box-short", result2)
	})
}
