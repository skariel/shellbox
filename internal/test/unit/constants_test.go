//go:build unit

package unit

import (
	"net"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// ConstantsTestSuite tests the constants and configuration validation
type ConstantsTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *ConstantsTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestNetworkConfiguration tests network CIDR and port configuration
func (suite *ConstantsTestSuite) TestNetworkConfiguration() {
	// Test that VNet address space is valid CIDR
	_, vnetNetwork, err := net.ParseCIDR("10.0.0.0/8")
	require.NoError(suite.T(), err, "VNet address space should be valid CIDR")

	// Test that bastion subnet is within VNet
	_, bastionNetwork, err := net.ParseCIDR("10.0.0.0/24")
	require.NoError(suite.T(), err, "Bastion subnet should be valid CIDR")
	assert.True(suite.T(), vnetNetwork.Contains(bastionNetwork.IP), "Bastion subnet should be within VNet")

	// Test that boxes subnet is within VNet
	_, boxesNetwork, err := net.ParseCIDR("10.1.0.0/16")
	require.NoError(suite.T(), err, "Boxes subnet should be valid CIDR")
	assert.True(suite.T(), vnetNetwork.Contains(boxesNetwork.IP), "Boxes subnet should be within VNet")

	// Test that subnets don't overlap
	assert.False(suite.T(), bastionNetwork.Contains(boxesNetwork.IP), "Bastion and boxes subnets should not overlap")
	assert.False(suite.T(), boxesNetwork.Contains(bastionNetwork.IP), "Boxes and bastion subnets should not overlap")

	// Test that subnet sizes are reasonable
	bastionOnes, bastionBits := bastionNetwork.Mask.Size()
	boxesOnes, boxesBits := boxesNetwork.Mask.Size()

	assert.Equal(suite.T(), 32, bastionBits, "Bastion subnet mask should be IPv4")
	assert.Equal(suite.T(), 32, boxesBits, "Boxes subnet mask should be IPv4")
	assert.Equal(suite.T(), 24, bastionOnes, "Bastion subnet should be /24")
	assert.Equal(suite.T(), 16, boxesOnes, "Boxes subnet should be /16")
}

// TestPortConfiguration tests SSH port configuration
func (suite *ConstantsTestSuite) TestPortConfiguration() {
	// Bastion SSH port should be non-standard for security
	assert.Equal(suite.T(), 2222, int(infra.BastionSSHPort), "Bastion SSH port should be 2222")
	assert.NotEqual(suite.T(), 22, int(infra.BastionSSHPort), "Bastion SSH port should not be standard SSH port")

	// Box SSH port should be non-standard for security
	assert.Equal(suite.T(), 2222, int(infra.BoxSSHPort), "Box SSH port should be 2222")
	assert.NotEqual(suite.T(), 22, int(infra.BoxSSHPort), "Box SSH port should not be standard SSH port")

	// Ports should be in valid range
	assert.GreaterOrEqual(suite.T(), int(infra.BastionSSHPort), 1, "Bastion SSH port should be >= 1")
	assert.LessOrEqual(suite.T(), int(infra.BastionSSHPort), 65535, "Bastion SSH port should be <= 65535")
	assert.GreaterOrEqual(suite.T(), int(infra.BoxSSHPort), 1, "Box SSH port should be >= 1")
	assert.LessOrEqual(suite.T(), int(infra.BoxSSHPort), 65535, "Box SSH port should be <= 65535")
}

// TestVMConfiguration tests VM image and size configuration
func (suite *ConstantsTestSuite) TestVMConfiguration() {
	// Test VM image configuration
	assert.Equal(suite.T(), "Canonical", infra.VMPublisher, "VM publisher should be Canonical")
	assert.Equal(suite.T(), "0001-com-ubuntu-server-jammy", infra.VMOffer, "VM offer should be Ubuntu Jammy")
	assert.Equal(suite.T(), "22_04-lts-gen2", infra.VMSku, "VM SKU should be Ubuntu 22.04 LTS Gen2")
	assert.Equal(suite.T(), "latest", infra.VMVersion, "VM version should be latest")

	// Test VM size is valid Azure VM size format
	vmSizePattern := regexp.MustCompile("^Standard_[A-Za-z0-9_]+$")
	assert.True(suite.T(), vmSizePattern.MatchString(infra.VMSize), "VM size should be valid Azure format: %s", infra.VMSize)
	assert.Equal(suite.T(), "Standard_D8s_v3", infra.VMSize, "VM size should be Standard_D8s_v3")

	// Test admin username
	assert.Equal(suite.T(), "shellbox", infra.AdminUsername, "Admin username should be shellbox")
	assert.NotEmpty(suite.T(), infra.AdminUsername, "Admin username should not be empty")
}

// TestSSHKeyPaths tests SSH key path configuration
func (suite *ConstantsTestSuite) TestSSHKeyPaths() {
	// Test deployment SSH key path
	assert.Equal(suite.T(), "$HOME/.ssh/id_ed25519", infra.DeploymentSSHKeyPath, "Deployment SSH key should use ed25519")
	assert.Contains(suite.T(), infra.DeploymentSSHKeyPath, ".ssh", "Deployment SSH key should be in .ssh directory")

	// Test bastion SSH key path
	assert.Equal(suite.T(), "/home/shellbox/.ssh/id_rsa", infra.BastionSSHKeyPath, "Bastion SSH key should use RSA")
	assert.Contains(suite.T(), infra.BastionSSHKeyPath, "/home/shellbox", "Bastion SSH key should be in shellbox home")
	assert.Contains(suite.T(), infra.BastionSSHKeyPath, ".ssh", "Bastion SSH key should be in .ssh directory")
}

// TestResourceRolesAndStatuses tests resource management constants
func (suite *ConstantsTestSuite) TestResourceRolesAndStatuses() {
	// Test resource roles
	validRoles := []string{infra.ResourceRoleInstance, infra.ResourceRoleVolume}
	assert.Equal(suite.T(), "instance", infra.ResourceRoleInstance, "Instance role should be 'instance'")
	assert.Equal(suite.T(), "volume", infra.ResourceRoleVolume, "Volume role should be 'volume'")

	for _, role := range validRoles {
		assert.NotEmpty(suite.T(), role, "Resource role should not be empty")
		assert.NotContains(suite.T(), role, " ", "Resource role should not contain spaces")
	}

	// Test resource statuses
	validStatuses := []string{infra.ResourceStatusFree, infra.ResourceStatusConnected, infra.ResourceStatusAttached}
	assert.Equal(suite.T(), "free", infra.ResourceStatusFree, "Free status should be 'free'")
	assert.Equal(suite.T(), "connected", infra.ResourceStatusConnected, "Connected status should be 'connected'")
	assert.Equal(suite.T(), "attached", infra.ResourceStatusAttached, "Attached status should be 'attached'")

	for _, status := range validStatuses {
		assert.NotEmpty(suite.T(), status, "Resource status should not be empty")
		assert.NotContains(suite.T(), status, " ", "Resource status should not contain spaces")
	}
}

// TestTagKeys tests tag key constants
func (suite *ConstantsTestSuite) TestTagKeys() {
	// Test pool tag keys
	poolTags := []string{infra.TagKeyRole, infra.TagKeyStatus, infra.TagKeyCreated, infra.TagKeyLastUsed}
	expectedPoolTags := []string{"shellbox:role", "shellbox:status", "shellbox:created", "shellbox:lastused"}

	for i, tag := range poolTags {
		assert.Equal(suite.T(), expectedPoolTags[i], tag, "Pool tag key should match expected value")
		assert.Contains(suite.T(), tag, "shellbox:", "Pool tag should have shellbox prefix")
		assert.NotContains(suite.T(), tag, " ", "Pool tag should not contain spaces")
	}

	// Test golden snapshot tag keys
	goldenTags := []string{infra.GoldenTagKeyRole, infra.GoldenTagKeyPurpose, infra.GoldenTagKeyCreated, infra.GoldenTagKeyStage}
	expectedGoldenTags := []string{"golden:role", "golden:purpose", "golden:created", "golden:stage"}

	for i, tag := range goldenTags {
		assert.Equal(suite.T(), expectedGoldenTags[i], tag, "Golden tag key should match expected value")
		assert.Contains(suite.T(), tag, "golden:", "Golden tag should have golden prefix")
		assert.NotContains(suite.T(), tag, " ", "Golden tag should not contain spaces")
	}

	// Test that pool and golden tags are in separate namespaces
	for _, poolTag := range poolTags {
		for _, goldenTag := range goldenTags {
			assert.NotEqual(suite.T(), poolTag, goldenTag, "Pool and golden tags should be in separate namespaces")
		}
	}
}

// TestAzureResourceTypes tests Azure resource type constants
func (suite *ConstantsTestSuite) TestAzureResourceTypes() {
	// Test resource types are valid Azure format
	assert.Equal(suite.T(), "microsoft.compute/virtualmachines", infra.AzureResourceTypeVM, "VM resource type should be correct")
	assert.Equal(suite.T(), "microsoft.compute/disks", infra.AzureResourceTypeDisk, "Disk resource type should be correct")

	// Test format consistency
	azureTypes := []string{infra.AzureResourceTypeVM, infra.AzureResourceTypeDisk}
	for _, azureType := range azureTypes {
		assert.Contains(suite.T(), azureType, "microsoft.", "Azure resource type should start with microsoft.")
		assert.Contains(suite.T(), azureType, "/", "Azure resource type should contain namespace separator")
		assert.Equal(suite.T(), strings.ToLower(azureType), azureType, "Azure resource type should be lowercase")
	}
}

// TestQueryAndDiskConstants tests query limits and disk configuration
func (suite *ConstantsTestSuite) TestQueryAndDiskConstants() {
	// Test query limits are reasonable
	assert.Equal(suite.T(), 10, infra.MaxQueryResults, "Max query results should be 10")
	assert.GreaterOrEqual(suite.T(), infra.MaxQueryResults, 1, "Max query results should be at least 1")
	assert.LessOrEqual(suite.T(), infra.MaxQueryResults, 1000, "Max query results should not be excessive")

	// Test volume size is reasonable
	assert.Equal(suite.T(), 100, infra.DefaultVolumeSizeGB, "Default volume size should be 100GB")
	assert.GreaterOrEqual(suite.T(), infra.DefaultVolumeSizeGB, 1, "Volume size should be at least 1GB")
	assert.LessOrEqual(suite.T(), infra.DefaultVolumeSizeGB, 1000, "Volume size should not be excessive")

	// Test golden snapshot prefix
	assert.Equal(suite.T(), "golden-snapshot", infra.GoldenSnapshotPrefix, "Golden snapshot prefix should be correct")
	assert.NotEmpty(suite.T(), infra.GoldenSnapshotPrefix, "Golden snapshot prefix should not be empty")
	assert.NotContains(suite.T(), infra.GoldenSnapshotPrefix, " ", "Golden snapshot prefix should not contain spaces")
}

// TestPoolConfiguration tests production and development pool configuration
func (suite *ConstantsTestSuite) TestPoolConfiguration() {
	// Test production pool configuration
	assert.Equal(suite.T(), 5, infra.DefaultMinFreeInstances, "Production min free instances should be 5")
	assert.Equal(suite.T(), 10, infra.DefaultMaxFreeInstances, "Production max free instances should be 10")
	assert.Equal(suite.T(), 100, infra.DefaultMaxTotalInstances, "Production max total instances should be 100")
	assert.Equal(suite.T(), 20, infra.DefaultMinFreeVolumes, "Production min free volumes should be 20")
	assert.Equal(suite.T(), 50, infra.DefaultMaxFreeVolumes, "Production max free volumes should be 50")
	assert.Equal(suite.T(), 500, infra.DefaultMaxTotalVolumes, "Production max total volumes should be 500")

	// Test development pool configuration
	assert.Equal(suite.T(), 1, infra.DevMinFreeInstances, "Dev min free instances should be 1")
	assert.Equal(suite.T(), 2, infra.DevMaxFreeInstances, "Dev max free instances should be 2")
	assert.Equal(suite.T(), 5, infra.DevMaxTotalInstances, "Dev max total instances should be 5")
	assert.Equal(suite.T(), 2, infra.DevMinFreeVolumes, "Dev min free volumes should be 2")
	assert.Equal(suite.T(), 5, infra.DevMaxFreeVolumes, "Dev max free volumes should be 5")
	assert.Equal(suite.T(), 20, infra.DevMaxTotalVolumes, "Dev max total volumes should be 20")

	// Test logical constraints
	assert.LessOrEqual(suite.T(), infra.DefaultMinFreeInstances, infra.DefaultMaxFreeInstances, "Min should be <= max free instances")
	assert.LessOrEqual(suite.T(), infra.DefaultMaxFreeInstances, infra.DefaultMaxTotalInstances, "Max free should be <= max total instances")
	assert.LessOrEqual(suite.T(), infra.DevMinFreeInstances, infra.DevMaxFreeInstances, "Dev min should be <= dev max free instances")
	assert.LessOrEqual(suite.T(), infra.DevMaxFreeInstances, infra.DevMaxTotalInstances, "Dev max free should be <= dev max total instances")

	assert.LessOrEqual(suite.T(), infra.DefaultMinFreeVolumes, infra.DefaultMaxFreeVolumes, "Min should be <= max free volumes")
	assert.LessOrEqual(suite.T(), infra.DefaultMaxFreeVolumes, infra.DefaultMaxTotalVolumes, "Max free should be <= max total volumes")
	assert.LessOrEqual(suite.T(), infra.DevMinFreeVolumes, infra.DevMaxFreeVolumes, "Dev min should be <= dev max free volumes")
	assert.LessOrEqual(suite.T(), infra.DevMaxFreeVolumes, infra.DevMaxTotalVolumes, "Dev max free should be <= dev max total volumes")

	// Test that dev configuration is smaller than production
	assert.LessOrEqual(suite.T(), infra.DevMaxTotalInstances, infra.DefaultMaxTotalInstances, "Dev max should be <= production max instances")
	assert.LessOrEqual(suite.T(), infra.DevMaxTotalVolumes, infra.DefaultMaxTotalVolumes, "Dev max should be <= production max volumes")
}

// TestPoolTimingConfiguration tests timing configuration for pools
func (suite *ConstantsTestSuite) TestPoolTimingConfiguration() {
	// Test production timing
	assert.Equal(suite.T(), 1*time.Minute, infra.DefaultCheckInterval, "Production check interval should be 1 minute")
	assert.Equal(suite.T(), 10*time.Minute, infra.DefaultScaleDownCooldown, "Production scale-down cooldown should be 10 minutes")

	// Test development timing
	assert.Equal(suite.T(), 30*time.Second, infra.DevCheckInterval, "Dev check interval should be 30 seconds")
	assert.Equal(suite.T(), 2*time.Minute, infra.DevScaleDownCooldown, "Dev scale-down cooldown should be 2 minutes")

	// Test logical constraints
	assert.Less(suite.T(), infra.DevCheckInterval, infra.DefaultCheckInterval, "Dev check interval should be faster than production")
	assert.Less(suite.T(), infra.DevScaleDownCooldown, infra.DefaultScaleDownCooldown, "Dev cooldown should be shorter than production")

	// Test minimum reasonable values
	assert.GreaterOrEqual(suite.T(), infra.DefaultCheckInterval, 30*time.Second, "Check interval should be at least 30 seconds")
	assert.GreaterOrEqual(suite.T(), infra.DefaultScaleDownCooldown, 1*time.Minute, "Cooldown should be at least 1 minute")
	assert.GreaterOrEqual(suite.T(), infra.DevCheckInterval, 10*time.Second, "Dev check interval should be at least 10 seconds")
	assert.GreaterOrEqual(suite.T(), infra.DevScaleDownCooldown, 30*time.Second, "Dev cooldown should be at least 30 seconds")
}

// TestLocationConfiguration tests Azure location configuration
func (suite *ConstantsTestSuite) TestLocationConfiguration() {
	// Test location is valid Azure region
	assert.Equal(suite.T(), "westus2", infra.Location, "Location should be westus2")
	assert.NotEmpty(suite.T(), infra.Location, "Location should not be empty")
	assert.NotContains(suite.T(), infra.Location, " ", "Location should not contain spaces")
	assert.Equal(suite.T(), strings.ToLower(infra.Location), infra.Location, "Location should be lowercase")
}

// TestTableStorageConfiguration tests table storage configuration
func (suite *ConstantsTestSuite) TestTableStorageConfiguration() {
	// This uses internal constants, so we'll test the functionality indirectly by checking FormatConfig
	suffix := "test-config"
	config := infra.FormatConfig(suffix)

	assert.NotEmpty(suite.T(), config, "FormatConfig should return non-empty string")
	assert.Contains(suite.T(), config, suffix, "Config should contain the suffix")
	assert.Contains(suite.T(), config, "Network Configuration", "Config should contain network info")
	assert.Contains(suite.T(), config, "VNet:", "Config should contain VNet info")
	assert.Contains(suite.T(), config, "Bastion Subnet:", "Config should contain bastion subnet info")
	assert.Contains(suite.T(), config, "Boxes Subnet:", "Config should contain boxes subnet info")
	assert.Contains(suite.T(), config, "NSG Rules:", "Config should contain NSG rules info")
}

// TestGoldenSnapshotResourceGroup tests golden snapshot resource group configuration
func (suite *ConstantsTestSuite) TestGoldenSnapshotResourceGroup() {
	assert.Equal(suite.T(), "shellbox-golden-images", infra.GoldenSnapshotResourceGroup, "Golden snapshot RG should be correct")
	assert.NotEmpty(suite.T(), infra.GoldenSnapshotResourceGroup, "Golden snapshot RG should not be empty")
	assert.Contains(suite.T(), infra.GoldenSnapshotResourceGroup, "shellbox", "Golden snapshot RG should contain shellbox")
	assert.Contains(suite.T(), infra.GoldenSnapshotResourceGroup, "golden", "Golden snapshot RG should contain golden")
}

// TestNSGRulesConfiguration tests NSG rules are properly configured
func (suite *ConstantsTestSuite) TestNSGRulesConfiguration() {
	// Test that BastionNSGRules is not empty
	assert.NotEmpty(suite.T(), infra.BastionNSGRules, "Bastion NSG rules should not be empty")
	assert.GreaterOrEqual(suite.T(), len(infra.BastionNSGRules), 3, "Should have at least 3 NSG rules")

	// Test each rule has required properties
	for i, rule := range infra.BastionNSGRules {
		assert.NotNil(suite.T(), rule, "Rule %d should not be nil", i)
		assert.NotNil(suite.T(), rule.Name, "Rule %d name should not be nil", i)
		assert.NotEmpty(suite.T(), *rule.Name, "Rule %d name should not be empty", i)

		assert.NotNil(suite.T(), rule.Properties, "Rule %d properties should not be nil", i)
		assert.NotNil(suite.T(), rule.Properties.Protocol, "Rule %d protocol should not be nil", i)
		assert.NotNil(suite.T(), rule.Properties.Access, "Rule %d access should not be nil", i)
		assert.NotNil(suite.T(), rule.Properties.Priority, "Rule %d priority should not be nil", i)
		assert.NotNil(suite.T(), rule.Properties.Direction, "Rule %d direction should not be nil", i)

		// Test priority is in valid range
		priority := *rule.Properties.Priority
		assert.GreaterOrEqual(suite.T(), priority, int32(100), "Rule %d priority should be >= 100", i)
		assert.LessOrEqual(suite.T(), priority, int32(4096), "Rule %d priority should be <= 4096", i)
	}

	// Test that priorities are unique within each direction (inbound/outbound can have same priority)
	inboundPriorities := make(map[int32]bool)
	outboundPriorities := make(map[int32]bool)
	for i, rule := range infra.BastionNSGRules {
		priority := *rule.Properties.Priority
		direction := *rule.Properties.Direction

		if direction == "Inbound" {
			assert.False(suite.T(), inboundPriorities[priority], "Inbound rule %d priority %d should be unique", i, priority)
			inboundPriorities[priority] = true
		} else if direction == "Outbound" {
			assert.False(suite.T(), outboundPriorities[priority], "Outbound rule %d priority %d should be unique", i, priority)
			outboundPriorities[priority] = true
		}
	}
}

// TestDefaultPollOptions tests Azure polling configuration
func (suite *ConstantsTestSuite) TestDefaultPollOptions() {
	assert.Equal(suite.T(), 2*time.Second, infra.DefaultPollOptions.Frequency, "Poll frequency should be 2 seconds")
	assert.GreaterOrEqual(suite.T(), infra.DefaultPollOptions.Frequency, 1*time.Second, "Poll frequency should be at least 1 second")
	assert.LessOrEqual(suite.T(), infra.DefaultPollOptions.Frequency, 10*time.Second, "Poll frequency should not be too slow")
}

// TestConfigurationConsistency tests that all configuration values are consistent
func (suite *ConstantsTestSuite) TestConfigurationConsistency() {
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
	assert.True(suite.T(), foundBastionSSHRule, "NSG rules should include bastion SSH port %d", infra.BastionSSHPort)

	// Test that golden snapshot resource group doesn't conflict with normal naming
	namer := infra.NewResourceNamer("test")
	normalRG := namer.ResourceGroup()
	assert.NotEqual(suite.T(), normalRG, infra.GoldenSnapshotResourceGroup, "Golden snapshot RG should not conflict with normal naming")
}

// Run the test suite
func TestConstantsTestSuite(t *testing.T) {
	suite.Run(t, new(ConstantsTestSuite))
}
