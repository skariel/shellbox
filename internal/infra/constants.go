package infra

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
)

// Resource group configuration
const (
	location = "westus2"
)

// Network configuration
const (
	vnetAddressSpace = "10.0.0.0/8"

	// Subnet CIDR configuration
	bastionSubnetCIDR = "10.0.0.0/24"
	boxesSubnetCIDR   = "10.1.0.0/16"
)

// Table Storage configuration
const (
	tableStorageConfigFile = ".tablestorage.json"
	tableEventLog          = "EventLog"
	tableResourceRegistry  = "ResourceRegistry"
)

// VM configuration
const (
	// VM image configuration
	VMPublisher = "Canonical"
	VMOffer     = "0001-com-ubuntu-server-jammy"
	VMSku       = "22_04-lts-gen2"
	VMVersion   = "latest"

	// SSH port configuration
	BastionSSHPort = 2222

	// Box SSH configuration
	BoxSSHPort = 2222

	// SSH key paths
	DeploymentSSHKeyPath = "$HOME/.ssh/id_ed25519"      // For deployment from dev machine
	BastionSSHKeyPath    = "/home/shellbox/.ssh/id_rsa" // For bastion host operations

	// VM default configuration
	VMSize        = "Standard_D8s_v3" // 8 vCPUs, 32GB RAM for good nested VM performance
	AdminUsername = "shellbox"
)

// Resource roles
const (
	ResourceRoleInstance = "instance"
	ResourceRoleVolume   = "volume"
)

// Resource statuses
const (
	ResourceStatusFree      = "free"
	ResourceStatusConnected = "connected"
	ResourceStatusAttached  = "attached"
)

// Tag keys
const (
	TagKeyRole     = "shellbox:role"
	TagKeyStatus   = "shellbox:status"
	TagKeyCreated  = "shellbox:created"
	TagKeyLastUsed = "shellbox:lastused"
)

// Azure resource types for Resource Graph queries
const (
	AzureResourceTypeVM   = "microsoft.compute/virtualmachines"
	AzureResourceTypeDisk = "microsoft.compute/disks"
)

// Query and disk constants
const (
	MaxQueryResults      = 10
	DefaultVolumeSizeGB  = 32
	GoldenSnapshotPrefix = "golden-snapshot"
)

// Pool configuration constants for production
const (
	DefaultMinFreeInstances  = 5
	DefaultMaxFreeInstances  = 10
	DefaultMaxTotalInstances = 100
	DefaultMinFreeVolumes    = 20
	DefaultMaxFreeVolumes    = 50
	DefaultMaxTotalVolumes   = 500
	DefaultCheckInterval     = 1 * time.Minute
	DefaultScaleDownCooldown = 10 * time.Minute
)

// Pool configuration constants for development
const (
	DevMinFreeInstances  = 1
	DevMaxFreeInstances  = 2
	DevMaxTotalInstances = 5
	DevMinFreeVolumes    = 2
	DevMaxFreeVolumes    = 5
	DevMaxTotalVolumes   = 20
	DevCheckInterval     = 30 * time.Second
	DevScaleDownCooldown = 2 * time.Minute
)

// Default polling options for Azure operations
var DefaultPollOptions = runtime.PollUntilDoneOptions{
	Frequency: 2 * time.Second,
}

// createNSGRule helper function to reduce boilerplate
func createNSGRule(name, protocol, srcAddr, dstAddr, dstPort string, access armnetwork.SecurityRuleAccess, priority int32, direction armnetwork.SecurityRuleDirection) *armnetwork.SecurityRule {
	return &armnetwork.SecurityRule{
		Name: to.Ptr(name),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocol(protocol)),
			SourceAddressPrefix:      to.Ptr(srcAddr),
			SourcePortRange:          to.Ptr("*"),
			DestinationAddressPrefix: to.Ptr(dstAddr),
			DestinationPortRange:     to.Ptr(dstPort),
			Access:                   to.Ptr(access),
			Priority:                 to.Ptr(priority),
			Direction:                to.Ptr(direction),
		},
	}
}

// NSG Rules configuration
var BastionNSGRules = []*armnetwork.SecurityRule{
	createNSGRule("AllowSSHFromInternet", "Tcp", "Internet", "*", "22", armnetwork.SecurityRuleAccessAllow, 100, armnetwork.SecurityRuleDirectionInbound),
	createNSGRule("AllowCustomSSHFromInternet", "Tcp", "Internet", "*", fmt.Sprintf("%d", BastionSSHPort), armnetwork.SecurityRuleAccessAllow, 110, armnetwork.SecurityRuleDirectionInbound),
	createNSGRule("AllowHTTPSFromInternet", "Tcp", "Internet", "*", "443", armnetwork.SecurityRuleAccessAllow, 120, armnetwork.SecurityRuleDirectionInbound),
	createNSGRule("AllowToBoxesSubnet", "*", "*", boxesSubnetCIDR, "*", armnetwork.SecurityRuleAccessAllow, 100, armnetwork.SecurityRuleDirectionOutbound),
	createNSGRule("AllowToInternet", "*", "*", "Internet", "*", armnetwork.SecurityRuleAccessAllow, 110, armnetwork.SecurityRuleDirectionOutbound),
}

func FormatConfig(suffix string) string {
	namer := NewResourceNamer(suffix)
	return fmt.Sprintf(`Network Configuration:
  VNet: %s (%s)
  Bastion Subnet: %s (%s)
  Boxes Subnet: %s (%s)
  NSG Rules:
%s
  Resource Group Suffix: %s`,
		namer.VNetName(), vnetAddressSpace,
		namer.BastionSubnetName(), bastionSubnetCIDR,
		namer.BoxesSubnetName(), boxesSubnetCIDR,
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
