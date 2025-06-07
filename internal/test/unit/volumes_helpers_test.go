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
	}

	tagMap := infra.VolumeTagsToMap(tags)

	if tagMap == nil {
		t.Errorf("tagMap should not be nil")
		return
	}
	if len(tagMap) != 5 {
		t.Errorf("tagMap should have 5 entries, got %d", len(tagMap))
	}
	if tagMap[infra.TagKeyRole] == nil || *tagMap[infra.TagKeyRole] != "volume" {
		t.Errorf("TagKeyRole should be 'volume'")
	}
	if tagMap[infra.TagKeyStatus] == nil || *tagMap[infra.TagKeyStatus] != "free" {
		t.Errorf("TagKeyStatus should be 'free'")
	}
	if tagMap[infra.TagKeyCreated] == nil || *tagMap[infra.TagKeyCreated] != "2024-01-01T00:00:00Z" {
		t.Errorf("TagKeyCreated should be '2024-01-01T00:00:00Z'")
	}
	if tagMap[infra.TagKeyLastUsed] == nil || *tagMap[infra.TagKeyLastUsed] != "2024-01-01T01:00:00Z" {
		t.Errorf("TagKeyLastUsed should be '2024-01-01T01:00:00Z'")
	}
	if tagMap[infra.TagKeyVolumeID] == nil || *tagMap[infra.TagKeyVolumeID] != "vol123" {
		t.Errorf("TagKeyVolumeID should be 'vol123'")
	}
}

func TestVolumeTagsToMapEmptyValues(t *testing.T) {
	var tags infra.VolumeTags // zero value
	tagMap := infra.VolumeTagsToMap(tags)

	if tagMap == nil {
		t.Errorf("tagMap should not be nil")
		return
	}
	if len(tagMap) != 5 {
		t.Errorf("tagMap should have 5 entries, got %d", len(tagMap))
	}
	for key, value := range tagMap {
		if value == nil || *value != "" {
			t.Errorf("Value should be empty string for key: %s", key)
		}
	}
}
