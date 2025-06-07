//go:build unit

package unit

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"shellbox/internal/infra"
)

// TestNetworkConfiguration tests network CIDR and port configuration
func TestNetworkConfiguration(t *testing.T) {
	// Test that VNet address space is valid CIDR
	_, vnetNetwork, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatalf("VNet address space should be valid CIDR: %v", err)
	}

	// Test that bastion subnet is within VNet
	_, bastionNetwork, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("Bastion subnet should be valid CIDR: %v", err)
	}
	if !vnetNetwork.Contains(bastionNetwork.IP) {
		t.Error("Bastion subnet should be within VNet")
	}

	// Test that boxes subnet is within VNet
	_, boxesNetwork, err := net.ParseCIDR("10.1.0.0/16")
	if err != nil {
		t.Fatalf("Boxes subnet should be valid CIDR: %v", err)
	}
	if !vnetNetwork.Contains(boxesNetwork.IP) {
		t.Error("Boxes subnet should be within VNet")
	}

	// Test that subnets don't overlap
	if bastionNetwork.Contains(boxesNetwork.IP) {
		t.Error("Bastion and boxes subnets should not overlap")
	}
	if boxesNetwork.Contains(bastionNetwork.IP) {
		t.Error("Boxes and bastion subnets should not overlap")
	}

	// Test that subnet sizes are reasonable
	bastionOnes, bastionBits := bastionNetwork.Mask.Size()
	boxesOnes, boxesBits := boxesNetwork.Mask.Size()

	if bastionBits != 32 {
		t.Errorf("Bastion subnet mask should be IPv4, got %d bits", bastionBits)
	}
	if boxesBits != 32 {
		t.Errorf("Boxes subnet mask should be IPv4, got %d bits", boxesBits)
	}
	if bastionOnes != 24 {
		t.Errorf("Bastion subnet should be /24, got /%d", bastionOnes)
	}
	if boxesOnes != 16 {
		t.Errorf("Boxes subnet should be /16, got /%d", boxesOnes)
	}
}

// TestPortConfiguration tests SSH port configuration
func TestPortConfiguration(t *testing.T) {
	// Bastion SSH port should be non-standard for security
	if int(infra.BastionSSHPort) != 2222 {
		t.Errorf("Bastion SSH port should be 2222, got %d", infra.BastionSSHPort)
	}
	if int(infra.BastionSSHPort) == 22 {
		t.Error("Bastion SSH port should not be standard SSH port")
	}

	// Box SSH port should be non-standard for security
	if int(infra.BoxSSHPort) != 2222 {
		t.Errorf("Box SSH port should be 2222, got %d", infra.BoxSSHPort)
	}
	if int(infra.BoxSSHPort) == 22 {
		t.Error("Box SSH port should not be standard SSH port")
	}

	// Ports should be in valid range
	if int(infra.BastionSSHPort) < 1 {
		t.Errorf("Bastion SSH port should be >= 1, got %d", infra.BastionSSHPort)
	}
	if int(infra.BastionSSHPort) > 65535 {
		t.Errorf("Bastion SSH port should be <= 65535, got %d", infra.BastionSSHPort)
	}
	if int(infra.BoxSSHPort) < 1 {
		t.Errorf("Box SSH port should be >= 1, got %d", infra.BoxSSHPort)
	}
	if int(infra.BoxSSHPort) > 65535 {
		t.Errorf("Box SSH port should be <= 65535, got %d", infra.BoxSSHPort)
	}
}

// TestVMConfiguration tests VM image and size configuration
func TestVMConfiguration(t *testing.T) {
	// Test VM image configuration
	if infra.VMPublisher != "Canonical" {
		t.Errorf("VM publisher should be Canonical, got %q", infra.VMPublisher)
	}
	if infra.VMOffer != "0001-com-ubuntu-server-jammy" {
		t.Errorf("VM offer should be Ubuntu Jammy, got %q", infra.VMOffer)
	}
	if infra.VMSku != "22_04-lts-gen2" {
		t.Errorf("VM SKU should be Ubuntu 22.04 LTS Gen2, got %q", infra.VMSku)
	}
	if infra.VMVersion != "latest" {
		t.Errorf("VM version should be latest, got %q", infra.VMVersion)
	}

	// Test VM size is valid Azure VM size format
	vmSizePattern := regexp.MustCompile("^Standard_[A-Za-z0-9_]+$")
	if !vmSizePattern.MatchString(infra.VMSize) {
		t.Errorf("VM size should be valid Azure format, got %s", infra.VMSize)
	}
	if infra.VMSize != "Standard_D8s_v3" {
		t.Errorf("VM size should be Standard_D8s_v3, got %q", infra.VMSize)
	}

	// Test admin username
	if infra.AdminUsername != "shellbox" {
		t.Errorf("Admin username should be shellbox, got %q", infra.AdminUsername)
	}
	if infra.AdminUsername == "" {
		t.Error("Admin username should not be empty")
	}
}

// TestSSHKeyPaths tests SSH key path configuration
func TestSSHKeyPaths(t *testing.T) {
	// Test deployment SSH key path
	if infra.DeploymentSSHKeyPath != "$HOME/.ssh/id_ed25519" {
		t.Errorf("Deployment SSH key should use ed25519, got %q", infra.DeploymentSSHKeyPath)
	}
	if !strings.Contains(infra.DeploymentSSHKeyPath, ".ssh") {
		t.Error("Deployment SSH key should be in .ssh directory")
	}

	// Test bastion SSH key path
	if infra.BastionSSHKeyPath != "/home/shellbox/.ssh/id_rsa" {
		t.Errorf("Bastion SSH key should use RSA, got %q", infra.BastionSSHKeyPath)
	}
	if !strings.Contains(infra.BastionSSHKeyPath, "/home/shellbox") {
		t.Error("Bastion SSH key should be in shellbox home")
	}
	if !strings.Contains(infra.BastionSSHKeyPath, ".ssh") {
		t.Error("Bastion SSH key should be in .ssh directory")
	}
}

// TestResourceRolesAndStatuses tests resource management constants
func TestResourceRolesAndStatuses(t *testing.T) {
	// Test resource roles
	validRoles := []string{infra.ResourceRoleInstance, infra.ResourceRoleVolume}
	if infra.ResourceRoleInstance != "instance" {
		t.Errorf("Instance role should be 'instance', got %q", infra.ResourceRoleInstance)
	}
	if infra.ResourceRoleVolume != "volume" {
		t.Errorf("Volume role should be 'volume', got %q", infra.ResourceRoleVolume)
	}

	for _, role := range validRoles {
		if role == "" {
			t.Error("Resource role should not be empty")
		}
		if strings.Contains(role, " ") {
			t.Errorf("Resource role should not contain spaces: %q", role)
		}
	}

	// Test resource statuses
	validStatuses := []string{infra.ResourceStatusFree, infra.ResourceStatusConnected, infra.ResourceStatusAttached}
	if infra.ResourceStatusFree != "free" {
		t.Errorf("Free status should be 'free', got %q", infra.ResourceStatusFree)
	}
	if infra.ResourceStatusConnected != "connected" {
		t.Errorf("Connected status should be 'connected', got %q", infra.ResourceStatusConnected)
	}
	if infra.ResourceStatusAttached != "attached" {
		t.Errorf("Attached status should be 'attached', got %q", infra.ResourceStatusAttached)
	}

	for _, status := range validStatuses {
		if status == "" {
			t.Error("Resource status should not be empty")
		}
		if strings.Contains(status, " ") {
			t.Errorf("Resource status should not contain spaces: %q", status)
		}
	}
}

// TestTagKeys tests tag key constants
func TestTagKeys(t *testing.T) {
	// Test pool tag keys
	poolTags := []string{infra.TagKeyRole, infra.TagKeyStatus, infra.TagKeyCreated, infra.TagKeyLastUsed}
	expectedPoolTags := []string{"shellbox:role", "shellbox:status", "shellbox:created", "shellbox:lastused"}

	for i, tag := range poolTags {
		if tag != expectedPoolTags[i] {
			t.Errorf("Pool tag key should match expected value, expected %q, got %q", expectedPoolTags[i], tag)
		}
		if !strings.Contains(tag, "shellbox:") {
			t.Errorf("Pool tag should have shellbox prefix, got %q", tag)
		}
		if strings.Contains(tag, " ") {
			t.Errorf("Pool tag should not contain spaces, got %q", tag)
		}
	}

	// Test golden snapshot tag keys
	goldenTags := []string{infra.GoldenTagKeyRole, infra.GoldenTagKeyPurpose, infra.GoldenTagKeyCreated, infra.GoldenTagKeyStage}
	expectedGoldenTags := []string{"golden:role", "golden:purpose", "golden:created", "golden:stage"}

	for i, tag := range goldenTags {
		if tag != expectedGoldenTags[i] {
			t.Errorf("Golden tag key should match expected value, expected %q, got %q", expectedGoldenTags[i], tag)
		}
		if !strings.Contains(tag, "golden:") {
			t.Errorf("Golden tag should have golden prefix, got %q", tag)
		}
		if strings.Contains(tag, " ") {
			t.Errorf("Golden tag should not contain spaces, got %q", tag)
		}
	}

	// Test that pool and golden tags are in separate namespaces
	for _, poolTag := range poolTags {
		for _, goldenTag := range goldenTags {
			if poolTag == goldenTag {
				t.Errorf("Pool and golden tags should be in separate namespaces, found duplicate: %q", poolTag)
			}
		}
	}
}

// TestAzureResourceTypes tests Azure resource type constants
func TestAzureResourceTypes(t *testing.T) {
	// Test resource types are valid Azure format
	if infra.AzureResourceTypeVM != "microsoft.compute/virtualmachines" {
		t.Errorf("VM resource type should be correct, expected %q, got %q", "microsoft.compute/virtualmachines", infra.AzureResourceTypeVM)
	}
	if infra.AzureResourceTypeDisk != "microsoft.compute/disks" {
		t.Errorf("Disk resource type should be correct, expected %q, got %q", "microsoft.compute/disks", infra.AzureResourceTypeDisk)
	}

	// Test format consistency
	azureTypes := []string{infra.AzureResourceTypeVM, infra.AzureResourceTypeDisk}
	for _, azureType := range azureTypes {
		if !strings.Contains(azureType, "microsoft.") {
			t.Errorf("Azure resource type should start with microsoft., got %q", azureType)
		}
		if !strings.Contains(azureType, "/") {
			t.Errorf("Azure resource type should contain namespace separator, got %q", azureType)
		}
		if strings.ToLower(azureType) != azureType {
			t.Errorf("Azure resource type should be lowercase, got %q", azureType)
		}
	}
}

// TestQueryAndDiskConstants tests query limits and disk configuration
func TestQueryAndDiskConstants(t *testing.T) {
	// Test query limits are reasonable
	if infra.MaxQueryResults != 10 {
		t.Errorf("Max query results should be 10, got %d", infra.MaxQueryResults)
	}
	if infra.MaxQueryResults < 1 {
		t.Errorf("Max query results should be at least 1, got %d", infra.MaxQueryResults)
	}
	if infra.MaxQueryResults > 1000 {
		t.Errorf("Max query results should not be excessive (>1000), got %d", infra.MaxQueryResults)
	}

	// Test volume size is reasonable
	if infra.DefaultVolumeSizeGB != 100 {
		t.Errorf("Default volume size should be 100GB, got %d", infra.DefaultVolumeSizeGB)
	}
	if infra.DefaultVolumeSizeGB < 1 {
		t.Errorf("Volume size should be at least 1GB, got %d", infra.DefaultVolumeSizeGB)
	}
	if infra.DefaultVolumeSizeGB > 1000 {
		t.Errorf("Volume size should not be excessive (>1000GB), got %d", infra.DefaultVolumeSizeGB)
	}

	// Test golden snapshot prefix
	if infra.GoldenSnapshotPrefix != "golden-snapshot" {
		t.Errorf("Golden snapshot prefix should be correct, expected %q, got %q", "golden-snapshot", infra.GoldenSnapshotPrefix)
	}
	if infra.GoldenSnapshotPrefix == "" {
		t.Error("Golden snapshot prefix should not be empty")
	}
	if strings.Contains(infra.GoldenSnapshotPrefix, " ") {
		t.Errorf("Golden snapshot prefix should not contain spaces, got %q", infra.GoldenSnapshotPrefix)
	}
}

// TestPoolConfiguration tests production and development pool configuration
func TestPoolConfiguration(t *testing.T) {
	// Test production pool configuration
	if infra.DefaultMinFreeInstances != 5 {
		t.Errorf("Production min free instances should be 5, got %d", infra.DefaultMinFreeInstances)
	}
	if infra.DefaultMaxFreeInstances != 10 {
		t.Errorf("Production max free instances should be 10, got %d", infra.DefaultMaxFreeInstances)
	}
	if infra.DefaultMaxTotalInstances != 100 {
		t.Errorf("Production max total instances should be 100, got %d", infra.DefaultMaxTotalInstances)
	}
	if infra.DefaultMinFreeVolumes != 20 {
		t.Errorf("Production min free volumes should be 20, got %d", infra.DefaultMinFreeVolumes)
	}
	if infra.DefaultMaxFreeVolumes != 50 {
		t.Errorf("Production max free volumes should be 50, got %d", infra.DefaultMaxFreeVolumes)
	}
	if infra.DefaultMaxTotalVolumes != 500 {
		t.Errorf("Production max total volumes should be 500, got %d", infra.DefaultMaxTotalVolumes)
	}

	// Test development pool configuration
	if infra.DevMinFreeInstances != 1 {
		t.Errorf("Dev min free instances should be 1, got %d", infra.DevMinFreeInstances)
	}
	if infra.DevMaxFreeInstances != 2 {
		t.Errorf("Dev max free instances should be 2, got %d", infra.DevMaxFreeInstances)
	}
	if infra.DevMaxTotalInstances != 5 {
		t.Errorf("Dev max total instances should be 5, got %d", infra.DevMaxTotalInstances)
	}
	if infra.DevMinFreeVolumes != 2 {
		t.Errorf("Dev min free volumes should be 2, got %d", infra.DevMinFreeVolumes)
	}
	if infra.DevMaxFreeVolumes != 5 {
		t.Errorf("Dev max free volumes should be 5, got %d", infra.DevMaxFreeVolumes)
	}
	if infra.DevMaxTotalVolumes != 20 {
		t.Errorf("Dev max total volumes should be 20, got %d", infra.DevMaxTotalVolumes)
	}

	// Test logical constraints
	if infra.DefaultMinFreeInstances > infra.DefaultMaxFreeInstances {
		t.Errorf("Min should be <= max free instances, got min=%d, max=%d", infra.DefaultMinFreeInstances, infra.DefaultMaxFreeInstances)
	}
	if infra.DefaultMaxFreeInstances > infra.DefaultMaxTotalInstances {
		t.Errorf("Max free should be <= max total instances, got free=%d, total=%d", infra.DefaultMaxFreeInstances, infra.DefaultMaxTotalInstances)
	}
	if infra.DevMinFreeInstances > infra.DevMaxFreeInstances {
		t.Errorf("Dev min should be <= dev max free instances, got min=%d, max=%d", infra.DevMinFreeInstances, infra.DevMaxFreeInstances)
	}
	if infra.DevMaxFreeInstances > infra.DevMaxTotalInstances {
		t.Errorf("Dev max free should be <= dev max total instances, got free=%d, total=%d", infra.DevMaxFreeInstances, infra.DevMaxTotalInstances)
	}

	if infra.DefaultMinFreeVolumes > infra.DefaultMaxFreeVolumes {
		t.Errorf("Min should be <= max free volumes, got min=%d, max=%d", infra.DefaultMinFreeVolumes, infra.DefaultMaxFreeVolumes)
	}
	if infra.DefaultMaxFreeVolumes > infra.DefaultMaxTotalVolumes {
		t.Errorf("Max free should be <= max total volumes, got free=%d, total=%d", infra.DefaultMaxFreeVolumes, infra.DefaultMaxTotalVolumes)
	}
	if infra.DevMinFreeVolumes > infra.DevMaxFreeVolumes {
		t.Errorf("Dev min should be <= dev max free volumes, got min=%d, max=%d", infra.DevMinFreeVolumes, infra.DevMaxFreeVolumes)
	}
	if infra.DevMaxFreeVolumes > infra.DevMaxTotalVolumes {
		t.Errorf("Dev max free should be <= dev max total volumes, got free=%d, total=%d", infra.DevMaxFreeVolumes, infra.DevMaxTotalVolumes)
	}

	// Test that dev configuration is smaller than production
	if infra.DevMaxTotalInstances > infra.DefaultMaxTotalInstances {
		t.Errorf("Dev max should be <= production max instances, got dev=%d, prod=%d", infra.DevMaxTotalInstances, infra.DefaultMaxTotalInstances)
	}
	if infra.DevMaxTotalVolumes > infra.DefaultMaxTotalVolumes {
		t.Errorf("Dev max should be <= production max volumes, got dev=%d, prod=%d", infra.DevMaxTotalVolumes, infra.DefaultMaxTotalVolumes)
	}
}

// TestPoolTimingConfiguration tests timing configuration for pools
func TestPoolTimingConfiguration(t *testing.T) {
	// Test production timing
	if infra.DefaultCheckInterval != 1*time.Minute {
		t.Errorf("Production check interval should be 1 minute, got %v", infra.DefaultCheckInterval)
	}
	if infra.DefaultScaleDownCooldown != 10*time.Minute {
		t.Errorf("Production scale-down cooldown should be 10 minutes, got %v", infra.DefaultScaleDownCooldown)
	}

	// Test development timing
	if infra.DevCheckInterval != 30*time.Second {
		t.Errorf("Dev check interval should be 30 seconds, got %v", infra.DevCheckInterval)
	}
	if infra.DevScaleDownCooldown != 2*time.Minute {
		t.Errorf("Dev scale-down cooldown should be 2 minutes, got %v", infra.DevScaleDownCooldown)
	}

	// Test logical constraints
	if infra.DevCheckInterval >= infra.DefaultCheckInterval {
		t.Errorf("Dev check interval should be faster than production, got dev=%v, prod=%v", infra.DevCheckInterval, infra.DefaultCheckInterval)
	}
	if infra.DevScaleDownCooldown >= infra.DefaultScaleDownCooldown {
		t.Errorf("Dev cooldown should be shorter than production, got dev=%v, prod=%v", infra.DevScaleDownCooldown, infra.DefaultScaleDownCooldown)
	}

	// Test minimum reasonable values
	if infra.DefaultCheckInterval < 30*time.Second {
		t.Errorf("Check interval should be at least 30 seconds, got %v", infra.DefaultCheckInterval)
	}
	if infra.DefaultScaleDownCooldown < 1*time.Minute {
		t.Errorf("Cooldown should be at least 1 minute, got %v", infra.DefaultScaleDownCooldown)
	}
	if infra.DevCheckInterval < 10*time.Second {
		t.Errorf("Dev check interval should be at least 10 seconds, got %v", infra.DevCheckInterval)
	}
	if infra.DevScaleDownCooldown < 30*time.Second {
		t.Errorf("Dev cooldown should be at least 30 seconds, got %v", infra.DevScaleDownCooldown)
	}
}

// TestLocationConfiguration tests Azure location configuration
func TestLocationConfiguration(t *testing.T) {
	// Test location is valid Azure region
	if infra.Location != "westus2" {
		t.Errorf("Location should be westus2, got %q", infra.Location)
	}
	if infra.Location == "" {
		t.Error("Location should not be empty")
	}
	if strings.Contains(infra.Location, " ") {
		t.Errorf("Location should not contain spaces, got %q", infra.Location)
	}
	if strings.ToLower(infra.Location) != infra.Location {
		t.Errorf("Location should be lowercase, got %q", infra.Location)
	}
}

// TestTableStorageConfiguration tests table storage configuration
func TestTableStorageConfiguration(t *testing.T) {
	// This uses internal constants, so we'll test the functionality indirectly by checking FormatConfig
	suffix := "test-config"
	config := infra.FormatConfig(suffix)

	if config == "" {
		t.Error("FormatConfig should return non-empty string")
	}
	if !strings.Contains(config, suffix) {
		t.Errorf("Config should contain the suffix %q", suffix)
	}
	if !strings.Contains(config, "Network Configuration") {
		t.Error("Config should contain network info")
	}
	if !strings.Contains(config, "VNet:") {
		t.Error("Config should contain VNet info")
	}
	if !strings.Contains(config, "Bastion Subnet:") {
		t.Error("Config should contain bastion subnet info")
	}
	if !strings.Contains(config, "Boxes Subnet:") {
		t.Error("Config should contain boxes subnet info")
	}
	if !strings.Contains(config, "NSG Rules:") {
		t.Error("Config should contain NSG rules info")
	}
}

// TestGoldenSnapshotResourceGroup tests golden snapshot resource group configuration
func TestGoldenSnapshotResourceGroup(t *testing.T) {
	if infra.GoldenSnapshotResourceGroup != "shellbox-golden-images" {
		t.Errorf("Golden snapshot RG should be correct, expected %q, got %q", "shellbox-golden-images", infra.GoldenSnapshotResourceGroup)
	}
	if infra.GoldenSnapshotResourceGroup == "" {
		t.Error("Golden snapshot RG should not be empty")
	}
	if !strings.Contains(infra.GoldenSnapshotResourceGroup, "shellbox") {
		t.Errorf("Golden snapshot RG should contain shellbox, got %q", infra.GoldenSnapshotResourceGroup)
	}
	if !strings.Contains(infra.GoldenSnapshotResourceGroup, "golden") {
		t.Errorf("Golden snapshot RG should contain golden, got %q", infra.GoldenSnapshotResourceGroup)
	}
}

// TestNSGRulesConfiguration tests NSG rules are properly configured
func TestNSGRulesConfiguration(t *testing.T) {
	// Test that BastionNSGRules is not empty
	if len(infra.BastionNSGRules) == 0 {
		t.Error("Bastion NSG rules should not be empty")
	}
	if len(infra.BastionNSGRules) < 3 {
		t.Errorf("Should have at least 3 NSG rules, got %d", len(infra.BastionNSGRules))
	}

	// Test each rule has required properties
	for i, rule := range infra.BastionNSGRules {
		if rule == nil {
			t.Errorf("Rule %d should not be nil", i)
			continue
		}
		if rule.Name == nil {
			t.Errorf("Rule %d name should not be nil", i)
			continue
		}
		if *rule.Name == "" {
			t.Errorf("Rule %d name should not be empty", i)
		}

		if rule.Properties == nil {
			t.Errorf("Rule %d properties should not be nil", i)
			continue
		}
		if rule.Properties.Protocol == nil {
			t.Errorf("Rule %d protocol should not be nil", i)
		}
		if rule.Properties.Access == nil {
			t.Errorf("Rule %d access should not be nil", i)
		}
		if rule.Properties.Priority == nil {
			t.Errorf("Rule %d priority should not be nil", i)
			continue
		}
		if rule.Properties.Direction == nil {
			t.Errorf("Rule %d direction should not be nil", i)
			continue
		}

		// Test priority is in valid range
		priority := *rule.Properties.Priority
		if priority < 100 {
			t.Errorf("Rule %d priority should be >= 100, got %d", i, priority)
		}
		if priority > 4096 {
			t.Errorf("Rule %d priority should be <= 4096, got %d", i, priority)
		}
	}

	// Test that priorities are unique within each direction (inbound/outbound can have same priority)
	inboundPriorities := make(map[int32]bool)
	outboundPriorities := make(map[int32]bool)
	for i, rule := range infra.BastionNSGRules {
		if rule == nil || rule.Properties == nil || rule.Properties.Priority == nil || rule.Properties.Direction == nil {
			continue
		}
		priority := *rule.Properties.Priority
		direction := *rule.Properties.Direction

		if direction == "Inbound" {
			if inboundPriorities[priority] {
				t.Errorf("Inbound rule %d priority %d should be unique", i, priority)
			}
			inboundPriorities[priority] = true
		} else if direction == "Outbound" {
			if outboundPriorities[priority] {
				t.Errorf("Outbound rule %d priority %d should be unique", i, priority)
			}
			outboundPriorities[priority] = true
		}
	}
}

// TestDefaultPollOptions tests Azure polling configuration
func TestDefaultPollOptions(t *testing.T) {
	if infra.DefaultPollOptions.Frequency != 2*time.Second {
		t.Errorf("Poll frequency should be 2 seconds, got %v", infra.DefaultPollOptions.Frequency)
	}
	if infra.DefaultPollOptions.Frequency < 1*time.Second {
		t.Errorf("Poll frequency should be at least 1 second, got %v", infra.DefaultPollOptions.Frequency)
	}
	if infra.DefaultPollOptions.Frequency > 10*time.Second {
		t.Errorf("Poll frequency should not be too slow (>10s), got %v", infra.DefaultPollOptions.Frequency)
	}
}

// TestConfigurationConsistency tests that all configuration values are consistent
func TestConfigurationConsistency(t *testing.T) {
	// Test that SSH ports referenced in NSG rules match the constants
	bastionSSHPortStr := strconv.Itoa(infra.BastionSSHPort)

	foundBastionSSHRule := false
	for _, rule := range infra.BastionNSGRules {
		if rule.Properties != nil && rule.Properties.DestinationPortRange != nil {
			if *rule.Properties.DestinationPortRange == bastionSSHPortStr {
				foundBastionSSHRule = true
				break
			}
		}
	}
	if !foundBastionSSHRule {
		t.Errorf("NSG rules should include bastion SSH port %d", infra.BastionSSHPort)
	}

	// Test that golden snapshot resource group doesn't conflict with normal naming
	namer := infra.NewResourceNamer("test")
	normalRG := namer.ResourceGroup()
	if normalRG == infra.GoldenSnapshotResourceGroup {
		t.Errorf("Golden snapshot RG should not conflict with normal naming, both are %q", normalRG)
	}
}
