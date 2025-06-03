package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"shellbox/internal/infra"
)

func TestStructsTestSuite(t *testing.T) {
	t.Run("TestVMConfig", func(t *testing.T) {
		config := infra.VMConfig{
			VMSize:        "Standard_D8s_v3",
			AdminUsername: "shellbox",
			SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com",
		}

		assert.Equal(t, "Standard_D8s_v3", config.VMSize)
		assert.Equal(t, "shellbox", config.AdminUsername)
		assert.NotEmpty(t, config.SSHPublicKey)
	})

	t.Run("TestInstanceTags", func(t *testing.T) {
		tags := infra.InstanceTags{
			Role:      infra.ResourceRoleInstance,
			Status:    infra.ResourceStatusFree,
			CreatedAt: "2024-01-01T00:00:00Z",
			LastUsed:  "2024-01-01T01:00:00Z",
		}

		assert.Equal(t, infra.ResourceRoleInstance, tags.Role)
		assert.Equal(t, infra.ResourceStatusFree, tags.Status)
		assert.Equal(t, "2024-01-01T00:00:00Z", tags.CreatedAt)
		assert.Equal(t, "2024-01-01T01:00:00Z", tags.LastUsed)
	})

	t.Run("TestTableStorageResult", func(t *testing.T) {
		result := infra.TableStorageResult{
			ConnectionString: "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key==;EndpointSuffix=core.windows.net",
			Error:            nil,
		}

		assert.Nil(t, result.Error)
		assert.Contains(t, result.ConnectionString, "AccountName=test")
	})

	t.Run("TestEventLogEntity", func(t *testing.T) {
		entity := infra.EventLogEntity{
			PartitionKey: "2024-01-01",
			RowKey:       "event-123",
			EventType:    "user_connect",
			SessionID:    "session-456",
			BoxID:        "box-789",
			UserKey:      "user123",
			Details:      "User connected to instance",
			Timestamp:    time.Now(),
		}

		assert.Equal(t, "2024-01-01", entity.PartitionKey)
		assert.Equal(t, "event-123", entity.RowKey)
		assert.Equal(t, "user_connect", entity.EventType)
		assert.Equal(t, "session-456", entity.SessionID)
		assert.Equal(t, "box-789", entity.BoxID)
		assert.Equal(t, "user123", entity.UserKey)
		assert.Equal(t, "User connected to instance", entity.Details)
		assert.False(t, entity.Timestamp.IsZero())
	})

	t.Run("TestResourceRegistryEntity", func(t *testing.T) {
		entity := infra.ResourceRegistryEntity{
			PartitionKey: "instances",
			RowKey:       "instance-123",
			Status:       infra.ResourceStatusFree,
			VMName:       "test-vm",
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			Metadata:     `{"key": "value"}`,
			Timestamp:    time.Now(),
		}

		assert.Equal(t, "instances", entity.PartitionKey)
		assert.Equal(t, "instance-123", entity.RowKey)
		assert.Equal(t, infra.ResourceStatusFree, entity.Status)
		assert.Equal(t, "test-vm", entity.VMName)
		assert.False(t, entity.CreatedAt.IsZero())
		assert.False(t, entity.LastActivity.IsZero())
		assert.Equal(t, `{"key": "value"}`, entity.Metadata)
		assert.False(t, entity.Timestamp.IsZero())
	})

	t.Run("TestQEMUScriptConfig", func(t *testing.T) {
		config := infra.QEMUScriptConfig{
			SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com",
			WorkingDir:    "~",
			SSHPort:       2222,
			MountDataDisk: false,
		}

		assert.Equal(t, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC... test@example.com", config.SSHPublicKey)
		assert.Equal(t, "~", config.WorkingDir)
		assert.Equal(t, 2222, config.SSHPort)
		assert.False(t, config.MountDataDisk)
	})

	t.Run("TestResourceInfo", func(t *testing.T) {
		now := time.Now()
		info := infra.ResourceInfo{
			ID:         "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
			Name:       "test-vm",
			Location:   "westus2",
			Tags:       map[string]string{"role": "instance", "status": "free"},
			LastUsed:   &now,
			CreatedAt:  &now,
			Status:     infra.ResourceStatusFree,
			Role:       infra.ResourceRoleInstance,
			ResourceID: "vm-123",
		}

		assert.Contains(t, info.ID, "virtualMachines")
		assert.Equal(t, "test-vm", info.Name)
		assert.Equal(t, "westus2", info.Location)
		assert.Equal(t, "instance", info.Tags["role"])
		assert.Equal(t, "free", info.Tags["status"])
		assert.NotNil(t, info.LastUsed)
		assert.NotNil(t, info.CreatedAt)
		assert.Equal(t, infra.ResourceStatusFree, info.Status)
		assert.Equal(t, infra.ResourceRoleInstance, info.Role)
		assert.Equal(t, "vm-123", info.ResourceID)
	})

	t.Run("TestResourceCounts", func(t *testing.T) {
		counts := infra.ResourceCounts{
			Free:      5,
			Connected: 3,
			Attached:  2,
			Total:     10,
		}

		assert.Equal(t, 5, counts.Free)
		assert.Equal(t, 3, counts.Connected)
		assert.Equal(t, 2, counts.Attached)
		assert.Equal(t, 10, counts.Total)

		// Test that counts make sense
		assert.Equal(t, counts.Free+counts.Connected+counts.Attached, counts.Total)
	})

	t.Run("TestAllocatedResources", func(t *testing.T) {
		resources := infra.AllocatedResources{
			InstanceID: "instance-123",
			VolumeID:   "volume-456",
			InstanceIP: "10.1.0.100",
		}

		assert.Equal(t, "instance-123", resources.InstanceID)
		assert.Equal(t, "volume-456", resources.VolumeID)
		assert.Equal(t, "10.1.0.100", resources.InstanceIP)
	})

	t.Run("TestPoolConfig", func(t *testing.T) {
		config := infra.PoolConfig{
			MinFreeInstances:  2,
			MaxFreeInstances:  5,
			MaxTotalInstances: 10,
			MinFreeVolumes:    3,
			MaxFreeVolumes:    8,
			MaxTotalVolumes:   20,
			CheckInterval:     30 * time.Second,
			ScaleDownCooldown: 5 * time.Minute,
		}

		assert.Equal(t, 2, config.MinFreeInstances)
		assert.Equal(t, 5, config.MaxFreeInstances)
		assert.Equal(t, 10, config.MaxTotalInstances)
		assert.Equal(t, 3, config.MinFreeVolumes)
		assert.Equal(t, 8, config.MaxFreeVolumes)
		assert.Equal(t, 20, config.MaxTotalVolumes)
		assert.Equal(t, 30*time.Second, config.CheckInterval)
		assert.Equal(t, 5*time.Minute, config.ScaleDownCooldown)
	})

	t.Run("TestAzureClients", func(t *testing.T) {
		clients := infra.AzureClients{
			Suffix:                       "test",
			SubscriptionID:               "sub-123",
			ResourceGroupName:            "shellbox-test",
			BastionSubnetID:              "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/bastion",
			BoxesSubnetID:                "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Network/virtualNetworks/vnet/subnets/boxes",
			TableStorageConnectionString: "DefaultEndpointsProtocol=https;AccountName=test;AccountKey=key==",
		}

		assert.Equal(t, "test", clients.Suffix)
		assert.Equal(t, "sub-123", clients.SubscriptionID)
		assert.Equal(t, "shellbox-test", clients.ResourceGroupName)
		assert.Contains(t, clients.BastionSubnetID, "bastion")
		assert.Contains(t, clients.BoxesSubnetID, "boxes")
		assert.Contains(t, clients.TableStorageConnectionString, "AccountName=test")
	})
}
