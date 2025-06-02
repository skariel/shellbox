//go:build compute

package compute

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestCreateInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Test VM configuration
	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance without error")
	require.NotEmpty(t, instanceID, "should return valid instance ID")

	// Verify instance was created by checking VM exists
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)

	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve created VM")

	// Verify VM properties
	assert.NotNil(t, vm.ID, "VM should have an ID")
	assert.Equal(t, vmName, *vm.Name, "VM should have correct name")
	assert.Equal(t, infra.Location, *vm.Location, "VM should be in correct location")

	// Verify VM tags
	require.NotNil(t, vm.Tags, "VM should have tags")
	assert.Equal(t, infra.ResourceRoleInstance, *vm.Tags[infra.TagKeyRole], "VM should have instance role")
	assert.Equal(t, infra.ResourceStatusFree, *vm.Tags[infra.TagKeyStatus], "VM should have free status")
	assert.Equal(t, instanceID, *vm.Tags["instance_id"], "VM should have correct instance ID")

	// Verify VM configuration
	assert.Equal(t, "Standard_B2s", string(*vm.Properties.HardwareProfile.VMSize), "VM should have correct size")
	assert.Equal(t, config.AdminUsername, *vm.Properties.OSProfile.AdminUsername, "VM should have correct admin username")
	assert.True(t, *vm.Properties.OSProfile.LinuxConfiguration.DisablePasswordAuthentication, "VM should disable password auth")

	// Verify networking setup
	require.NotNil(t, vm.Properties.NetworkProfile, "VM should have network profile")
	require.Len(t, vm.Properties.NetworkProfile.NetworkInterfaces, 1, "VM should have one NIC")

	nicID := *vm.Properties.NetworkProfile.NetworkInterfaces[0].ID
	assert.Contains(t, nicID, instanceID, "NIC ID should contain instance ID")
}

func TestCreateInstanceWithInvalidConfig(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tests := []struct {
		name   string
		config *infra.VMConfig
	}{
		{
			name: "missing SSH key",
			config: &infra.VMConfig{
				VMSize:        "Standard_B2s",
				AdminUsername: "shellbox",
				SSHPublicKey:  "",
			},
		},
		{
			name: "missing admin username",
			config: &infra.VMConfig{
				VMSize:       "Standard_B2s",
				SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := infra.CreateInstance(ctx, env.Clients, tt.config)
			assert.Error(t, err, "should fail with invalid config")
		})
	}
}

func TestFindInstancesByStatus(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create two instances with different statuses
	_, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create first instance")

	instanceID2, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create second instance")

	// Update second instance to connected status
	err = infra.UpdateInstanceStatus(ctx, env.Clients, instanceID2, infra.ResourceStatusConnected)
	require.NoError(t, err, "should update instance status")

	// Find free instances
	freeInstances, err := infra.FindInstancesByStatus(ctx, env.Clients, infra.ResourceStatusFree)
	require.NoError(t, err, "should find free instances")
	assert.Len(t, freeInstances, 1, "should find one free instance")

	// Find connected instances
	connectedInstances, err := infra.FindInstancesByStatus(ctx, env.Clients, infra.ResourceStatusConnected)
	require.NoError(t, err, "should find connected instances")
	assert.Len(t, connectedInstances, 1, "should find one connected instance")

	// Find instances with non-existent status
	nonExistentInstances, err := infra.FindInstancesByStatus(ctx, env.Clients, "nonexistent")
	require.NoError(t, err, "should complete search for non-existent status")
	assert.Len(t, nonExistentInstances, 0, "should find no instances with non-existent status")
}

func TestUpdateInstanceStatus(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	// Update status to connected
	err = infra.UpdateInstanceStatus(ctx, env.Clients, instanceID, infra.ResourceStatusConnected)
	require.NoError(t, err, "should update status to connected")

	// Verify status was updated
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)
	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after status update")

	assert.Equal(t, infra.ResourceStatusConnected, *vm.Tags[infra.TagKeyStatus], "VM should have updated status")
	assert.NotEmpty(t, *vm.Tags[infra.TagKeyLastUsed], "VM should have updated last used timestamp")

	// Update status back to free
	err = infra.UpdateInstanceStatus(ctx, env.Clients, instanceID, infra.ResourceStatusFree)
	require.NoError(t, err, "should update status to free")

	// Verify status was updated again
	vm, err = env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after second status update")
	assert.Equal(t, infra.ResourceStatusFree, *vm.Tags[infra.TagKeyStatus], "VM should have updated status")
}

func TestGetInstancePrivateIP(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	// Get private IP
	privateIP, err := infra.GetInstancePrivateIP(ctx, env.Clients, instanceID)
	require.NoError(t, err, "should get private IP")

	// Verify IP format and range
	assert.NotEmpty(t, privateIP, "should return non-empty IP")
	assert.Regexp(t, `^10\.1\.\d+\.\d+$`, privateIP, "IP should be in boxes subnet range")
}

func TestDeallocateBox(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)

	// Verify VM is running initially
	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM")
	require.NotNil(t, vm.Properties.ProvisioningState, "VM should have provisioning state")

	// Deallocate the VM
	err = infra.DeallocateBox(ctx, env.Clients, vmName)
	require.NoError(t, err, "should deallocate VM without error")

	// Verify VM is deallocated
	vm, err = env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after deallocation")

	// Check instance view to get power state
	instanceView, err := env.Clients.ComputeClient.InstanceView(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should get instance view")

	// Find power state
	var powerState string
	for _, status := range instanceView.Statuses {
		if status.Code != nil && len(*status.Code) > 10 && (*status.Code)[:10] == "PowerState" {
			powerState = *status.Code
			break
		}
	}
	assert.Contains(t, powerState, "deallocated", "VM should be in deallocated power state")
}

func TestDeleteInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)
	nicName := namer.BoxNICName(instanceID)
	nsgName := namer.BoxNSGName(instanceID)
	osDiskName := namer.BoxOSDiskName(instanceID)

	// Verify resources exist before deletion
	_, err = env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "VM should exist before deletion")

	_, err = env.Clients.NICClient.Get(ctx, env.Clients.ResourceGroupName, nicName, nil)
	require.NoError(t, err, "NIC should exist before deletion")

	_, err = env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, nsgName, nil)
	require.NoError(t, err, "NSG should exist before deletion")

	// Delete instance
	err = infra.DeleteInstance(ctx, env.Clients, env.Clients.ResourceGroupName, vmName)
	require.NoError(t, err, "should delete instance without error")

	// Verify resources are deleted (should return not found errors)
	_, err = env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	assert.Error(t, err, "VM should not exist after deletion")

	_, err = env.Clients.NICClient.Get(ctx, env.Clients.ResourceGroupName, nicName, nil)
	assert.Error(t, err, "NIC should not exist after deletion")

	_, err = env.Clients.NSGClient.Get(ctx, env.Clients.ResourceGroupName, nsgName, nil)
	assert.Error(t, err, "NSG should not exist after deletion")

	_, err = env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, osDiskName, nil)
	assert.Error(t, err, "OS disk should not exist after deletion")
}

func TestAttachVolumeToInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	// Create a test volume
	volumeID, err := infra.CreateVolume(ctx, env.Clients, &infra.VolumeConfig{
		DiskSize: 32,
	})
	require.NoError(t, err, "should create volume")

	// Attach volume to instance
	err = infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeID)
	require.NoError(t, err, "should attach volume to instance")

	// Verify volume is attached
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)
	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after volume attachment")

	require.NotNil(t, vm.Properties.StorageProfile.DataDisks, "VM should have data disks")
	assert.Len(t, vm.Properties.StorageProfile.DataDisks, 1, "VM should have one data disk attached")

	dataDisk := vm.Properties.StorageProfile.DataDisks[0]
	volumeName := namer.VolumePoolDiskName(volumeID)
	assert.Equal(t, volumeName, *dataDisk.Name, "attached disk should have correct name")
	assert.Equal(t, int32(0), *dataDisk.Lun, "attached disk should have LUN 0")
}

func TestInstanceCreationPerformance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	start := time.Now()

	// Create instance
	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")
	require.NotEmpty(t, instanceID, "should return valid instance ID")

	duration := time.Since(start)

	// Instance creation should complete within 10 minutes
	assert.Less(t, duration, 10*time.Minute, "instance creation should complete within 10 minutes")

	t.Logf("Instance creation took %v", duration)
}
