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

// User identification
const (
	UserIDLength = 32
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

	// VM default configuration
	VMSize              = "Standard_D8s_v3" // 8 vCPUs, 32GB RAM for good nested VM performance
	AdminUsername       = "shellbox"
	BastionComputerName = "shellbox-bastion"
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
	TagKeyRole       = "shellbox:role"
	TagKeyStatus     = "shellbox:status"
	TagKeyCreated    = "shellbox:created"
	TagKeyLastUsed   = "shellbox:lastused"
	TagKeyVolumeID   = "shellbox:volumeid"
	TagKeyInstanceID = "shellbox:instanceid"
	TagKeyUserID     = "shellbox:userid"
	TagKeyBoxName    = "shellbox:boxname"
)

// Tag keys for golden snapshot resources (separate namespace)
const (
	GoldenTagKeyRole    = "golden:role"
	GoldenTagKeyPurpose = "golden:purpose"
	GoldenTagKeyCreated = "golden:created"
	GoldenTagKeyStage   = "golden:stage"
)

// Event types for logging
const (
	EventTypeInstanceCreate  = "instance_create"
	EventTypeInstanceDelete  = "instance_delete"
	EventTypeVolumeCreate    = "volume_create"
	EventTypeVolumeDelete    = "volume_delete"
	EventTypeSessionStart    = "session_start"
	EventTypeResourceConnect = "resource_connect"
)

// Golden snapshot role values
const (
	GoldenRoleTempDataDisk = "temp-data-disk"
	GoldenRoleTempVM       = "temp-vm"
	GoldenRoleSnapshot     = "golden-snapshot"
	GoldenRoleImage        = "golden-image"
)

// System user constants
const (
	SystemUserUbuntu = "ubuntu"
)

// File system paths
const (
	QEMUMemoryPath    = "/mnt/userdata/qemu-memory/ubuntu-mem"
	QEMUDisksPath     = "/mnt/userdata/qemu-disks"
	QEMUBaseDiskPath  = "/mnt/userdata/qemu-disks/ubuntu-base.qcow2"
	QEMUCloudInitPath = "/mnt/userdata/qemu-disks/cloud-init.iso"
	QEMUMonitorSocket = "/tmp/qemu-monitor.sock"
	TempConfigPath    = "/tmp/tablestorage.json"
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
	GoldenSnapshotResourceGroup    = "shellbox-golden-images-6"
	GlobalSharedStorageAccountName = "shellboxshared6"
)

// Timeout constants
const (
	GoldenVMSetupTimeout = 15 * time.Minute // Timeout for golden VM QEMU setup and SSH connectivity
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
