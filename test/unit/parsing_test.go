//go:build unit

package unit

import (
	"testing"

	"shellbox/internal/infra"
	"shellbox/internal/sshserver"
)

// TestExtractDiskNameFromID tests disk name extraction from Azure resource ID
func TestExtractDiskNameFromID(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			result := infra.ExtractDiskNameFromID(tc.diskID)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestExtractSuffix tests suffix extraction from resource group names
func TestExtractSuffix(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			result := infra.ExtractSuffix(tc.resourceGroup)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestExtractInstanceIDFromVMName tests instance ID extraction from VM names
func TestExtractInstanceIDFromVMName(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			result := infra.ExtractInstanceIDFromVMName(tc.vmName)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

// TestVolumeTagsToMap tests conversion of volume tags to Azure tags map
func TestVolumeTagsToMap(t *testing.T) {
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
				UserID:    "abc123def456",
			},
			expected: map[string]*string{
				"shellbox:role":     stringPtr("volume"),
				"shellbox:status":   stringPtr("free"),
				"shellbox:created":  stringPtr("2023-01-01T00:00:00Z"),
				"shellbox:lastused": stringPtr("2023-01-01T12:00:00Z"),
				"shellbox:volumeid": stringPtr("vol-123"),
				"shellbox:userid":   stringPtr("abc123def456"),
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
				"shellbox:volumeid": stringPtr(""),
				"shellbox:userid":   stringPtr(""),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := infra.VolumeTagsToMap(tc.tags)

			// Compare each key-value pair
			for key, expectedValue := range tc.expected {
				actualValue, exists := result[key]
				if !exists {
					t.Errorf("Key %s should exist in result", key)
					continue
				}
				if expectedValue == nil {
					if actualValue != nil {
						t.Errorf("Value for key %s should be nil", key)
					}
				} else {
					if actualValue == nil {
						t.Errorf("Value for key %s should not be nil", key)
					} else if *expectedValue != *actualValue {
						t.Errorf("Value for key %s should match: expected %q, got %q", key, *expectedValue, *actualValue)
					}
				}
			}

			// Ensure no extra keys
			if len(result) != len(tc.expected) {
				t.Errorf("Result should have same number of keys: expected %d, got %d", len(tc.expected), len(result))
			}
		})
	}
}

// TestParseArgs tests command line argument parsing
func TestParseArgs(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
			result := sshserver.ParseArgs(tc.cmdLine)
			if len(result) != len(tc.expected) {
				t.Errorf("Expected %d arguments, got %d", len(tc.expected), len(result))
				return
			}
			for i, expected := range tc.expected {
				if result[i] != expected {
					t.Errorf("Argument %d: expected %q, got %q", i, expected, result[i])
				}
			}
		})
	}
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}
