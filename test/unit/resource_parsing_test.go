//go:build unit

package unit

import (
	"testing"
	"time"

	"shellbox/internal/infra"
)

// TestParseBasicFields tests parsing of basic resource fields from Azure Resource Graph response
func TestParseBasicFields(t *testing.T) {
	testCases := []struct {
		name         string
		resourceMap  map[string]interface{}
		expectedName string
		expectedID   string
		expectedLoc  string
	}{
		{
			name: "Complete basic fields",
			resourceMap: map[string]interface{}{
				"name":     "test-vm-123",
				"id":       "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/test-vm-123",
				"location": "westus2",
				"type":     "microsoft.compute/virtualmachines",
			},
			expectedName: "test-vm-123",
			expectedID:   "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/test-vm-123",
			expectedLoc:  "westus2",
		},
		{
			name: "Missing fields",
			resourceMap: map[string]interface{}{
				"type": "microsoft.compute/disks",
			},
			expectedName: "",
			expectedID:   "",
			expectedLoc:  "",
		},
		{
			name: "Wrong field types",
			resourceMap: map[string]interface{}{
				"name":     123,
				"id":       true,
				"location": []string{"westus2"},
			},
			expectedName: "",
			expectedID:   "",
			expectedLoc:  "",
		},
		{
			name:         "Empty map",
			resourceMap:  map[string]interface{}{},
			expectedName: "",
			expectedID:   "",
			expectedLoc:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &infra.ResourceInfo{}
			infra.ParseBasicFields(resource, tc.resourceMap)

			if resource.Name != tc.expectedName {
				t.Errorf("Expected Name %q, got %q", tc.expectedName, resource.Name)
			}
			if resource.ID != tc.expectedID {
				t.Errorf("Expected ID %q, got %q", tc.expectedID, resource.ID)
			}
			if resource.Location != tc.expectedLoc {
				t.Errorf("Expected Location %q, got %q", tc.expectedLoc, resource.Location)
			}
		})
	}
}

// TestParseTags tests parsing of resource tags from Azure Resource Graph response
func TestParseTags(t *testing.T) {
	testCases := []struct {
		name        string
		resourceMap map[string]interface{}
		expectedTag func(*infra.ResourceInfo) bool
	}{
		{
			name: "Valid tags map",
			resourceMap: map[string]interface{}{
				"tags": map[string]interface{}{
					"shellbox:role":     "instance",
					"shellbox:status":   "free",
					"shellbox:created":  "2023-01-01T00:00:00Z",
					"shellbox:lastused": "2023-01-01T12:00:00Z",
					"environment":       "production",
				},
			},
			expectedTag: func(r *infra.ResourceInfo) bool {
				expectedCreated, _ := time.Parse(time.RFC3339, "2023-01-01T00:00:00Z")
				expectedLastUsed, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
				return r.Role == "instance" &&
					r.Status == "free" &&
					r.CreatedAt != nil && r.CreatedAt.Equal(expectedCreated) &&
					r.LastUsed != nil && r.LastUsed.Equal(expectedLastUsed)
			},
		},
		{
			name: "Missing shellbox tags",
			resourceMap: map[string]interface{}{
				"tags": map[string]interface{}{
					"environment": "production",
					"team":        "platform",
				},
			},
			expectedTag: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == "" && r.CreatedAt == nil && r.LastUsed == nil
			},
		},
		{
			name: "Wrong tag value types",
			resourceMap: map[string]interface{}{
				"tags": map[string]interface{}{
					"shellbox:role":   123,
					"shellbox:status": true,
				},
			},
			expectedTag: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == ""
			},
		},
		{
			name: "No tags field",
			resourceMap: map[string]interface{}{
				"name": "test-resource",
			},
			expectedTag: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == "" && r.CreatedAt == nil && r.LastUsed == nil
			},
		},
		{
			name: "Tags field wrong type",
			resourceMap: map[string]interface{}{
				"tags": "not-a-map",
			},
			expectedTag: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == "" && r.CreatedAt == nil && r.LastUsed == nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &infra.ResourceInfo{}
			infra.ParseTags(resource, tc.resourceMap)

			if !tc.expectedTag(resource) {
				t.Error("Tag parsing result should match expected pattern")
			}
		})
	}
}

// TestParseProjectedFields tests parsing of projected fields with specific tag extraction
func TestParseProjectedFields(t *testing.T) {
	testCases := []struct {
		name         string
		resourceMap  map[string]interface{}
		expectedFunc func(*infra.ResourceInfo) bool
	}{
		{
			name: "VM with projected role and status",
			resourceMap: map[string]interface{}{
				"role":   "instance",
				"status": "connected",
			},
			expectedFunc: func(r *infra.ResourceInfo) bool {
				return r.Role == "instance" && r.Status == "connected"
			},
		},
		{
			name: "Disk with projected role and status",
			resourceMap: map[string]interface{}{
				"role":   "volume",
				"status": "attached",
			},
			expectedFunc: func(r *infra.ResourceInfo) bool {
				return r.Role == "volume" && r.Status == "attached"
			},
		},
		{
			name: "Missing projected fields",
			resourceMap: map[string]interface{}{
				"name": "some-resource",
			},
			expectedFunc: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == ""
			},
		},
		{
			name: "Wrong projected field types",
			resourceMap: map[string]interface{}{
				"role":   123,
				"status": []string{"free"},
			},
			expectedFunc: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == ""
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := &infra.ResourceInfo{}
			infra.ParseProjectedFields(resource, tc.resourceMap)

			if !tc.expectedFunc(resource) {
				t.Error("Projected fields parsing should match expected pattern")
			}
		})
	}
}

// TestCompleteResourceInfoParsing tests the complete parsing process
func TestCompleteResourceInfoParsing(t *testing.T) {
	testCases := []struct {
		name        string
		resourceMap map[string]interface{}
		expected    infra.ResourceInfo
	}{
		{
			name: "Complete VM resource",
			resourceMap: map[string]interface{}{
				"name":     "shellbox-box-abc123-test",
				"id":       "/subscriptions/sub/resourceGroups/shellbox-test/providers/Microsoft.Compute/virtualMachines/shellbox-box-abc123-test",
				"location": "westus2",
				"tags": map[string]interface{}{
					"shellbox:role":     "instance",
					"shellbox:status":   "free",
					"shellbox:created":  "2023-01-01T00:00:00Z",
					"shellbox:lastused": "2023-01-01T12:00:00Z",
				},
				"role":   "instance",
				"status": "free",
			},
			expected: infra.ResourceInfo{
				Name:     "shellbox-box-abc123-test",
				ID:       "/subscriptions/sub/resourceGroups/shellbox-test/providers/Microsoft.Compute/virtualMachines/shellbox-box-abc123-test",
				Location: "westus2",
				Role:     "instance",
				Status:   "free",
				// Note: CreatedAt and LastUsed will be set by the parsing function
			},
		},
		{
			name: "Complete volume resource",
			resourceMap: map[string]interface{}{
				"name":     "volume-pool-disk-456",
				"id":       "/subscriptions/sub/resourceGroups/shellbox-test/providers/Microsoft.Compute/disks/volume-pool-disk-456",
				"location": "westus2",
				"tags": map[string]interface{}{
					"shellbox:role":     "volume",
					"shellbox:status":   "attached",
					"shellbox:created":  "2023-01-02T00:00:00Z",
					"shellbox:lastused": "2023-01-02T18:00:00Z",
				},
				"role":   "volume",
				"status": "attached",
			},
			expected: infra.ResourceInfo{
				Name:     "volume-pool-disk-456",
				ID:       "/subscriptions/sub/resourceGroups/shellbox-test/providers/Microsoft.Compute/disks/volume-pool-disk-456",
				Location: "westus2",
				Role:     "volume",
				Status:   "attached",
				// Note: CreatedAt and LastUsed will be set by the parsing function
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := infra.ParseResourceInfo(tc.resourceMap)

			if result.Name != tc.expected.Name {
				t.Errorf("Expected Name %q, got %q", tc.expected.Name, result.Name)
			}
			if result.ID != tc.expected.ID {
				t.Errorf("Expected ID %q, got %q", tc.expected.ID, result.ID)
			}
			if result.Location != tc.expected.Location {
				t.Errorf("Expected Location %q, got %q", tc.expected.Location, result.Location)
			}
			if result.Role != tc.expected.Role {
				t.Errorf("Expected Role %q, got %q", tc.expected.Role, result.Role)
			}
			if result.Status != tc.expected.Status {
				t.Errorf("Expected Status %q, got %q", tc.expected.Status, result.Status)
			}
			// Check timestamps separately by parsing from the input
			if tagsInterface, ok := tc.resourceMap["tags"]; ok {
				if tagsMap, ok := tagsInterface.(map[string]interface{}); ok {
					if createdStr, ok := tagsMap["shellbox:created"].(string); ok {
						expectedCreated, _ := time.Parse(time.RFC3339, createdStr)
						if result.CreatedAt == nil {
							t.Errorf("Expected CreatedAt to be parsed, got nil")
						} else if !result.CreatedAt.Equal(expectedCreated) {
							t.Errorf("Expected CreatedAt %v, got %v", expectedCreated, *result.CreatedAt)
						}
					}
					if lastUsedStr, ok := tagsMap["shellbox:lastused"].(string); ok {
						expectedLastUsed, _ := time.Parse(time.RFC3339, lastUsedStr)
						if result.LastUsed == nil {
							t.Errorf("Expected LastUsed to be parsed, got nil")
						} else if !result.LastUsed.Equal(expectedLastUsed) {
							t.Errorf("Expected LastUsed %v, got %v", expectedLastUsed, *result.LastUsed)
						}
					}
				}
			}
		})
	}
}
