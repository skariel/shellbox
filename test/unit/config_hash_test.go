//go:build unit

package unit

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"shellbox/internal/infra"
	"shellbox/test"
)

// TestFormatConfig tests the configuration formatting function
func TestFormatConfig(t *testing.T) {
	env := test.SetupMinimalTestEnvironment(t)
	_ = env // For later use if needed

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
		t.Run(tc.name, func(t *testing.T) {
			config := infra.FormatConfig(tc.suffix)

			if tc.expectedNotEmpty {
				if config == "" {
					t.Errorf("Config should not be empty")
				}
			}

			for _, expected := range tc.expectedContains {
				if !strings.Contains(config, expected) {
					t.Errorf("Config should contain: %s", expected)
				}
			}

			// Test that config contains network information
			if !strings.Contains(config, "10.0.0.0/8") {
				t.Errorf("Should contain VNet CIDR")
			}
			if !strings.Contains(config, "10.0.0.0/24") {
				t.Errorf("Should contain bastion subnet CIDR")
			}
			if !strings.Contains(config, "10.1.0.0/16") {
				t.Errorf("Should contain boxes subnet CIDR")
			}

			// Test that config is well-structured
			lines := strings.Split(config, "\n")
			if len(lines) <= 5 {
				t.Errorf("Config should have multiple lines, got %d", len(lines))
			}
		})
	}
}

// TestFormatConfigConsistency tests that the same suffix produces the same config
func TestFormatConfigConsistency(t *testing.T) {
	suffix := "consistency-test"

	config1 := infra.FormatConfig(suffix)
	config2 := infra.FormatConfig(suffix)

	if config1 != config2 {
		t.Errorf("Same suffix should produce identical config")
	}
	if config1 == "" {
		t.Errorf("Config should not be empty")
	}
}

// TestFormatConfigUniqueness tests that different suffixes produce different configs
func TestFormatConfigUniqueness(t *testing.T) {
	config1 := infra.FormatConfig("suffix1")
	config2 := infra.FormatConfig("suffix2")

	if config1 == config2 {
		t.Errorf("Different suffixes should produce different configs")
	}
	if config1 == "" {
		t.Errorf("Config1 should not be empty")
	}
	if config2 == "" {
		t.Errorf("Config2 should not be empty")
	}

	// Both should contain common elements
	if !strings.Contains(config1, "Network Configuration") {
		t.Errorf("Config1 should contain Network Configuration")
	}
	if !strings.Contains(config2, "Network Configuration") {
		t.Errorf("Config2 should contain Network Configuration")
	}

	// But should contain their respective suffixes
	if !strings.Contains(config1, "suffix1") {
		t.Errorf("Config1 should contain suffix1")
	}
	if !strings.Contains(config2, "suffix2") {
		t.Errorf("Config2 should contain suffix2")
	}
	if strings.Contains(config1, "suffix2") {
		t.Errorf("Config1 should not contain suffix2")
	}
	if strings.Contains(config2, "suffix1") {
		t.Errorf("Config2 should not contain suffix1")
	}
}

// TestGenerateConfigHash tests the configuration hash generation
func TestGenerateConfigHash(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			hash, err := infra.GenerateConfigHash(tc.suffix)

			if tc.shouldError {
				if err == nil {
					t.Errorf("Should return error")
				}
			} else {
				if err != nil {
					t.Fatalf("Should not return error: %v", err)
				}
				if len(hash) != tc.expectedLength {
					t.Errorf("Hash should be %d characters, got %d", tc.expectedLength, len(hash))
				}

				// Test that hash contains only hex characters
				for _, char := range hash {
					if !isHexChar(char) {
						t.Errorf("Hash should contain only hex characters: %c", char)
					}
				}

				// Test that hash is not empty
				if hash == "" {
					t.Errorf("Hash should not be empty")
				}
			}
		})
	}
}

// TestGenerateConfigHashConsistency tests that the same suffix produces the same hash
func TestGenerateConfigHashConsistency(t *testing.T) {
	suffix := "hash-consistency-test"

	hash1, err1 := infra.GenerateConfigHash(suffix)
	hash2, err2 := infra.GenerateConfigHash(suffix)

	if err1 != nil {
		t.Fatalf("First hash generation should not error: %v", err1)
	}
	if err2 != nil {
		t.Fatalf("Second hash generation should not error: %v", err2)
	}

	if hash1 != hash2 {
		t.Errorf("Same suffix should produce identical hash")
	}
	if len(hash1) != 8 {
		t.Errorf("Hash should be 8 characters, got %d", len(hash1))
	}
	if len(hash2) != 8 {
		t.Errorf("Hash should be 8 characters, got %d", len(hash2))
	}
}

// TestGenerateConfigHashUniqueness tests that different suffixes produce different hashes
func TestGenerateConfigHashUniqueness(t *testing.T) {
	suffixes := []string{"test1", "test2", "different", "another-test", ""}
	hashes := make(map[string]string)

	for _, suffix := range suffixes {
		hash, err := infra.GenerateConfigHash(suffix)
		if err != nil {
			t.Fatalf("Hash generation should not error for suffix: %s, error: %v", suffix, err)
		}

		// Check that this hash is unique
		for otherSuffix, otherHash := range hashes {
			if suffix != otherSuffix {
				if hash == otherHash {
					t.Errorf("Different suffixes should produce different hashes: %s vs %s", suffix, otherSuffix)
				}
			}
		}

		hashes[suffix] = hash
	}
}

// TestHashAlgorithmCorrectness tests that the hash is generated correctly
func TestHashAlgorithmCorrectness(t *testing.T) {
	suffix := "algorithm-test"

	// Generate hash using the function
	actualHash, err := infra.GenerateConfigHash(suffix)
	if err != nil {
		t.Fatalf("Hash generation should not error: %v", err)
	}

	// Generate hash manually to verify algorithm
	config := infra.FormatConfig(suffix)
	hasher := sha256.New()
	hasher.Write([]byte(config))
	expectedHash := hex.EncodeToString(hasher.Sum(nil))[:8]

	if actualHash != expectedHash {
		t.Errorf("Hash should match manual calculation, got %s, want %s", actualHash, expectedHash)
	}
	if len(actualHash) != 8 {
		t.Errorf("Hash should be truncated to 8 characters, got %d", len(actualHash))
	}
}

// TestHashCollisionResistance tests basic collision resistance properties
func TestHashCollisionResistance(t *testing.T) {
	// Test with similar suffixes that might cause collisions in a weak hash
	similarSuffixes := []string{
		"test", "test1", "test2", "test12", "test21",
		"a", "aa", "aaa", "ab", "ba",
		"production", "productio", "productin",
	}

	hashes := make(map[string][]string)

	for _, suffix := range similarSuffixes {
		hash, err := infra.GenerateConfigHash(suffix)
		if err != nil {
			t.Fatalf("Hash generation should not error: %v", err)
		}

		if existingSuffixes, exists := hashes[hash]; exists {
			// Collision detected - this is very unlikely with SHA256 but let's document it
			t.Logf("Hash collision detected: %s and %v both hash to %s",
				suffix, existingSuffixes, hash)
		} else {
			hashes[hash] = []string{suffix}
		}
	}

	// For this small test set, we expect no collisions with SHA256
	if len(hashes) != len(similarSuffixes) {
		t.Errorf("Should have unique hashes for all test suffixes (collision unlikely with SHA256)")
	}
}

// Helper function to check if character is a valid hex digit
func isHexChar(char rune) bool {
	return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
}

// Run the test suite
func TestConfigHashTestSuite(t *testing.T) {
	// Run all the individual test functions
	t.Run("TestFormatConfig", TestFormatConfig)
	t.Run("TestFormatConfigConsistency", TestFormatConfigConsistency)
	t.Run("TestFormatConfigUniqueness", TestFormatConfigUniqueness)
	t.Run("TestGenerateConfigHash", TestGenerateConfigHash)
	t.Run("TestGenerateConfigHashConsistency", TestGenerateConfigHashConsistency)
	t.Run("TestGenerateConfigHashUniqueness", TestGenerateConfigHashUniqueness)
	t.Run("TestHashAlgorithmCorrectness", TestHashAlgorithmCorrectness)
	t.Run("TestHashCollisionResistance", TestHashCollisionResistance)
}
