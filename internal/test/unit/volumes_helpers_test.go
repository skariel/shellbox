package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"shellbox/internal/infra"
)

func TestVolumesHelpersTestSuite(t *testing.T) {
	t.Run("TestVolumeTagsToMap", func(t *testing.T) {
		tags := infra.VolumeTags{
			Role:      "volume",
			Status:    "free",
			CreatedAt: "2024-01-01T00:00:00Z",
			LastUsed:  "2024-01-01T01:00:00Z",
			VolumeID:  "vol123",
		}

		tagMap := infra.VolumeTagsToMap(tags)

		// Should contain all tag fields
		assert.NotNil(t, tagMap)
		assert.Equal(t, "volume", *tagMap["shellbox:role"])
		assert.Equal(t, "free", *tagMap["shellbox:status"])
		assert.Equal(t, "2024-01-01T00:00:00Z", *tagMap["shellbox:created"])
		assert.Equal(t, "2024-01-01T01:00:00Z", *tagMap["shellbox:lastused"])

		// Should have exactly 5 entries (role, status, created, lastused, volume_id)
		assert.Len(t, tagMap, 5)
	})

	t.Run("TestVolumeTagsToMapEmptyValues", func(t *testing.T) {
		tags := infra.VolumeTags{
			Role:      "",
			Status:    "",
			CreatedAt: "",
			LastUsed:  "",
		}

		tagMap := infra.VolumeTagsToMap(tags)

		// Should still contain all keys with empty values
		assert.NotNil(t, tagMap)
		assert.Equal(t, "", *tagMap["shellbox:role"])
		assert.Equal(t, "", *tagMap["shellbox:status"])
		assert.Equal(t, "", *tagMap["shellbox:created"])
		assert.Equal(t, "", *tagMap["shellbox:lastused"])
		assert.Len(t, tagMap, 5)
	})

	t.Run("TestVolumeTagsToMapConsistentKeys", func(t *testing.T) {
		tags := infra.VolumeTags{
			Role:      "test",
			Status:    "attached",
			CreatedAt: "2024-06-03T12:00:00Z",
			LastUsed:  "2024-06-03T13:00:00Z",
		}

		tagMap := infra.VolumeTagsToMap(tags)

		// Keys should match the constants
		expectedKeys := []string{
			infra.TagKeyRole,
			infra.TagKeyStatus,
			infra.TagKeyCreated,
			infra.TagKeyLastUsed,
		}

		for _, key := range expectedKeys {
			assert.Contains(t, tagMap, key, "Should contain expected tag key: %s", key)
			assert.NotNil(t, tagMap[key], "Tag value should not be nil for key: %s", key)
		}
	})

	t.Run("TestVolumeTagsStructure", func(t *testing.T) {
		// Test that VolumeTags struct can be properly initialized
		tags := infra.VolumeTags{
			Role:      infra.ResourceRoleVolume,
			Status:    infra.ResourceStatusFree,
			CreatedAt: "2024-01-01T00:00:00Z",
			LastUsed:  "2024-01-01T01:00:00Z",
		}

		assert.Equal(t, infra.ResourceRoleVolume, tags.Role)
		assert.Equal(t, infra.ResourceStatusFree, tags.Status)
		assert.Equal(t, "2024-01-01T00:00:00Z", tags.CreatedAt)
		assert.Equal(t, "2024-01-01T01:00:00Z", tags.LastUsed)
	})

	t.Run("TestVolumeTagsWithResourceConstants", func(t *testing.T) {
		// Test with all valid resource status values
		statusValues := []string{
			infra.ResourceStatusFree,
			infra.ResourceStatusAttached,
		}

		for _, status := range statusValues {
			tags := infra.VolumeTags{
				Role:      infra.ResourceRoleVolume,
				Status:    status,
				CreatedAt: "2024-01-01T00:00:00Z",
				LastUsed:  "2024-01-01T01:00:00Z",
			}

			tagMap := infra.VolumeTagsToMap(tags)
			assert.Equal(t, status, *tagMap[infra.TagKeyStatus])
		}
	})

	t.Run("TestTagKeyConstants", func(t *testing.T) {
		// Verify tag key constants have expected values
		assert.Equal(t, "shellbox:role", infra.TagKeyRole)
		assert.Equal(t, "shellbox:status", infra.TagKeyStatus)
		assert.Equal(t, "shellbox:created", infra.TagKeyCreated)
		assert.Equal(t, "shellbox:lastused", infra.TagKeyLastUsed)
	})

	t.Run("TestVolumeTagsDefaultValues", func(t *testing.T) {
		// Test zero-value VolumeTags
		var tags infra.VolumeTags
		tagMap := infra.VolumeTagsToMap(tags)

		assert.NotNil(t, tagMap)
		assert.Len(t, tagMap, 5)

		// All values should be empty strings
		for key, value := range tagMap {
			assert.NotNil(t, value, "Value should not be nil for key: %s", key)
			assert.Equal(t, "", *value, "Value should be empty string for key: %s", key)
		}
	})

	t.Run("TestVolumeInfoStruct", func(t *testing.T) {
		// Test that VolumeInfo struct can be properly initialized
		volumeInfo := infra.VolumeInfo{
			ResourceID: "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Compute/disks/disk1",
			Name:       "disk1",
		}

		assert.Equal(t, "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Compute/disks/disk1", volumeInfo.ResourceID)
		assert.Equal(t, "disk1", volumeInfo.Name)
	})

	t.Run("TestVolumeConfigStruct", func(t *testing.T) {
		// Test that VolumeConfig struct can be properly initialized
		config := infra.VolumeConfig{
			DiskSize: 100,
		}

		assert.Equal(t, int32(100), config.DiskSize)
	})
}
