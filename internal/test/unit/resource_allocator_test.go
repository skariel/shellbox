package unit

import (
	"testing"
	"time"

	"shellbox/internal/infra"
)

// TestAllocatedResourcesStruct tests the AllocatedResources structure
func TestAllocatedResourcesStruct(t *testing.T) {
	resources := &infra.AllocatedResources{
		InstanceID: "test-instance-123",
		VolumeID:   "test-volume-456",
		InstanceIP: "10.1.2.3",
	}

	if resources.InstanceID != "test-instance-123" {
		t.Errorf("InstanceID should be set correctly, expected test-instance-123, got %s", resources.InstanceID)
	}
	if resources.VolumeID != "test-volume-456" {
		t.Errorf("VolumeID should be set correctly, expected test-volume-456, got %s", resources.VolumeID)
	}
	if resources.InstanceIP != "10.1.2.3" {
		t.Errorf("InstanceIP should be set correctly, expected 10.1.2.3, got %s", resources.InstanceIP)
	}

	// Test empty struct
	emptyResources := &infra.AllocatedResources{}
	if emptyResources.InstanceID != "" {
		t.Errorf("Empty struct should have empty InstanceID, got %s", emptyResources.InstanceID)
	}
	if emptyResources.VolumeID != "" {
		t.Errorf("Empty struct should have empty VolumeID, got %s", emptyResources.VolumeID)
	}
	if emptyResources.InstanceIP != "" {
		t.Errorf("Empty struct should have empty InstanceIP, got %s", emptyResources.InstanceIP)
	}
}

// TestResourceInfoStruct tests the ResourceInfo structure
func TestResourceInfoStruct(t *testing.T) {
	now := time.Now()
	created := now.Add(-1 * time.Hour)

	resource := infra.ResourceInfo{
		ID:         "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
		Name:       "test-vm",
		Location:   "westus2",
		Tags:       map[string]string{"role": "instance", "status": "free"},
		LastUsed:   &now,
		CreatedAt:  &created,
		Status:     "free",
		Role:       "instance",
		ResourceID: "test-instance-123",
	}

	if !contains(resource.ID, "virtualMachines") {
		t.Errorf("ID should contain resource type 'virtualMachines', got %s", resource.ID)
	}
	if resource.Name != "test-vm" {
		t.Errorf("Name should be set correctly, expected test-vm, got %s", resource.Name)
	}
	if resource.Location != "westus2" {
		t.Errorf("Location should be set correctly, expected westus2, got %s", resource.Location)
	}
	if resource.Tags["role"] != "instance" {
		t.Errorf("Tags should contain role 'instance', got %s", resource.Tags["role"])
	}
	if resource.Tags["status"] != "free" {
		t.Errorf("Tags should contain status 'free', got %s", resource.Tags["status"])
	}
	if resource.LastUsed == nil {
		t.Error("LastUsed should be set")
	}
	if resource.CreatedAt == nil {
		t.Error("CreatedAt should be set")
	}
	if resource.LastUsed != nil && resource.CreatedAt != nil && !resource.LastUsed.After(*resource.CreatedAt) {
		t.Error("LastUsed should be after CreatedAt")
	}
	if resource.Status != "free" {
		t.Errorf("Status should be set correctly, expected free, got %s", resource.Status)
	}
	if resource.Role != "instance" {
		t.Errorf("Role should be set correctly, expected instance, got %s", resource.Role)
	}
	if resource.ResourceID != "test-instance-123" {
		t.Errorf("ResourceID should be set correctly, expected test-instance-123, got %s", resource.ResourceID)
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 || indexOfString(s, substr) >= 0)
}

// indexOfString returns the index of the first occurrence of substr in s, or -1 if not found
func indexOfString(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
