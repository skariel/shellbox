package infra

// Resource group configuration
const (
	resourceGroupPrefix = "shellbox-infra"
	location           = "westus2"
)

// Network configuration
const (
	vnetName         = "shellbox-network"
	vnetAddressSpace = "10.0.0.0/8"

	// Bastion subnet configuration
	bastionSubnetName = "bastion-subnet"
	bastionSubnetCIDR = "10.0.0.0/24"
	bastionNSGName    = "bastion-nsg"

	// Boxes subnet configuration
	boxesSubnetName = "boxes-subnet"
	boxesSubnetCIDR = "10.1.0.0/16"
)

// VM configuration
const (
	// VM image configuration
	VMPublisher = "Canonical"
	VMOffer    = "0001-com-ubuntu-server-jammy"
	VMSku      = "22_04-lts-gen2"
	VMVersion  = "latest"

	// Bastion VM configuration
	bastionVMName  = "shellbox-bastion"
	bastionNICName = "bastion-nic"
	bastionIPName  = "bastion-ip"
)

// Role definitions
const (
	contributorRoleID = "/providers/Microsoft.Authorization/roleDefinitions/b24988ac-6180-42a0-ab88-20f7382dd24c"
	readerRoleID      = "/providers/Microsoft.Authorization/roleDefinitions/acdd72a7-3385-48ef-bd42-f606fba81ae7"
)
