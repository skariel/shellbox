package infra

import (
	"fmt"
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

func (r *ResourceNamer) BoxDataDiskName(boxID string) string {
	return fmt.Sprintf("shellbox-%s-box-%s-data-disk", r.suffix, boxID)
}

func (r *ResourceNamer) VolumePoolDiskName(volumeID string) string {
	return fmt.Sprintf("shellbox-%s-volume-%s", r.suffix, volumeID)
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
