//go:build unit

package unit

import (
	"testing"
	"time"

	"shellbox/internal/infra"
)

// TestNewDefaultPoolConfig tests the production pool configuration constructor
func TestNewDefaultPoolConfig(t *testing.T) {
	config := infra.NewDefaultPoolConfig()

	// Test instance configuration
	if config.MinFreeInstances != infra.DefaultMinFreeInstances {
		t.Errorf("Min free instances should match constant, expected %d, got %d", infra.DefaultMinFreeInstances, config.MinFreeInstances)
	}
	if config.MaxFreeInstances != infra.DefaultMaxFreeInstances {
		t.Errorf("Max free instances should match constant, expected %d, got %d", infra.DefaultMaxFreeInstances, config.MaxFreeInstances)
	}
	if config.MaxTotalInstances != infra.DefaultMaxTotalInstances {
		t.Errorf("Max total instances should match constant, expected %d, got %d", infra.DefaultMaxTotalInstances, config.MaxTotalInstances)
	}

	// Test volume configuration
	if config.MinFreeVolumes != infra.DefaultMinFreeVolumes {
		t.Errorf("Min free volumes should match constant, expected %d, got %d", infra.DefaultMinFreeVolumes, config.MinFreeVolumes)
	}
	if config.MaxFreeVolumes != infra.DefaultMaxFreeVolumes {
		t.Errorf("Max free volumes should match constant, expected %d, got %d", infra.DefaultMaxFreeVolumes, config.MaxFreeVolumes)
	}
	if config.MaxTotalVolumes != infra.DefaultMaxTotalVolumes {
		t.Errorf("Max total volumes should match constant, expected %d, got %d", infra.DefaultMaxTotalVolumes, config.MaxTotalVolumes)
	}

	// Test timing configuration
	if config.CheckInterval != infra.DefaultCheckInterval {
		t.Errorf("Check interval should match constant, expected %v, got %v", infra.DefaultCheckInterval, config.CheckInterval)
	}
	if config.ScaleDownCooldown != infra.DefaultScaleDownCooldown {
		t.Errorf("Scale down cooldown should match constant, expected %v, got %v", infra.DefaultScaleDownCooldown, config.ScaleDownCooldown)
	}

	// Test logical constraints for production
	if config.MinFreeInstances > config.MaxFreeInstances {
		t.Errorf("Min free instances <= max free instances, got min=%d, max=%d", config.MinFreeInstances, config.MaxFreeInstances)
	}
	if config.MaxFreeInstances > config.MaxTotalInstances {
		t.Errorf("Max free instances <= max total instances, got free=%d, total=%d", config.MaxFreeInstances, config.MaxTotalInstances)
	}
	if config.MinFreeVolumes > config.MaxFreeVolumes {
		t.Errorf("Min free volumes <= max free volumes, got min=%d, max=%d", config.MinFreeVolumes, config.MaxFreeVolumes)
	}
	if config.MaxFreeVolumes > config.MaxTotalVolumes {
		t.Errorf("Max free volumes <= max total volumes, got free=%d, total=%d", config.MaxFreeVolumes, config.MaxTotalVolumes)
	}

	// Test reasonable production values
	if config.MinFreeInstances < 1 {
		t.Errorf("Should maintain at least 1 free instance, got %d", config.MinFreeInstances)
	}
	if config.MinFreeVolumes < 1 {
		t.Errorf("Should maintain at least 1 free volume, got %d", config.MinFreeVolumes)
	}
	if config.CheckInterval < 30*time.Second {
		t.Errorf("Check interval should be reasonable (>= 30s), got %v", config.CheckInterval)
	}
	if config.ScaleDownCooldown < 1*time.Minute {
		t.Errorf("Cooldown should be reasonable (>= 1m), got %v", config.ScaleDownCooldown)
	}
}

// TestNewDevPoolConfig tests the development pool configuration constructor
func TestNewDevPoolConfig(t *testing.T) {
	config := infra.NewDevPoolConfig()

	// Test instance configuration
	if config.MinFreeInstances != infra.DevMinFreeInstances {
		t.Errorf("Dev min free instances should match constant, expected %d, got %d", infra.DevMinFreeInstances, config.MinFreeInstances)
	}
	if config.MaxFreeInstances != infra.DevMaxFreeInstances {
		t.Errorf("Dev max free instances should match constant, expected %d, got %d", infra.DevMaxFreeInstances, config.MaxFreeInstances)
	}
	if config.MaxTotalInstances != infra.DevMaxTotalInstances {
		t.Errorf("Dev max total instances should match constant, expected %d, got %d", infra.DevMaxTotalInstances, config.MaxTotalInstances)
	}

	// Test volume configuration
	if config.MinFreeVolumes != infra.DevMinFreeVolumes {
		t.Errorf("Dev min free volumes should match constant, expected %d, got %d", infra.DevMinFreeVolumes, config.MinFreeVolumes)
	}
	if config.MaxFreeVolumes != infra.DevMaxFreeVolumes {
		t.Errorf("Dev max free volumes should match constant, expected %d, got %d", infra.DevMaxFreeVolumes, config.MaxFreeVolumes)
	}
	if config.MaxTotalVolumes != infra.DevMaxTotalVolumes {
		t.Errorf("Dev max total volumes should match constant, expected %d, got %d", infra.DevMaxTotalVolumes, config.MaxTotalVolumes)
	}

	// Test timing configuration
	if config.CheckInterval != infra.DevCheckInterval {
		t.Errorf("Dev check interval should match constant, expected %v, got %v", infra.DevCheckInterval, config.CheckInterval)
	}
	if config.ScaleDownCooldown != infra.DevScaleDownCooldown {
		t.Errorf("Dev scale down cooldown should match constant, expected %v, got %v", infra.DevScaleDownCooldown, config.ScaleDownCooldown)
	}

	// Test logical constraints for development
	if config.MinFreeInstances > config.MaxFreeInstances {
		t.Errorf("Dev min free instances <= max free instances, got min=%d, max=%d", config.MinFreeInstances, config.MaxFreeInstances)
	}
	if config.MaxFreeInstances > config.MaxTotalInstances {
		t.Errorf("Dev max free instances <= max total instances, got free=%d, total=%d", config.MaxFreeInstances, config.MaxTotalInstances)
	}
	if config.MinFreeVolumes > config.MaxFreeVolumes {
		t.Errorf("Dev min free volumes <= max free volumes, got min=%d, max=%d", config.MinFreeVolumes, config.MaxFreeVolumes)
	}
	if config.MaxFreeVolumes > config.MaxTotalVolumes {
		t.Errorf("Dev max free volumes <= max total volumes, got free=%d, total=%d", config.MaxFreeVolumes, config.MaxTotalVolumes)
	}

	// Test reasonable development values
	if config.MinFreeInstances < 1 {
		t.Errorf("Dev should maintain at least 1 free instance, got %d", config.MinFreeInstances)
	}
	if config.MinFreeVolumes < 1 {
		t.Errorf("Dev should maintain at least 1 free volume, got %d", config.MinFreeVolumes)
	}
	if config.CheckInterval < 10*time.Second {
		t.Errorf("Dev check interval should be reasonable (>= 10s), got %v", config.CheckInterval)
	}
	if config.ScaleDownCooldown < 30*time.Second {
		t.Errorf("Dev cooldown should be reasonable (>= 30s), got %v", config.ScaleDownCooldown)
	}
}

// TestPoolConfigComparison tests that dev config is appropriately smaller than production
func TestPoolConfigComparison(t *testing.T) {
	defaultConfig := infra.NewDefaultPoolConfig()
	devConfig := infra.NewDevPoolConfig()

	// Dev should have smaller or equal limits than production
	if devConfig.MinFreeInstances > defaultConfig.MinFreeInstances {
		t.Errorf("Dev min instances <= production, got dev=%d, prod=%d", devConfig.MinFreeInstances, defaultConfig.MinFreeInstances)
	}
	if devConfig.MaxFreeInstances > defaultConfig.MaxFreeInstances {
		t.Errorf("Dev max free instances <= production, got dev=%d, prod=%d", devConfig.MaxFreeInstances, defaultConfig.MaxFreeInstances)
	}
	if devConfig.MaxTotalInstances > defaultConfig.MaxTotalInstances {
		t.Errorf("Dev max total instances <= production, got dev=%d, prod=%d", devConfig.MaxTotalInstances, defaultConfig.MaxTotalInstances)
	}

	if devConfig.MinFreeVolumes > defaultConfig.MinFreeVolumes {
		t.Errorf("Dev min volumes <= production, got dev=%d, prod=%d", devConfig.MinFreeVolumes, defaultConfig.MinFreeVolumes)
	}
	if devConfig.MaxFreeVolumes > defaultConfig.MaxFreeVolumes {
		t.Errorf("Dev max free volumes <= production, got dev=%d, prod=%d", devConfig.MaxFreeVolumes, defaultConfig.MaxFreeVolumes)
	}
	if devConfig.MaxTotalVolumes > defaultConfig.MaxTotalVolumes {
		t.Errorf("Dev max total volumes <= production, got dev=%d, prod=%d", devConfig.MaxTotalVolumes, defaultConfig.MaxTotalVolumes)
	}

	// Dev should have faster checks and shorter cooldowns
	if devConfig.CheckInterval > defaultConfig.CheckInterval {
		t.Errorf("Dev check interval <= production, got dev=%v, prod=%v", devConfig.CheckInterval, defaultConfig.CheckInterval)
	}
	if devConfig.ScaleDownCooldown > defaultConfig.ScaleDownCooldown {
		t.Errorf("Dev cooldown <= production, got dev=%v, prod=%v", devConfig.ScaleDownCooldown, defaultConfig.ScaleDownCooldown)
	}
}

// TestPoolConfigStructure tests that the PoolConfig struct is properly defined
func TestPoolConfigStructure(t *testing.T) {
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
	if config.MinFreeInstances != 1 {
		t.Errorf("Expected MinFreeInstances=1, got %d", config.MinFreeInstances)
	}
	if config.MaxFreeInstances != 2 {
		t.Errorf("Expected MaxFreeInstances=2, got %d", config.MaxFreeInstances)
	}
	if config.MaxTotalInstances != 10 {
		t.Errorf("Expected MaxTotalInstances=10, got %d", config.MaxTotalInstances)
	}
	if config.MinFreeVolumes != 3 {
		t.Errorf("Expected MinFreeVolumes=3, got %d", config.MinFreeVolumes)
	}
	if config.MaxFreeVolumes != 5 {
		t.Errorf("Expected MaxFreeVolumes=5, got %d", config.MaxFreeVolumes)
	}
	if config.MaxTotalVolumes != 20 {
		t.Errorf("Expected MaxTotalVolumes=20, got %d", config.MaxTotalVolumes)
	}
	if config.CheckInterval != 30*time.Second {
		t.Errorf("Expected CheckInterval=30s, got %v", config.CheckInterval)
	}
	if config.ScaleDownCooldown != 2*time.Minute {
		t.Errorf("Expected ScaleDownCooldown=2m, got %v", config.ScaleDownCooldown)
	}
}

// TestPoolConfigValidation tests pool configuration validation logic
func TestPoolConfigValidation(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			isValid := validatePoolConfigHelper(tc.config)
			if isValid != tc.isValid {
				t.Errorf("%s: expected valid=%v, got %v", tc.reason, tc.isValid, isValid)
			}
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
func TestPoolConfigConstants(t *testing.T) {
	// Test production constants
	if infra.DefaultMinFreeInstances != 5 {
		t.Errorf("Production min free instances: expected 5, got %d", infra.DefaultMinFreeInstances)
	}
	if infra.DefaultMaxFreeInstances != 10 {
		t.Errorf("Production max free instances: expected 10, got %d", infra.DefaultMaxFreeInstances)
	}
	if infra.DefaultMaxTotalInstances != 100 {
		t.Errorf("Production max total instances: expected 100, got %d", infra.DefaultMaxTotalInstances)
	}
	if infra.DefaultMinFreeVolumes != 20 {
		t.Errorf("Production min free volumes: expected 20, got %d", infra.DefaultMinFreeVolumes)
	}
	if infra.DefaultMaxFreeVolumes != 50 {
		t.Errorf("Production max free volumes: expected 50, got %d", infra.DefaultMaxFreeVolumes)
	}
	if infra.DefaultMaxTotalVolumes != 500 {
		t.Errorf("Production max total volumes: expected 500, got %d", infra.DefaultMaxTotalVolumes)
	}
	if infra.DefaultCheckInterval != 1*time.Minute {
		t.Errorf("Production check interval: expected 1m, got %v", infra.DefaultCheckInterval)
	}
	if infra.DefaultScaleDownCooldown != 10*time.Minute {
		t.Errorf("Production scale down cooldown: expected 10m, got %v", infra.DefaultScaleDownCooldown)
	}

	// Test development constants
	if infra.DevMinFreeInstances != 1 {
		t.Errorf("Dev min free instances: expected 1, got %d", infra.DevMinFreeInstances)
	}
	if infra.DevMaxFreeInstances != 2 {
		t.Errorf("Dev max free instances: expected 2, got %d", infra.DevMaxFreeInstances)
	}
	if infra.DevMaxTotalInstances != 5 {
		t.Errorf("Dev max total instances: expected 5, got %d", infra.DevMaxTotalInstances)
	}
	if infra.DevMinFreeVolumes != 2 {
		t.Errorf("Dev min free volumes: expected 2, got %d", infra.DevMinFreeVolumes)
	}
	if infra.DevMaxFreeVolumes != 5 {
		t.Errorf("Dev max free volumes: expected 5, got %d", infra.DevMaxFreeVolumes)
	}
	if infra.DevMaxTotalVolumes != 20 {
		t.Errorf("Dev max total volumes: expected 20, got %d", infra.DevMaxTotalVolumes)
	}
	if infra.DevCheckInterval != 30*time.Second {
		t.Errorf("Dev check interval: expected 30s, got %v", infra.DevCheckInterval)
	}
	if infra.DevScaleDownCooldown != 2*time.Minute {
		t.Errorf("Dev scale down cooldown: expected 2m, got %v", infra.DevScaleDownCooldown)
	}
}
