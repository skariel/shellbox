package unit

import (
	"testing"

	"shellbox/internal/infra"
)

func TestVolumeTagsToMap(t *testing.T) {
	tags := infra.VolumeTags{
		Role:      "volume",
		Status:    "free",
		CreatedAt: "2024-01-01T00:00:00Z",
		LastUsed:  "2024-01-01T01:00:00Z",
		VolumeID:  "vol123",
		UserID:    "abc123def456",
		BoxName:   "mybox",
	}

	tagMap := infra.VolumeTagsToMap(tags)

	if tagMap == nil {
		t.Errorf("tagMap should not be nil")
		return
	}
	if len(tagMap) != 7 {
		t.Errorf("tagMap should have 7 entries, got %d", len(tagMap))
	}

	// Check all expected tag values
	expected := map[string]string{
		infra.TagKeyRole:     "volume",
		infra.TagKeyStatus:   "free",
		infra.TagKeyCreated:  "2024-01-01T00:00:00Z",
		infra.TagKeyLastUsed: "2024-01-01T01:00:00Z",
		infra.TagKeyVolumeID: "vol123",
		infra.TagKeyUserID:   "abc123def456",
		infra.TagKeyBoxName:  "mybox",
	}

	for key, expectedValue := range expected {
		if tagMap[key] == nil || *tagMap[key] != expectedValue {
			t.Errorf("Tag %s should be '%s', got '%v'", key, expectedValue, tagMap[key])
		}
	}
}

func TestVolumeTagsToMapEmptyValues(t *testing.T) {
	var tags infra.VolumeTags // zero value
	tagMap := infra.VolumeTagsToMap(tags)

	if tagMap == nil {
		t.Errorf("tagMap should not be nil")
		return
	}
	if len(tagMap) != 7 {
		t.Errorf("tagMap should have 7 entries, got %d", len(tagMap))
	}
	for key, value := range tagMap {
		if value == nil || *value != "" {
			t.Errorf("Value should be empty string for key: %s", key)
		}
	}
}
