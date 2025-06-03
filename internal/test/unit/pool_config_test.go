//go:build unit

package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// PoolConfigTestSuite tests pool configuration constructors and validation
type PoolConfigTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *PoolConfigTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestNewDefaultPoolConfig tests the production pool configuration constructor
func (suite *PoolConfigTestSuite) TestNewDefaultPoolConfig() {
	config := infra.NewDefaultPoolConfig()

	// Test instance configuration
	assert.Equal(suite.T(), infra.DefaultMinFreeInstances, config.MinFreeInstances, "Min free instances should match constant")
	assert.Equal(suite.T(), infra.DefaultMaxFreeInstances, config.MaxFreeInstances, "Max free instances should match constant")
	assert.Equal(suite.T(), infra.DefaultMaxTotalInstances, config.MaxTotalInstances, "Max total instances should match constant")

	// Test volume configuration
	assert.Equal(suite.T(), infra.DefaultMinFreeVolumes, config.MinFreeVolumes, "Min free volumes should match constant")
	assert.Equal(suite.T(), infra.DefaultMaxFreeVolumes, config.MaxFreeVolumes, "Max free volumes should match constant")
	assert.Equal(suite.T(), infra.DefaultMaxTotalVolumes, config.MaxTotalVolumes, "Max total volumes should match constant")

	// Test timing configuration
	assert.Equal(suite.T(), infra.DefaultCheckInterval, config.CheckInterval, "Check interval should match constant")
	assert.Equal(suite.T(), infra.DefaultScaleDownCooldown, config.ScaleDownCooldown, "Scale down cooldown should match constant")

	// Test logical constraints for production
	assert.LessOrEqual(suite.T(), config.MinFreeInstances, config.MaxFreeInstances, "Min free instances <= max free instances")
	assert.LessOrEqual(suite.T(), config.MaxFreeInstances, config.MaxTotalInstances, "Max free instances <= max total instances")
	assert.LessOrEqual(suite.T(), config.MinFreeVolumes, config.MaxFreeVolumes, "Min free volumes <= max free volumes")
	assert.LessOrEqual(suite.T(), config.MaxFreeVolumes, config.MaxTotalVolumes, "Max free volumes <= max total volumes")

	// Test reasonable production values
	assert.GreaterOrEqual(suite.T(), config.MinFreeInstances, 1, "Should maintain at least 1 free instance")
	assert.GreaterOrEqual(suite.T(), config.MinFreeVolumes, 1, "Should maintain at least 1 free volume")
	assert.GreaterOrEqual(suite.T(), config.CheckInterval, 30*time.Second, "Check interval should be reasonable")
	assert.GreaterOrEqual(suite.T(), config.ScaleDownCooldown, 1*time.Minute, "Cooldown should be reasonable")
}

// TestNewDevPoolConfig tests the development pool configuration constructor
func (suite *PoolConfigTestSuite) TestNewDevPoolConfig() {
	config := infra.NewDevPoolConfig()

	// Test instance configuration
	assert.Equal(suite.T(), infra.DevMinFreeInstances, config.MinFreeInstances, "Dev min free instances should match constant")
	assert.Equal(suite.T(), infra.DevMaxFreeInstances, config.MaxFreeInstances, "Dev max free instances should match constant")
	assert.Equal(suite.T(), infra.DevMaxTotalInstances, config.MaxTotalInstances, "Dev max total instances should match constant")

	// Test volume configuration
	assert.Equal(suite.T(), infra.DevMinFreeVolumes, config.MinFreeVolumes, "Dev min free volumes should match constant")
	assert.Equal(suite.T(), infra.DevMaxFreeVolumes, config.MaxFreeVolumes, "Dev max free volumes should match constant")
	assert.Equal(suite.T(), infra.DevMaxTotalVolumes, config.MaxTotalVolumes, "Dev max total volumes should match constant")

	// Test timing configuration
	assert.Equal(suite.T(), infra.DevCheckInterval, config.CheckInterval, "Dev check interval should match constant")
	assert.Equal(suite.T(), infra.DevScaleDownCooldown, config.ScaleDownCooldown, "Dev scale down cooldown should match constant")

	// Test logical constraints for development
	assert.LessOrEqual(suite.T(), config.MinFreeInstances, config.MaxFreeInstances, "Dev min free instances <= max free instances")
	assert.LessOrEqual(suite.T(), config.MaxFreeInstances, config.MaxTotalInstances, "Dev max free instances <= max total instances")
	assert.LessOrEqual(suite.T(), config.MinFreeVolumes, config.MaxFreeVolumes, "Dev min free volumes <= max free volumes")
	assert.LessOrEqual(suite.T(), config.MaxFreeVolumes, config.MaxTotalVolumes, "Dev max free volumes <= max total volumes")

	// Test reasonable development values
	assert.GreaterOrEqual(suite.T(), config.MinFreeInstances, 1, "Dev should maintain at least 1 free instance")
	assert.GreaterOrEqual(suite.T(), config.MinFreeVolumes, 1, "Dev should maintain at least 1 free volume")
	assert.GreaterOrEqual(suite.T(), config.CheckInterval, 10*time.Second, "Dev check interval should be reasonable")
	assert.GreaterOrEqual(suite.T(), config.ScaleDownCooldown, 30*time.Second, "Dev cooldown should be reasonable")
}

// TestPoolConfigComparison tests that dev config is appropriately smaller than production
func (suite *PoolConfigTestSuite) TestPoolConfigComparison() {
	defaultConfig := infra.NewDefaultPoolConfig()
	devConfig := infra.NewDevPoolConfig()

	// Dev should have smaller or equal limits than production
	assert.LessOrEqual(suite.T(), devConfig.MinFreeInstances, defaultConfig.MinFreeInstances, "Dev min instances <= production")
	assert.LessOrEqual(suite.T(), devConfig.MaxFreeInstances, defaultConfig.MaxFreeInstances, "Dev max free instances <= production")
	assert.LessOrEqual(suite.T(), devConfig.MaxTotalInstances, defaultConfig.MaxTotalInstances, "Dev max total instances <= production")

	assert.LessOrEqual(suite.T(), devConfig.MinFreeVolumes, defaultConfig.MinFreeVolumes, "Dev min volumes <= production")
	assert.LessOrEqual(suite.T(), devConfig.MaxFreeVolumes, defaultConfig.MaxFreeVolumes, "Dev max free volumes <= production")
	assert.LessOrEqual(suite.T(), devConfig.MaxTotalVolumes, defaultConfig.MaxTotalVolumes, "Dev max total volumes <= production")

	// Dev should have faster checks and shorter cooldowns
	assert.LessOrEqual(suite.T(), devConfig.CheckInterval, defaultConfig.CheckInterval, "Dev check interval <= production")
	assert.LessOrEqual(suite.T(), devConfig.ScaleDownCooldown, defaultConfig.ScaleDownCooldown, "Dev cooldown <= production")
}

// TestPoolConfigStructure tests that the PoolConfig struct is properly defined
func (suite *PoolConfigTestSuite) TestPoolConfigStructure() {
	config := infra.PoolConfig{}

	// Test that all fields can be set (testing struct completeness)
	config.MinFreeInstances = 1
	config.MaxFreeInstances = 2
	config.MaxTotalInstances = 10
	config.MinFreeVolumes = 3
	config.MaxFreeVolumes = 5
	config.MaxTotalVolumes = 20
	config.CheckInterval = 30 * time.Second
	config.ScaleDownCooldown = 2 * time.Minute

	// Verify all fields were set properly
	assert.Equal(suite.T(), 1, config.MinFreeInstances)
	assert.Equal(suite.T(), 2, config.MaxFreeInstances)
	assert.Equal(suite.T(), 10, config.MaxTotalInstances)
	assert.Equal(suite.T(), 3, config.MinFreeVolumes)
	assert.Equal(suite.T(), 5, config.MaxFreeVolumes)
	assert.Equal(suite.T(), 20, config.MaxTotalVolumes)
	assert.Equal(suite.T(), 30*time.Second, config.CheckInterval)
	assert.Equal(suite.T(), 2*time.Minute, config.ScaleDownCooldown)
}

// TestPoolConfigValidation tests pool configuration validation logic
func (suite *PoolConfigTestSuite) TestPoolConfigValidation() {
	testCases := []struct {
		name    string
		config  infra.PoolConfig
		isValid bool
		reason  string
	}{
		{
			name:    "Valid production config",
			config:  infra.NewDefaultPoolConfig(),
			isValid: true,
			reason:  "Default production config should be valid",
		},
		{
			name:    "Valid dev config",
			config:  infra.NewDevPoolConfig(),
			isValid: true,
			reason:  "Default dev config should be valid",
		},
		{
			name: "Invalid: min > max instances",
			config: infra.PoolConfig{
				MinFreeInstances:  10,
				MaxFreeInstances:  5,
				MaxTotalInstances: 20,
				MinFreeVolumes:    1,
				MaxFreeVolumes:    5,
				MaxTotalVolumes:   20,
				CheckInterval:     1 * time.Minute,
				ScaleDownCooldown: 5 * time.Minute,
			},
			isValid: false,
			reason:  "Min free instances should not exceed max free instances",
		},
		{
			name: "Invalid: max free > max total instances",
			config: infra.PoolConfig{
				MinFreeInstances:  1,
				MaxFreeInstances:  15,
				MaxTotalInstances: 10,
				MinFreeVolumes:    1,
				MaxFreeVolumes:    5,
				MaxTotalVolumes:   20,
				CheckInterval:     1 * time.Minute,
				ScaleDownCooldown: 5 * time.Minute,
			},
			isValid: false,
			reason:  "Max free instances should not exceed max total instances",
		},
		{
			name: "Invalid: zero instances",
			config: infra.PoolConfig{
				MinFreeInstances:  0,
				MaxFreeInstances:  0,
				MaxTotalInstances: 0,
				MinFreeVolumes:    1,
				MaxFreeVolumes:    5,
				MaxTotalVolumes:   20,
				CheckInterval:     1 * time.Minute,
				ScaleDownCooldown: 5 * time.Minute,
			},
			isValid: false,
			reason:  "Should maintain at least one instance",
		},
		{
			name: "Invalid: too short check interval",
			config: infra.PoolConfig{
				MinFreeInstances:  1,
				MaxFreeInstances:  5,
				MaxTotalInstances: 10,
				MinFreeVolumes:    1,
				MaxFreeVolumes:    5,
				MaxTotalVolumes:   20,
				CheckInterval:     1 * time.Second,
				ScaleDownCooldown: 5 * time.Minute,
			},
			isValid: false,
			reason:  "Check interval should be reasonable (>= 10s)",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			isValid := validatePoolConfigHelper(tc.config)
			assert.Equal(t, tc.isValid, isValid, tc.reason)
		})
	}
}

// Helper function to validate pool configuration
func validatePoolConfigHelper(config infra.PoolConfig) bool {
	// Check instance constraints
	if config.MinFreeInstances > config.MaxFreeInstances {
		return false
	}
	if config.MaxFreeInstances > config.MaxTotalInstances {
		return false
	}
	if config.MaxTotalInstances == 0 {
		return false
	}

	// Check volume constraints
	if config.MinFreeVolumes > config.MaxFreeVolumes {
		return false
	}
	if config.MaxFreeVolumes > config.MaxTotalVolumes {
		return false
	}
	if config.MaxTotalVolumes == 0 {
		return false
	}

	// Check timing constraints
	if config.CheckInterval < 10*time.Second {
		return false
	}
	if config.ScaleDownCooldown < 30*time.Second {
		return false
	}

	return true
}

// TestPoolConfigConstants tests that the constants used are reasonable
func (suite *PoolConfigTestSuite) TestPoolConfigConstants() {
	// Test production constants
	assert.Equal(suite.T(), 5, infra.DefaultMinFreeInstances, "Production min free instances")
	assert.Equal(suite.T(), 10, infra.DefaultMaxFreeInstances, "Production max free instances")
	assert.Equal(suite.T(), 100, infra.DefaultMaxTotalInstances, "Production max total instances")
	assert.Equal(suite.T(), 20, infra.DefaultMinFreeVolumes, "Production min free volumes")
	assert.Equal(suite.T(), 50, infra.DefaultMaxFreeVolumes, "Production max free volumes")
	assert.Equal(suite.T(), 500, infra.DefaultMaxTotalVolumes, "Production max total volumes")
	assert.Equal(suite.T(), 1*time.Minute, infra.DefaultCheckInterval, "Production check interval")
	assert.Equal(suite.T(), 10*time.Minute, infra.DefaultScaleDownCooldown, "Production scale down cooldown")

	// Test development constants
	assert.Equal(suite.T(), 1, infra.DevMinFreeInstances, "Dev min free instances")
	assert.Equal(suite.T(), 2, infra.DevMaxFreeInstances, "Dev max free instances")
	assert.Equal(suite.T(), 5, infra.DevMaxTotalInstances, "Dev max total instances")
	assert.Equal(suite.T(), 2, infra.DevMinFreeVolumes, "Dev min free volumes")
	assert.Equal(suite.T(), 5, infra.DevMaxFreeVolumes, "Dev max free volumes")
	assert.Equal(suite.T(), 20, infra.DevMaxTotalVolumes, "Dev max total volumes")
	assert.Equal(suite.T(), 30*time.Second, infra.DevCheckInterval, "Dev check interval")
	assert.Equal(suite.T(), 2*time.Minute, infra.DevScaleDownCooldown, "Dev scale down cooldown")
}

// Run the test suite
func TestPoolConfigTestSuite(t *testing.T) {
	suite.Run(t, new(PoolConfigTestSuite))
}
