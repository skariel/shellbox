package infra

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
)

// Resource group configuration
const (
	Location = "westus2"
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

	// Shared storage account for testing (no suffix)
	TestingStorageAccountBaseName = "shellboxtest536567"

	// Table name constants (used as base for suffixed table names)
	tableEventLog         = "EventLog"
	tableResourceRegistry = "ResourceRegistry"
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
	ResourceRoleTemp     = "temp"
)

// Resource statuses
const (
	ResourceStatusFree      = "free"
	ResourceStatusConnected = "connected"
	ResourceStatusAttached  = "attached"
)

// Tag keys for pool resources
const (
	TagKeyRole     = "shellbox:role"
	TagKeyStatus   = "shellbox:status"
	TagKeyCreated  = "shellbox:created"
	TagKeyLastUsed = "shellbox:lastused"
	TagKeyVolumeID = "shellbox:volumeid"
)

// Tag keys for golden snapshot resources (separate namespace)
const (
	GoldenTagKeyRole    = "golden:role"
	GoldenTagKeyPurpose = "golden:purpose"
	GoldenTagKeyCreated = "golden:created"
	GoldenTagKeyStage   = "golden:stage"
)

// Azure resource types for Resource Graph queries
const (
	AzureResourceTypeVM   = "microsoft.compute/virtualmachines"
	AzureResourceTypeDisk = "microsoft.compute/disks"
)

// Query and disk constants
const (
	MaxQueryResults      = 10
	DefaultVolumeSizeGB  = 100
	GoldenSnapshotPrefix = "golden-snapshot"
)

// Persistent resource group for golden snapshots (shared across deployments)
const (
	GoldenSnapshotResourceGroup = "shellbox-golden-images"
)

// Default polling options for Azure operations
var DefaultPollOptions = runtime.PollUntilDoneOptions{
	Frequency: 2 * time.Second,
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

func GenerateConfigHash(suffix string) (string, error) {
	hashInput := FormatConfig(suffix)

	hasher := sha256.New()
	hasher.Write([]byte(hashInput))
	return hex.EncodeToString(hasher.Sum(nil))[:8], nil
}
