//go:build unit

package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

// ResourceParsingTestSuite tests resource info parsing functions
type ResourceParsingTestSuite struct {
	suite.Suite
	env *test.Environment
}

// SetupSuite runs once before all tests in the suite
func (suite *ResourceParsingTestSuite) SetupSuite() {
	suite.env = test.SetupMinimalTestEnvironment(suite.T())
}

// TestParseBasicFields tests parsing of basic resource fields from Azure Resource Graph response
func (suite *ResourceParsingTestSuite) TestParseBasicFields() {
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
		suite.T().Run(tc.name, func(t *testing.T) {
			resource := &infra.ResourceInfo{}
			infra.ParseBasicFields(resource, tc.resourceMap)

			assert.Equal(t, tc.expectedName, resource.Name)
			assert.Equal(t, tc.expectedID, resource.ID)
			assert.Equal(t, tc.expectedLoc, resource.Location)
		})
	}
}

// TestParseTags tests parsing of resource tags from Azure Resource Graph response
func (suite *ResourceParsingTestSuite) TestParseTags() {
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
				return r.Role == "instance" &&
					r.Status == "free" &&
					r.CreatedAt == "2023-01-01T00:00:00Z" &&
					r.LastUsed == "2023-01-01T12:00:00Z"
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
				return r.Role == "" && r.Status == "" && r.CreatedAt == "" && r.LastUsed == ""
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
				return r.Role == "" && r.Status == "" && r.CreatedAt == "" && r.LastUsed == ""
			},
		},
		{
			name: "Tags field wrong type",
			resourceMap: map[string]interface{}{
				"tags": "not-a-map",
			},
			expectedTag: func(r *infra.ResourceInfo) bool {
				return r.Role == "" && r.Status == "" && r.CreatedAt == "" && r.LastUsed == ""
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			resource := &infra.ResourceInfo{}
			infra.ParseTags(resource, tc.resourceMap)

			assert.True(t, tc.expectedTag(resource), "Tag parsing result should match expected pattern")
		})
	}
}

// TestParseProjectedFields tests parsing of projected fields with specific tag extraction
func (suite *ResourceParsingTestSuite) TestParseProjectedFields() {
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
		suite.T().Run(tc.name, func(t *testing.T) {
			resource := &infra.ResourceInfo{}
			infra.ParseProjectedFields(resource, tc.resourceMap)

			assert.True(t, tc.expectedFunc(resource), "Projected fields parsing should match expected pattern")
		})
	}
}

// TestCompleteResourceInfoParsing tests the complete parsing process
func (suite *ResourceParsingTestSuite) TestCompleteResourceInfoParsing() {
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
				Name:      "shellbox-box-abc123-test",
				ID:        "/subscriptions/sub/resourceGroups/shellbox-test/providers/Microsoft.Compute/virtualMachines/shellbox-box-abc123-test",
				Location:  "westus2",
				Role:      "instance",
				Status:    "free",
				CreatedAt: "2023-01-01T00:00:00Z",
				LastUsed:  "2023-01-01T12:00:00Z",
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
				Name:      "volume-pool-disk-456",
				ID:        "/subscriptions/sub/resourceGroups/shellbox-test/providers/Microsoft.Compute/disks/volume-pool-disk-456",
				Location:  "westus2",
				Role:      "volume",
				Status:    "attached",
				CreatedAt: "2023-01-02T00:00:00Z",
				LastUsed:  "2023-01-02T18:00:00Z",
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result := infra.ParseResourceInfo(tc.resourceMap)

			assert.Equal(t, tc.expected.Name, result.Name)
			assert.Equal(t, tc.expected.ID, result.ID)
			assert.Equal(t, tc.expected.Location, result.Location)
			assert.Equal(t, tc.expected.Role, result.Role)
			assert.Equal(t, tc.expected.Status, result.Status)
			assert.Equal(t, tc.expected.CreatedAt, result.CreatedAt)
			assert.Equal(t, tc.expected.LastUsed, result.LastUsed)
		})
	}
}

// Run the test suite
func TestResourceParsingTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceParsingTestSuite))
}
