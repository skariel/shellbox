//go:build unit

package unit

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// ConfigHashTestSuite tests configuration formatting and hash generation
type ConfigHashTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *ConfigHashTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestFormatConfig tests the configuration formatting function
func (suite *ConfigHashTestSuite) TestFormatConfig() {
	testCases := []struct {
		name             string
		suffix           string
		expectedContains []string
		expectedNotEmpty bool
	}{
		{
			name:   "Standard suffix",
			suffix: "test123",
			expectedContains: []string{
				"Network Configuration",
				"test123",
				"VNet:",
				"Bastion Subnet:",
				"Boxes Subnet:",
				"NSG Rules:",
				"westus2",
			},
			expectedNotEmpty: true,
		},
		{
			name:   "Suffix with hyphens",
			suffix: "my-test-env",
			expectedContains: []string{
				"Network Configuration",
				"my-test-env",
				"VNet:",
				"Bastion Subnet:",
				"Boxes Subnet:",
			},
			expectedNotEmpty: true,
		},
		{
			name:   "Empty suffix",
			suffix: "",
			expectedContains: []string{
				"Network Configuration",
				"VNet:",
				"Bastion Subnet:",
				"Boxes Subnet:",
			},
			expectedNotEmpty: true,
		},
		{
			name:   "Long suffix",
			suffix: "very-long-suffix-for-testing-config-generation",
			expectedContains: []string{
				"Network Configuration",
				"very-long-suffix-for-testing-config-generation",
				"VNet:",
			},
			expectedNotEmpty: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			config := infra.FormatConfig(tc.suffix)

			if tc.expectedNotEmpty {
				assert.NotEmpty(t, config, "Config should not be empty")
			}

			for _, expected := range tc.expectedContains {
				assert.Contains(t, config, expected, "Config should contain: %s", expected)
			}

			// Test that config contains network information
			assert.Contains(t, config, "10.0.0.0/8", "Should contain VNet CIDR")
			assert.Contains(t, config, "10.0.0.0/24", "Should contain bastion subnet CIDR")
			assert.Contains(t, config, "10.1.0.0/16", "Should contain boxes subnet CIDR")

			// Test that config is well-structured
			lines := strings.Split(config, "\n")
			assert.Greater(t, len(lines), 5, "Config should have multiple lines")
		})
	}
}

// TestFormatConfigConsistency tests that the same suffix produces the same config
func (suite *ConfigHashTestSuite) TestFormatConfigConsistency() {
	suffix := "consistency-test"

	config1 := infra.FormatConfig(suffix)
	config2 := infra.FormatConfig(suffix)

	assert.Equal(suite.T(), config1, config2, "Same suffix should produce identical config")
	assert.NotEmpty(suite.T(), config1, "Config should not be empty")
}

// TestFormatConfigUniqueness tests that different suffixes produce different configs
func (suite *ConfigHashTestSuite) TestFormatConfigUniqueness() {
	config1 := infra.FormatConfig("suffix1")
	config2 := infra.FormatConfig("suffix2")

	assert.NotEqual(suite.T(), config1, config2, "Different suffixes should produce different configs")
	assert.NotEmpty(suite.T(), config1, "Config1 should not be empty")
	assert.NotEmpty(suite.T(), config2, "Config2 should not be empty")

	// Both should contain common elements
	assert.Contains(suite.T(), config1, "Network Configuration")
	assert.Contains(suite.T(), config2, "Network Configuration")

	// But should contain their respective suffixes
	assert.Contains(suite.T(), config1, "suffix1")
	assert.Contains(suite.T(), config2, "suffix2")
	assert.NotContains(suite.T(), config1, "suffix2")
	assert.NotContains(suite.T(), config2, "suffix1")
}

// TestGenerateConfigHash tests the configuration hash generation
func (suite *ConfigHashTestSuite) TestGenerateConfigHash() {
	testCases := []struct {
		name           string
		suffix         string
		expectedLength int
		shouldError    bool
	}{
		{
			name:           "Standard suffix",
			suffix:         "test123",
			expectedLength: 8,
			shouldError:    false,
		},
		{
			name:           "Empty suffix",
			suffix:         "",
			expectedLength: 8,
			shouldError:    false,
		},
		{
			name:           "Complex suffix",
			suffix:         "complex-test-suffix-123",
			expectedLength: 8,
			shouldError:    false,
		},
		{
			name:           "Unicode suffix",
			suffix:         "test-üñíçødé",
			expectedLength: 8,
			shouldError:    false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			hash, err := infra.GenerateConfigHash(tc.suffix)

			if tc.shouldError {
				assert.Error(t, err, "Should return error")
			} else {
				require.NoError(t, err, "Should not return error")
				assert.Len(t, hash, tc.expectedLength, "Hash should be 8 characters")

				// Test that hash contains only hex characters
				for _, char := range hash {
					assert.True(t, isHexChar(char), "Hash should contain only hex characters: %c", char)
				}

				// Test that hash is not empty
				assert.NotEmpty(t, hash, "Hash should not be empty")
			}
		})
	}
}

// TestGenerateConfigHashConsistency tests that the same suffix produces the same hash
func (suite *ConfigHashTestSuite) TestGenerateConfigHashConsistency() {
	suffix := "hash-consistency-test"

	hash1, err1 := infra.GenerateConfigHash(suffix)
	hash2, err2 := infra.GenerateConfigHash(suffix)

	require.NoError(suite.T(), err1, "First hash generation should not error")
	require.NoError(suite.T(), err2, "Second hash generation should not error")

	assert.Equal(suite.T(), hash1, hash2, "Same suffix should produce identical hash")
	assert.Len(suite.T(), hash1, 8, "Hash should be 8 characters")
	assert.Len(suite.T(), hash2, 8, "Hash should be 8 characters")
}

// TestGenerateConfigHashUniqueness tests that different suffixes produce different hashes
func (suite *ConfigHashTestSuite) TestGenerateConfigHashUniqueness() {
	suffixes := []string{"test1", "test2", "different", "another-test", ""}
	hashes := make(map[string]string)

	for _, suffix := range suffixes {
		hash, err := infra.GenerateConfigHash(suffix)
		require.NoError(suite.T(), err, "Hash generation should not error for suffix: %s", suffix)

		// Check that this hash is unique
		for otherSuffix, otherHash := range hashes {
			if suffix != otherSuffix {
				assert.NotEqual(suite.T(), hash, otherHash,
					"Different suffixes should produce different hashes: %s vs %s", suffix, otherSuffix)
			}
		}

		hashes[suffix] = hash
	}
}

// TestHashAlgorithmCorrectness tests that the hash is generated correctly
func (suite *ConfigHashTestSuite) TestHashAlgorithmCorrectness() {
	suffix := "algorithm-test"

	// Generate hash using the function
	actualHash, err := infra.GenerateConfigHash(suffix)
	require.NoError(suite.T(), err, "Hash generation should not error")

	// Generate hash manually to verify algorithm
	config := infra.FormatConfig(suffix)
	hasher := sha256.New()
	hasher.Write([]byte(config))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))[:8]

	assert.Equal(suite.T(), expectedHash, actualHash, "Hash should match manual calculation")
	assert.Len(suite.T(), actualHash, 8, "Hash should be truncated to 8 characters")
}

// TestHashCollisionResistance tests basic collision resistance properties
func (suite *ConfigHashTestSuite) TestHashCollisionResistance() {
	// Test with similar suffixes that might cause collisions in a weak hash
	similarSuffixes := []string{
		"test", "test1", "test2", "test12", "test21",
		"a", "aa", "aaa", "ab", "ba",
		"production", "productio", "productin",
	}

	hashes := make(map[string][]string)

	for _, suffix := range similarSuffixes {
		hash, err := infra.GenerateConfigHash(suffix)
		require.NoError(suite.T(), err, "Hash generation should not error")

		if existingSuffixes, exists := hashes[hash]; exists {
			// Collision detected - this is very unlikely with SHA256 but let's document it
			suite.T().Logf("Hash collision detected: %s and %v both hash to %s",
				suffix, existingSuffixes, hash)
		} else {
			hashes[hash] = []string{suffix}
		}
	}

	// For this small test set, we expect no collisions with SHA256
	assert.Len(suite.T(), hashes, len(similarSuffixes),
		"Should have unique hashes for all test suffixes (collision unlikely with SHA256)")
}

// Helper function to check if character is a valid hex digit
func isHexChar(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
}

// Run the test suite
func TestConfigHashTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigHashTestSuite))
}
