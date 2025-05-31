package infra

import "fmt"

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
	return fmt.Sprintf("shellbox%s", r.suffix)
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
