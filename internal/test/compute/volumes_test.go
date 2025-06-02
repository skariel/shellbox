//go:build compute

package compute

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestCreateVolumeWithConfig(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Test volume creation with VolumeConfig
	config := &infra.VolumeConfig{
		DiskSize: 64,
	}

	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	require.NoError(t, err, "should create volume without error")
	require.NotEmpty(t, volumeID, "should return valid volume ID")

	// Verify volume was created by retrieving it
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	volumeName := namer.VolumePoolDiskName(volumeID)

	disk, err := env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should be able to retrieve created volume")

	// Verify volume properties
	assert.Equal(t, volumeName, *disk.Name, "volume should have correct name")
	assert.Equal(t, int32(64), *disk.Properties.DiskSizeGB, "volume should have correct size")
	assert.Equal(t, infra.Location, *disk.Location, "volume should be in correct location")

	// Verify tags are correctly set
	require.NotNil(t, disk.Tags, "volume should have tags")
	assert.Equal(t, infra.ResourceRoleVolume, *disk.Tags[infra.TagKeyRole], "volume should have correct role tag")
	assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "volume should have correct status tag")
	assert.Equal(t, volumeID, *disk.Tags["volume_id"], "volume should have correct volume ID tag")
}

func TestCreateVolumeWithDifferentSizes(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	testCases := []struct {
		name     string
		diskSize int32
	}{
		{"small volume", 32},
		{"medium volume", 128},
		{"large volume", 512},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &infra.VolumeConfig{
				DiskSize: tc.diskSize,
			}

			volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
			require.NoError(t, err, "should create volume")

			// Verify size
			namer := infra.NewResourceNamer(env.Clients.Suffix)
			volumeName := namer.VolumePoolDiskName(volumeID)

			disk, err := env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
			require.NoError(t, err, "should retrieve volume")
			assert.Equal(t, tc.diskSize, *disk.Properties.DiskSizeGB, "volume should have correct size")
		})
	}
}

func TestVolumeAttachmentToInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create instance
	vmConfig := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
	require.NoError(t, err, "should create instance")

	// Create volume
	volumeConfig := &infra.VolumeConfig{
		DiskSize: 64,
	}

	volumeID, err := infra.CreateVolume(ctx, env.Clients, volumeConfig)
	require.NoError(t, err, "should create volume")

	// Test attachment
	err = infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeID)
	require.NoError(t, err, "should attach volume to instance")

	// Verify attachment
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)
	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after attachment")

	require.NotNil(t, vm.Properties.StorageProfile.DataDisks, "VM should have data disks")
	assert.Len(t, vm.Properties.StorageProfile.DataDisks, 1, "VM should have one data disk")

	dataDisk := vm.Properties.StorageProfile.DataDisks[0]
	volumeName := namer.VolumePoolDiskName(volumeID)
	assert.Equal(t, volumeName, *dataDisk.Name, "attached disk should have correct name")
	assert.Equal(t, int32(0), *dataDisk.Lun, "attached disk should have LUN 0")

	// Test volume status update
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusAttached)
	require.NoError(t, err, "should update volume status")

	// Verify status update
	disk, err := env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should retrieve volume after status update")
	assert.Equal(t, infra.ResourceStatusAttached, *disk.Tags[infra.TagKeyStatus], "volume should have updated status")
}

func TestVolumeDetachmentFromInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create and set up instance with attached volume
	vmConfig := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
	require.NoError(t, err, "should create instance")

	volumeConfig := &infra.VolumeConfig{
		DiskSize: 64,
	}

	volumeID, err := infra.CreateVolume(ctx, env.Clients, volumeConfig)
	require.NoError(t, err, "should create volume")

	err = infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeID)
	require.NoError(t, err, "should attach volume")

	// Now test detachment by modifying VM to remove data disk
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)

	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM")

	// Remove data disks
	vm.Properties.StorageProfile.DataDisks = nil

	// Update VM to detach volume
	poller, err := env.Clients.ComputeClient.BeginCreateOrUpdate(ctx, env.Clients.ResourceGroupName, vmName, vm.VirtualMachine, nil)
	require.NoError(t, err, "should start VM update for detachment")

	_, err = poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	require.NoError(t, err, "should complete volume detachment")

	// Verify detachment
	vm, err = env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after detachment")
	assert.Len(t, vm.Properties.StorageProfile.DataDisks, 0, "VM should have no data disks after detachment")

	// Update volume status back to free
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusFree)
	require.NoError(t, err, "should update volume status to free")
}

func TestMultipleVolumeAttachment(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create instance
	vmConfig := &infra.VMConfig{
		VMSize:        "Standard_D2s_v3", // Larger size to support multiple disks
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
	require.NoError(t, err, "should create instance")

	// Create multiple volumes
	volumeIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		volumeConfig := &infra.VolumeConfig{
			DiskSize: 32,
		}

		volumeID, err := infra.CreateVolume(ctx, env.Clients, volumeConfig)
		require.NoError(t, err, "should create volume %d", i)
		volumeIDs[i] = volumeID
	}

	// Attach first volume (the function only supports one volume currently)
	err = infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeIDs[0])
	require.NoError(t, err, "should attach first volume")

	// Verify only one volume is attached
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	vmName := namer.BoxVMName(instanceID)
	vm, err := env.Clients.ComputeClient.Get(ctx, env.Clients.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM")

	require.NotNil(t, vm.Properties.StorageProfile.DataDisks, "VM should have data disks")
	assert.Len(t, vm.Properties.StorageProfile.DataDisks, 1, "VM should have one data disk attached")

	// Additional volumes should still exist but unattached
	for i := 1; i < 3; i++ {
		volumeName := namer.VolumePoolDiskName(volumeIDs[i])
		disk, err := env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
		require.NoError(t, err, "volume %d should exist", i)
		assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "unattached volume should be free")
	}
}

func TestVolumeLifecycle(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create volume
	config := &infra.VolumeConfig{
		DiskSize: 64,
	}

	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	require.NoError(t, err, "should create volume")

	namer := infra.NewResourceNamer(env.Clients.Suffix)
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Verify volume exists with correct initial state
	disk, err := env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "volume should exist")
	assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "volume should start as free")

	// Update status
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusAttached)
	require.NoError(t, err, "should update volume status")

	// Verify status update
	disk, err = env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should retrieve volume after status update")
	assert.Equal(t, infra.ResourceStatusAttached, *disk.Tags[infra.TagKeyStatus], "volume should have updated status")

	// Update status back to free
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusFree)
	require.NoError(t, err, "should update volume status back to free")

	// Delete volume
	err = infra.DeleteVolume(ctx, env.Clients, env.Clients.ResourceGroupName, volumeName)
	require.NoError(t, err, "should delete volume")

	// Verify volume is deleted
	_, err = env.Clients.DisksClient.Get(ctx, env.Clients.ResourceGroupName, volumeName, nil)
	assert.Error(t, err, "volume should not exist after deletion")
}

func TestFindVolumesByRole(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create volumes with different roles
	volumeConfig := &infra.VolumeConfig{
		DiskSize: 32,
	}

	// Create regular volume (role: volume)
	_, err := infra.CreateVolume(ctx, env.Clients, volumeConfig)
	require.NoError(t, err, "should create first volume")

	// Create temp volume manually
	volumeID2 := uuid.New().String()
	namer := infra.NewResourceNamer(env.Clients.Suffix)
	volumeName2 := namer.VolumePoolDiskName(volumeID2)

	now := time.Now().UTC()
	tempTags := infra.VolumeTags{
		Role:      infra.ResourceRoleTemp,
		Status:    infra.ResourceStatusFree,
		CreatedAt: now.Format(time.RFC3339),
		LastUsed:  now.Format(time.RFC3339),
		VolumeID:  volumeID2,
	}

	_, err = infra.CreateVolumeWithTags(ctx, env.Clients, env.Clients.ResourceGroupName, volumeName2, 32, tempTags)
	require.NoError(t, err, "should create temp volume")

	// Test finding volumes by role (filter by test suffix for isolation)
	volumeRoleVolumes, err := infra.FindVolumesByRole(ctx, env.Clients, env.Clients.ResourceGroupName, infra.ResourceRoleVolume, env.Clients.Suffix)
	require.NoError(t, err, "should find volume role volumes")
	assert.Len(t, volumeRoleVolumes, 1, "should find one volume with volume role")

	tempRoleVolumes, err := infra.FindVolumesByRole(ctx, env.Clients, env.Clients.ResourceGroupName, infra.ResourceRoleTemp, env.Clients.Suffix)
	require.NoError(t, err, "should find temp role volumes")
	assert.Len(t, tempRoleVolumes, 1, "should find one volume with temp role")

	// Test finding non-existent role (filter by test suffix for isolation)
	nonExistentRoleVolumes, err := infra.FindVolumesByRole(ctx, env.Clients, env.Clients.ResourceGroupName, "nonexistent", env.Clients.Suffix)
	require.NoError(t, err, "should complete search for non-existent role")
	assert.Len(t, nonExistentRoleVolumes, 0, "should find no volumes with non-existent role")
}

func TestVolumeErrorHandling(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tests := []struct {
		name   string
		config *infra.VolumeConfig
	}{
		{
			name:   "zero disk size",
			config: &infra.VolumeConfig{DiskSize: 0},
		},
		{
			name:   "negative disk size",
			config: &infra.VolumeConfig{DiskSize: -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := infra.CreateVolume(ctx, env.Clients, tt.config)
			assert.Error(t, err, "should fail with invalid disk size")
		})
	}
}

func TestVolumeCreationPerformance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	config := &infra.VolumeConfig{
		DiskSize: 32,
	}

	start := time.Now()

	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	require.NoError(t, err, "should create volume")
	require.NotEmpty(t, volumeID, "should return valid volume ID")

	duration := time.Since(start)

	// Volume creation should complete within 5 minutes
	assert.Less(t, duration, 5*time.Minute, "volume creation should complete within 5 minutes")

	t.Logf("Volume creation took %v", duration)
}
