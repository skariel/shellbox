package infra

import (
	"fmt"
	"strings"
)

type ResourceNamer struct {
	suffix string
}

func NewResourceNamer(suffix string) *ResourceNamer {
	return &ResourceNamer{suffix: suffix}
}

func (r *ResourceNamer) ResourceGroup() string {
	return fmt.Sprintf("shellbox-%s", r.suffix)
}

func (r *ResourceNamer) VNetName() string {
	return fmt.Sprintf("shellbox-%s-vnet", r.suffix)
}

func (r *ResourceNamer) BastionSubnetName() string {
	return fmt.Sprintf("shellbox-%s-bastion-subnet", r.suffix)
}

func (r *ResourceNamer) BoxesSubnetName() string {
	return fmt.Sprintf("shellbox-%s-boxes-subnet", r.suffix)
}

func (r *ResourceNamer) BastionNSGName() string {
	return fmt.Sprintf("shellbox-%s-bastion-nsg", r.suffix)
}

func (r *ResourceNamer) BoxNSGName(boxID string) string {
	return fmt.Sprintf("shellbox-%s-box-%s-nsg", r.suffix, boxID)
}

func (r *ResourceNamer) BastionVMName() string {
	return fmt.Sprintf("shellbox-%s-bastion-vm", r.suffix)
}

func (r *ResourceNamer) BoxVMName(boxID string) string {
	return fmt.Sprintf("shellbox-%s-box-%s-vm", r.suffix, boxID)
}

func (r *ResourceNamer) BastionComputerName() string {
	return "shellbox-bastion"
}

func (r *ResourceNamer) BoxComputerName(boxID string) string {
	if len(boxID) > 8 {
		return fmt.Sprintf("shellbox-box-%s", boxID[:8])
	}
	return fmt.Sprintf("shellbox-box-%s", boxID)
}

func (r *ResourceNamer) BastionNICName() string {
	return fmt.Sprintf("shellbox-%s-bastion-nic", r.suffix)
}

func (r *ResourceNamer) BoxNICName(boxID string) string {
	return fmt.Sprintf("shellbox-%s-box-%s-nic", r.suffix, boxID)
}

func (r *ResourceNamer) BastionPublicIPName() string {
	return fmt.Sprintf("shellbox-%s-bastion-pip", r.suffix)
}

func (r *ResourceNamer) BastionOSDiskName() string {
	return fmt.Sprintf("shellbox-%s-bastion-os-disk", r.suffix)
}

func (r *ResourceNamer) BoxOSDiskName(boxID string) string {
	return fmt.Sprintf("shellbox-%s-box-%s-os-disk", r.suffix, boxID)
}

func (r *ResourceNamer) StorageAccountName() string {
	// Storage account names must be 3-24 chars, lowercase letters and numbers only
	// Remove hyphens and truncate suffix if needed
	cleanSuffix := r.cleanSuffixAlphanumeric(false) // lowercase only
	// Ensure total length is <= 24 chars
	prefix := "sb" // shortened from "shellbox"
	maxSuffixLen := 24 - len(prefix)
	if len(cleanSuffix) > maxSuffixLen {
		cleanSuffix = cleanSuffix[:maxSuffixLen]
	}
	return fmt.Sprintf("%s%s", prefix, cleanSuffix)
}

func (r *ResourceNamer) GoldenSnapshotName() string {
	return fmt.Sprintf("shellbox-%s-golden-snapshot", r.suffix)
}

func (r *ResourceNamer) BoxDataDiskName(boxID string) string {
	return fmt.Sprintf("shellbox-%s-box-%s-data-disk", r.suffix, boxID)
}

func (r *ResourceNamer) VolumePoolDiskName(volumeID string) string {
	return fmt.Sprintf("shellbox-%s-volume-%s", r.suffix, volumeID)
}

// SharedStorageAccountName returns the suffixed storage account name
// Uses "test" prefix for testing environments, "prod" prefix for production
func (r *ResourceNamer) SharedStorageAccountName() string {
	// Storage account names must be 3-24 chars, lowercase letters and numbers only
	cleanSuffix := r.cleanSuffixAlphanumeric(false) // lowercase only

	// Choose prefix based on environment type
	prefix := "shellboxprod"
	if strings.Contains(r.suffix, "test") {
		prefix = "shellboxtest"
	}

	// Ensure total length is <= 24 chars
	maxSuffixLen := 24 - len(prefix)
	if len(cleanSuffix) > maxSuffixLen {
		cleanSuffix = cleanSuffix[:maxSuffixLen]
	}
	return fmt.Sprintf("%s%s", prefix, cleanSuffix)
}

// GlobalSharedStorageAccountName returns the global storage account name for shared resources
// This is used for storage that lives in the golden resource group and is shared across all deployments
func (r *ResourceNamer) GlobalSharedStorageAccountName() string {
	// Storage account names must be 3-24 chars, lowercase letters and numbers only
	// Use a fixed name for the global shared storage account
	return "shellboxshared"
}

// EventLogTableName returns the suffixed table name for EventLog
func (r *ResourceNamer) EventLogTableName() string {
	cleanSuffix := r.cleanSuffixForTable()
	return fmt.Sprintf("%s%s", tableEventLog, cleanSuffix)
}

// ResourceRegistryTableName returns the suffixed table name for ResourceRegistry
func (r *ResourceNamer) ResourceRegistryTableName() string {
	cleanSuffix := r.cleanSuffixForTable()
	return fmt.Sprintf("%s%s", tableResourceRegistry, cleanSuffix)
}

// cleanSuffixForTable removes invalid characters from suffix for Azure Table names
// Table names can only contain alphanumeric characters
func (r *ResourceNamer) cleanSuffixForTable() string {
	return r.cleanSuffixAlphanumeric(true) // allow uppercase
}

// cleanSuffixAlphanumeric removes non-alphanumeric characters from suffix
// If allowUppercase is true, allows both upper and lowercase letters
// If allowUppercase is false, allows only lowercase letters and numbers
func (r *ResourceNamer) cleanSuffixAlphanumeric(allowUppercase bool) string {
	cleanSuffix := ""
	for _, char := range r.suffix {
		isLower := char >= 'a' && char <= 'z'
		isUpper := char >= 'A' && char <= 'Z'
		isDigit := char >= '0' && char <= '9'

		if isDigit || isLower || (allowUppercase && isUpper) {
			cleanSuffix += string(char)
		}
	}
	return cleanSuffix
}
