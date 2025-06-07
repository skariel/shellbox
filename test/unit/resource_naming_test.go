package unit

import (
	"testing"

	"shellbox/internal/infra"
)

// Helper function to reduce cyclomatic complexity in naming tests
func verifyNameMapping(t *testing.T, expected, actual map[string]string) {
	t.Helper()
	for key, expectedValue := range expected {
		if actualValue := actual[key]; actualValue != expectedValue {
			t.Errorf("%s = %q, want %q", key, actualValue, expectedValue)
		}
	}
}

func testBasicNaming(t *testing.T, namer *infra.ResourceNamer) {
	t.Helper()
	expected := map[string]string{
		"ResourceGroup":       "shellbox-test123",
		"VNetName":            "shellbox-test123-vnet",
		"BastionSubnetName":   "shellbox-test123-bastion-subnet",
		"BoxesSubnetName":     "shellbox-test123-boxes-subnet",
		"BastionNSGName":      "shellbox-test123-bastion-nsg",
		"BastionVMName":       "shellbox-test123-bastion-vm",
		"BastionComputerName": "shellbox-bastion",
		"BastionNICName":      "shellbox-test123-bastion-nic",
		"BastionPublicIPName": "shellbox-test123-bastion-pip",
		"BastionOSDiskName":   "shellbox-test123-bastion-os-disk",
	}
	actual := map[string]string{
		"ResourceGroup":       namer.ResourceGroup(),
		"VNetName":            namer.VNetName(),
		"BastionSubnetName":   namer.BastionSubnetName(),
		"BoxesSubnetName":     namer.BoxesSubnetName(),
		"BastionNSGName":      namer.BastionNSGName(),
		"BastionVMName":       namer.BastionVMName(),
		"BastionComputerName": namer.BastionComputerName(),
		"BastionNICName":      namer.BastionNICName(),
		"BastionPublicIPName": namer.BastionPublicIPName(),
		"BastionOSDiskName":   namer.BastionOSDiskName(),
	}
	verifyNameMapping(t, expected, actual)
}

func testBoxNaming(t *testing.T, namer *infra.ResourceNamer) {
	t.Helper()
	boxID := "abc123"
	expected := map[string]string{
		"BoxNSGName":      "shellbox-test123-box-abc123-nsg",
		"BoxVMName":       "shellbox-test123-box-abc123-vm",
		"BoxComputerName": "shellbox-box-abc123",
		"BoxNICName":      "shellbox-test123-box-abc123-nic",
		"BoxOSDiskName":   "shellbox-test123-box-abc123-os-disk",
		"BoxDataDiskName": "shellbox-test123-box-abc123-data-disk",
	}
	actual := map[string]string{
		"BoxNSGName":      namer.BoxNSGName(boxID),
		"BoxVMName":       namer.BoxVMName(boxID),
		"BoxComputerName": namer.BoxComputerName(boxID),
		"BoxNICName":      namer.BoxNICName(boxID),
		"BoxOSDiskName":   namer.BoxOSDiskName(boxID),
		"BoxDataDiskName": namer.BoxDataDiskName(boxID),
	}
	verifyNameMapping(t, expected, actual)
}

func testStorageAccountNaming(t *testing.T) {
	t.Helper()
	testCases := []struct {
		suffix   string
		expected string
		name     string
	}{
		{"test123", "sbtest123", "basic"},
		{"Test-456", "sbest456", "with special chars"},
	}

	for _, tc := range testCases {
		namer := infra.NewResourceNamer(tc.suffix)
		if actual := namer.StorageAccountName(); actual != tc.expected {
			t.Errorf("StorageAccountName() for %s = %q, want %q", tc.name, actual, tc.expected)
		}
	}

	namer3 := infra.NewResourceNamer("verylongsuffixthatwillbetruncated")
	result := namer3.StorageAccountName()
	if len(result) > 24 {
		t.Errorf("StorageAccountName() length = %d, should be <= 24", len(result))
	}
	if len(result) < 2 || result[:2] != "sb" {
		t.Errorf("StorageAccountName() = %q, should start with 'sb'", result)
	}
}

func testTableNaming(t *testing.T) {
	t.Helper()
	testCases := []struct {
		suffix   string
		expected map[string]string
		name     string
	}{
		{
			"test123",
			map[string]string{
				"EventLog":         "EventLogtest123",
				"ResourceRegistry": "ResourceRegistrytest123",
			},
			"basic",
		},
		{
			"test-456",
			map[string]string{
				"EventLog":         "EventLogtest456",
				"ResourceRegistry": "ResourceRegistrytest456",
			},
			"with special chars",
		},
	}

	for _, tc := range testCases {
		namer := infra.NewResourceNamer(tc.suffix)
		if actual := namer.EventLogTableName(); actual != tc.expected["EventLog"] {
			t.Errorf("EventLogTableName() for %s = %q, want %q", tc.name, actual, tc.expected["EventLog"])
		}
		if actual := namer.ResourceRegistryTableName(); actual != tc.expected["ResourceRegistry"] {
			t.Errorf("ResourceRegistryTableName() for %s = %q, want %q", tc.name, actual, tc.expected["ResourceRegistry"])
		}
	}
}

func TestResourceNamerTestSuite(t *testing.T) {
	suffix := "test123"
	namer := infra.NewResourceNamer(suffix)

	t.Run("TestBasicNaming", func(t *testing.T) {
		testBasicNaming(t, namer)
	})

	t.Run("TestBoxNaming", func(t *testing.T) {
		testBoxNaming(t, namer)
	})

	t.Run("TestVolumeNaming", func(t *testing.T) {
		volumeID := "vol456"
		expected := "shellbox-test123-volume-vol456"
		actual := namer.VolumePoolDiskName(volumeID)
		if actual != expected {
			t.Errorf("VolumePoolDiskName(%q) = %q, want %q", volumeID, actual, expected)
		}
	})

	t.Run("TestStorageAccountNaming", func(t *testing.T) {
		testStorageAccountNaming(t)
	})

	t.Run("TestTableNaming", func(t *testing.T) {
		testTableNaming(t)
	})

	t.Run("TestGoldenSnapshotNaming", func(t *testing.T) {
		expected := "shellbox-test123-golden-snapshot"
		actual := namer.GoldenSnapshotName()
		if actual != expected {
			t.Errorf("GoldenSnapshotName() = %q, want %q", actual, expected)
		}
	})

	t.Run("TestSharedStorageAccountName", func(t *testing.T) {
		expected := "shellboxtest536567"
		actual := namer.SharedStorageAccountName()
		if actual != expected {
			t.Errorf("SharedStorageAccountName() = %q, want %q", actual, expected)
		}
	})

	t.Run("TestBoxComputerNameTruncation", func(t *testing.T) {
		// Test long box ID truncation (8 char limit)
		longBoxID := "verylongboxid123456789"
		result := namer.BoxComputerName(longBoxID)
		expected := "shellbox-box-verylong"
		if result != expected {
			t.Errorf("BoxComputerName(%q) = %q, want %q", longBoxID, result, expected)
		}

		// Test short box ID (no truncation)
		shortBoxID := "short"
		result2 := namer.BoxComputerName(shortBoxID)
		expected2 := "shellbox-box-short"
		if result2 != expected2 {
			t.Errorf("BoxComputerName(%q) = %q, want %q", shortBoxID, result2, expected2)
		}
	})
}
