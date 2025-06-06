//go:build unit

package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/sshserver"
	"shellbox/internal/test"
)

// ParsingTestSuite tests string and data parsing functions
type ParsingTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *ParsingTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestExtractDiskNameFromID tests disk name extraction from Azure resource ID
func (suite *ParsingTestSuite) TestExtractDiskNameFromID() {
	testCases := []struct {
		name     string
		diskID   string
		expected string
	}{
		{
			name:     "Valid Azure disk ID",
			diskID:   "/subscriptions/sub-id/resourceGroups/rg-name/providers/Microsoft.Compute/disks/disk-name",
			expected: "disk-name",
		},
		{
			name:     "Valid Azure disk ID with hyphens",
			diskID:   "/subscriptions/sub-id/resourceGroups/rg-name/providers/Microsoft.Compute/disks/volume-pool-disk-123-456",
			expected: "volume-pool-disk-123-456",
		},
		{
			name:     "Single segment",
			diskID:   "simple-disk-name",
			expected: "simple-disk-name",
		},
		{
			name:     "Empty string",
			diskID:   "",
			expected: "",
		},
		{
			name:     "Just slashes",
			diskID:   "///",
			expected: "",
		},
		{
			name:     "Ending with slash",
			diskID:   "/path/to/disk/",
			expected: "",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := infra.ExtractDiskNameFromID(tc.diskID)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractSuffix tests suffix extraction from resource group names
func (suite *ParsingTestSuite) TestExtractSuffix() {
	testCases := []struct {
		name          string
		resourceGroup string
		expected      string
	}{
		{
			name:          "Valid shellbox resource group",
			resourceGroup: "shellbox-test123",
			expected:      "test123",
		},
		{
			name:          "Valid shellbox resource group with hyphens",
			resourceGroup: "shellbox-my-test-suffix",
			expected:      "my-test-suffix",
		},
		{
			name:          "Just prefix",
			resourceGroup: "shellbox-",
			expected:      "",
		},
		{
			name:          "No prefix",
			resourceGroup: "other-resource-group",
			expected:      "other-resource-group",
		},
		{
			name:          "Empty string",
			resourceGroup: "",
			expected:      "",
		},
		{
			name:          "Just shellbox",
			resourceGroup: "shellbox",
			expected:      "shellbox",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := infra.ExtractSuffix(tc.resourceGroup)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestExtractInstanceIDFromVMName tests instance ID extraction from VM names
func (suite *ParsingTestSuite) TestExtractInstanceIDFromVMName() {
	testCases := []struct {
		name     string
		vmName   string
		expected string
	}{
		{
			name:     "Valid box VM name",
			vmName:   "shellbox-box-abc123-test",
			expected: "abc123",
		},
		{
			name:     "Valid VM name with multiple parts",
			vmName:   "shellbox-box-instance-id-456-suffix",
			expected: "id-456",
		},
		{
			name:     "Short VM name",
			vmName:   "box-id",
			expected: "",
		},
		{
			name:     "Single part",
			vmName:   "vmname",
			expected: "",
		},
		{
			name:     "Empty string",
			vmName:   "",
			expected: "",
		},
		{
			name:     "Minimum valid parts",
			vmName:   "a-b-c-d",
			expected: "c",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := infra.ExtractInstanceIDFromVMName(tc.vmName)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestVolumeTagsToMap tests conversion of volume tags to Azure tags map
func (suite *ParsingTestSuite) TestVolumeTagsToMap() {
	testCases := []struct {
		name     string
		tags     infra.VolumeTags
		expected map[string]*string
	}{
		{
			name: "Complete volume tags",
			tags: infra.VolumeTags{
				Role:      "volume",
				Status:    "free",
				CreatedAt: "2023-01-01T00:00:00Z",
				LastUsed:  "2023-01-01T12:00:00Z",
				VolumeID:  "vol-123",
			},
			expected: map[string]*string{
				"shellbox:role":     stringPtr("volume"),
				"shellbox:status":   stringPtr("free"),
				"shellbox:created":  stringPtr("2023-01-01T00:00:00Z"),
				"shellbox:lastused": stringPtr("2023-01-01T12:00:00Z"),
				"volumeID":          stringPtr("vol-123"),
			},
		},
		{
			name: "Empty volume tags",
			tags: infra.VolumeTags{},
			expected: map[string]*string{
				"shellbox:role":     stringPtr(""),
				"shellbox:status":   stringPtr(""),
				"shellbox:created":  stringPtr(""),
				"shellbox:lastused": stringPtr(""),
				"volumeID":          stringPtr(""),
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := infra.VolumeTagsToMap(tc.tags)

			// Compare each key-value pair
			for key, expectedValue := range tc.expected {
				actualValue, exists := result[key]
				assert.True(t, exists, "Key %s should exist in result", key)
				if expectedValue == nil {
					assert.Nil(t, actualValue, "Value for key %s should be nil", key)
				} else {
					assert.NotNil(t, actualValue, "Value for key %s should not be nil", key)
					assert.Equal(t, *expectedValue, *actualValue, "Value for key %s should match", key)
				}
			}

			// Ensure no extra keys
			assert.Len(t, result, len(tc.expected), "Result should have same number of keys")
		})
	}
}

// TestParseArgs tests command line argument parsing
func (suite *ParsingTestSuite) TestParseArgs() {
	testCases := []struct {
		name     string
		cmdLine  string
		expected []string
	}{
		{
			name:     "Simple command",
			cmdLine:  "spinup user123",
			expected: []string{"spinup", "user123"},
		},
		{
			name:     "Command with multiple args",
			cmdLine:  "spinup user123 --verbose --timeout 30",
			expected: []string{"spinup", "user123", "--verbose", "--timeout", "30"},
		},
		{
			name:     "Command with extra spaces",
			cmdLine:  "  spinup   user123  ",
			expected: []string{"spinup", "user123"},
		},
		{
			name:     "Empty command",
			cmdLine:  "",
			expected: []string{},
		},
		{
			name:     "Just spaces",
			cmdLine:  "   ",
			expected: []string{},
		},
		{
			name:     "Single word",
			cmdLine:  "help",
			expected: []string{"help"},
		},
		{
			name:     "Command with tabs",
			cmdLine:  "spinup\tuser123",
			expected: []string{"spinup", "user123"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := sshserver.ParseArgs(tc.cmdLine)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

// Run the test suite
func TestParsingTestSuite(t *testing.T) {
	suite.Run(t, new(ParsingTestSuite))
}
