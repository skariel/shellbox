package infra

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
)

// Resource group configuration
const (
	resourceGroupPrefix = "shellbox-infra"
	location            = "westus2"
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
	VMOffer     = "0001-com-ubuntu-server-jammy"
	VMSku       = "22_04-lts-gen2"
	VMVersion   = "latest"

	// Bastion VM configuration
	bastionVMName  = "shellbox-bastion"
	bastionNICName = "bastion-nic"
	bastionIPName  = "bastion-ip"
)

// NSG Rules configuration
var BastionNSGRules = []*armnetwork.SecurityRule{
	{
		Name: to.Ptr("AllowSSHFromInternet"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			SourceAddressPrefix:      to.Ptr("Internet"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("22"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Priority:                 to.Ptr(int32(100)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
		},
	},
	{
		Name: to.Ptr("AllowHTTPSFromInternet"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
			SourceAddressPrefix:      to.Ptr("Internet"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("443"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Priority:                 to.Ptr(int32(110)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
		},
	},
	{
		Name: to.Ptr("DenyAllInbound"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("*"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
			Priority:                 to.Ptr(int32(4096)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
		},
	},
	{
		Name: to.Ptr("AllowToBoxesSubnet"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr(boxesSubnetCIDR),
			DestinationPortRange:     to.Ptr("*"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Priority:                 to.Ptr(int32(100)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
		},
	},
	{
		Name: to.Ptr("AllowToInternet"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("Internet"),
			DestinationPortRange:     to.Ptr("*"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
			Priority:                 to.Ptr(int32(110)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
		},
	},
	{
		Name: to.Ptr("DenyAllOutbound"),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolAsterisk),
			SourceAddressPrefix:      to.Ptr("*"),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr("*"),
			DestinationPortRange:     to.Ptr("*"),
			Access:                   to.Ptr(armnetwork.SecurityRuleAccessDeny),
			Priority:                 to.Ptr(int32(4096)),
			Direction:                to.Ptr(armnetwork.SecurityRuleDirectionOutbound),
		},
	},
}

func FormatConfig(suffix string) string {
	return fmt.Sprintf(`Network Configuration:
  VNet: %s (%s)
  Bastion Subnet: %s (%s)
  Boxes Subnet: %s (%s)
  NSG Rules:
%s
  Resource Group Suffix: %s`,
		vnetName, vnetAddressSpace,
		bastionSubnetName, bastionSubnetCIDR,
		boxesSubnetName, boxesSubnetCIDR,
		formatNSGRules(BastionNSGRules),
		suffix)
}

func formatNSGRules(rules []*armnetwork.SecurityRule) string {
	var result string
	for _, rule := range rules {
		result += fmt.Sprintf("    - %s: %s %s->%s (%s)\n",
			*rule.Name,
			*rule.Properties.Access,
			*rule.Properties.SourceAddressPrefix,
			*rule.Properties.DestinationAddressPrefix,
			*rule.Properties.Direction)
	}
	return result
}

func generateConfigHash(suffix string) (string, error) {
	hashInput := FormatConfig(suffix)

	hasher := sha256.New()
	hasher.Write([]byte(hashInput))
	return hex.EncodeToString(hasher.Sum(nil))[:8], nil
}
