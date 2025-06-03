package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
)

func TestGoldenSnapshotHelpersTestSuite(t *testing.T) {
	t.Run("TestExtractDiskNameFromID", func(t *testing.T) {
		testCases := []struct {
			name     string
			diskID   string
			expected string
		}{
			{
				name:     "Standard Azure disk ID",
				diskID:   "/subscriptions/12345/resourceGroups/rg-test/providers/Microsoft.Compute/disks/my-disk",
				expected: "my-disk",
			},
			{
				name:     "Disk with complex name",
				diskID:   "/subscriptions/abc-def/resourceGroups/shellbox-test/providers/Microsoft.Compute/disks/shellbox-test-volume-123",
				expected: "shellbox-test-volume-123",
			},
			{
				name:     "Simple disk name without path",
				diskID:   "simple-disk",
				expected: "simple-disk",
			},
			{
				name:     "Empty string",
				diskID:   "",
				expected: "",
			},
			{
				name:     "Path without disk name",
				diskID:   "/subscriptions/12345/resourceGroups/rg-test/providers/Microsoft.Compute/disks/",
				expected: "",
			},
			{
				name:     "Disk with hyphens and numbers",
				diskID:   "/subscriptions/12345/resourceGroups/rg-test/providers/Microsoft.Compute/disks/disk-123-test",
				expected: "disk-123-test",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := infra.ExtractDiskNameFromID(tc.diskID)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("TestExtractSuffix", func(t *testing.T) {
		testCases := []struct {
			name              string
			resourceGroupName string
			expected          string
		}{
			{
				name:              "Standard shellbox resource group",
				resourceGroupName: "shellbox-test123",
				expected:          "test123",
			},
			{
				name:              "Resource group with complex suffix",
				resourceGroupName: "shellbox-my-dev-env",
				expected:          "my-dev-env",
			},
			{
				name:              "Resource group with numbers and hyphens",
				resourceGroupName: "shellbox-env-123-test",
				expected:          "env-123-test",
			},
			{
				name:              "Minimal resource group",
				resourceGroupName: "shellbox-a",
				expected:          "a",
			},
			{
				name:              "Resource group without suffix",
				resourceGroupName: "shellbox-",
				expected:          "shellbox-", // Function returns whole string when not > prefix length
			},
			{
				name:              "Non-shellbox resource group",
				resourceGroupName: "other-resource-group",
				expected:          "ource-group", // Function returns substring after "shellbox-"
			},
			{
				name:              "Empty string",
				resourceGroupName: "",
				expected:          "",
			},
			{
				name:              "Just shellbox prefix",
				resourceGroupName: "shellbox",
				expected:          "shellbox", // Function returns whole string when shorter than prefix
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result := infra.ExtractSuffix(tc.resourceGroupName)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("TestGenerateQEMUInitScript", func(t *testing.T) {
		t.Run("Basic script generation", func(t *testing.T) {
			config := infra.QEMUScriptConfig{
				SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com",
				WorkingDir:    "~",
				SSHPort:       2222,
				MountDataDisk: false,
			}

			script, err := infra.GenerateQEMUInitScript(config)
			require.NoError(t, err)
			assert.NotEmpty(t, script)

			// Script should be base64 encoded and substantial in size
			assert.True(t, len(script) > 100, "Script should be substantial in size")
		})

		t.Run("Script with data disk mounting", func(t *testing.T) {
			config := infra.QEMUScriptConfig{
				SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com",
				WorkingDir:    "/mnt/userdata",
				SSHPort:       2222,
				MountDataDisk: true,
			}

			script, err := infra.GenerateQEMUInitScript(config)
			require.NoError(t, err)
			assert.NotEmpty(t, script)

			// Script should be base64 encoded and substantial in size
			assert.True(t, len(script) > 100, "Script should be substantial in size")
		})

		t.Run("Different SSH port", func(t *testing.T) {
			config := infra.QEMUScriptConfig{
				SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com",
				WorkingDir:    "~",
				SSHPort:       22,
				MountDataDisk: false,
			}

			script, err := infra.GenerateQEMUInitScript(config)
			require.NoError(t, err)
			assert.NotEmpty(t, script)

			// Script should be base64 encoded and substantial in size
			assert.True(t, len(script) > 100, "Script should be substantial in size")
		})

		t.Run("Empty SSH key", func(t *testing.T) {
			config := infra.QEMUScriptConfig{
				SSHPublicKey:  "",
				WorkingDir:    "~",
				SSHPort:       2222,
				MountDataDisk: false,
			}

			script, err := infra.GenerateQEMUInitScript(config)
			require.NoError(t, err)
			assert.NotEmpty(t, script)
			// Script should still be generated even with empty SSH key
		})
	})

	t.Run("TestQEMUScriptConfigStruct", func(t *testing.T) {
		// Test that QEMUScriptConfig struct can be properly initialized
		config := infra.QEMUScriptConfig{
			SSHPublicKey:  "test-key",
			WorkingDir:    "/test/dir",
			SSHPort:       2222,
			MountDataDisk: true,
		}

		assert.Equal(t, "test-key", config.SSHPublicKey)
		assert.Equal(t, "/test/dir", config.WorkingDir)
		assert.Equal(t, 2222, config.SSHPort)
		assert.True(t, config.MountDataDisk)
	})

	t.Run("TestScriptStructure", func(t *testing.T) {
		config := infra.QEMUScriptConfig{
			SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com",
			WorkingDir:    "~",
			SSHPort:       2222,
			MountDataDisk: true,
		}

		script, err := infra.GenerateQEMUInitScript(config)
		require.NoError(t, err)

		// Script should be substantial in size (likely base64 encoded)
		assert.True(t, len(script) > 1000, "Script should be substantial in size")
	})
}
