package integration

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
	"shellbox/internal/test"
)

func TestVolumeCreationAndDeletion(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	testCases := []struct {
		name   string
		sizeGB int32
		useAPI bool // true for CreateVolume API, false for CreateVolumeWithTags
	}{
		{"DefaultSize_API", infra.DefaultVolumeSizeGB, true},
		{"DefaultSize_WithTags", infra.DefaultVolumeSizeGB, false},
		{"CustomSize_250GB", 250, true},
		{"CustomSize_500GB", 500, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			volumeID := uuid.New().String()
			namer := env.GetResourceNamer()
			volumeName := namer.VolumePoolDiskName(volumeID)

			test.LogTestProgress(t, "creating volume", "volumeID", volumeID, "name", volumeName, "sizeGB", tc.sizeGB, "useAPI", tc.useAPI)

			var err error
			var returnedVolumeID string

			if tc.useAPI {
				// Use CreateVolume API
				config := &infra.VolumeConfig{DiskSize: tc.sizeGB}
				returnedVolumeID, err = infra.CreateVolume(ctx, env.Clients, config)
				require.NoError(t, err, "should create volume without error")
				assert.NotEmpty(t, returnedVolumeID, "volume ID should be returned")
				// Update volume name with returned ID
				volumeName = namer.VolumePoolDiskName(returnedVolumeID)
			} else {
				// Use CreateVolumeWithTags API
				tags := infra.VolumeTags{
					Role:     infra.ResourceRoleVolume,
					Status:   infra.ResourceStatusFree,
					VolumeID: volumeID,
				}
				volumeInfo, err := infra.CreateVolumeWithTags(ctx, env.Clients, env.ResourceGroupName, volumeName, tc.sizeGB, tags)
				require.NoError(t, err, "should create volume without error")

				// Verify volume properties from returned info
				assert.Equal(t, volumeName, volumeInfo.Name, "volume should have correct name")
				assert.Equal(t, tc.sizeGB, volumeInfo.SizeGB, "volume should have correct size")
				assert.Equal(t, infra.Location, volumeInfo.Location, "volume should be in correct location")
				assert.NotEmpty(t, volumeInfo.ResourceID, "volume should have resource ID")
				assert.Equal(t, volumeID, volumeInfo.VolumeID, "volume should have correct volume ID")

				// Verify volume tags from returned info
				assert.Equal(t, infra.ResourceRoleVolume, volumeInfo.Tags.Role, "volume should have correct role tag")
				assert.Equal(t, infra.ResourceStatusFree, volumeInfo.Tags.Status, "volume should have correct status tag")
				assert.NotEmpty(t, volumeInfo.Tags.CreatedAt, "volume should have created timestamp")
				assert.NotEmpty(t, volumeInfo.Tags.LastUsed, "volume should have last used timestamp")
			}

			test.LogTestProgress(t, "verifying volume can be retrieved")

			// Verify volume can be retrieved directly
			disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
			require.NoError(t, err, "should be able to retrieve created volume")
			assert.Equal(t, volumeName, *disk.Name, "retrieved volume should have correct name")
			assert.Equal(t, tc.sizeGB, *disk.Properties.DiskSizeGB, "retrieved volume should have correct size")

			// Verify tags are correctly set
			require.NotNil(t, disk.Tags, "volume should have tags")
			assert.Equal(t, infra.ResourceRoleVolume, *disk.Tags[infra.TagKeyRole], "volume should have correct role tag")
			if !tc.useAPI {
				// Only volumes created with CreateVolumeWithTags have these specific tags
				assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "volume should have correct status tag")
				assert.Equal(t, volumeID, *disk.Tags["volume_id"], "volume should have correct volume ID tag")
			}

			test.LogTestProgress(t, "testing volume deletion")

			// Test volume deletion
			err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
			require.NoError(t, err, "should delete volume without error")

			// Verify volume is deleted
			_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
			assert.Error(t, err, "should not be able to retrieve deleted volume")

			test.LogTestProgress(t, "volume test completed", "size", tc.sizeGB)
		})
	}
}

func TestFindVolumesByRole(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()

	test.LogTestProgress(t, "creating multiple volumes with different roles")

	// Create volumes with different roles
	volumes := []struct {
		volumeID string
		role     string
		name     string
	}{
		{"", infra.ResourceRoleVolume, ""},
		{"", infra.ResourceRoleVolume, ""},
		{"", "temp", ""},
	}

	// Create all volumes and capture their IDs
	for i := range volumes {
		if volumes[i].role == infra.ResourceRoleVolume {
			// Use CreateVolume for standard volumes
			config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
			volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
			require.NoError(t, err, "should create volume without error")

			volumes[i].volumeID = volumeID
			volumes[i].name = namer.VolumePoolDiskName(volumeID)
		} else {
			// Use CreateVolumeWithTags for custom roles
			volumeID := uuid.New().String()
			volumeName := namer.VolumePoolDiskName(volumeID)
			tags := infra.VolumeTags{
				Role:      volumes[i].role,
				Status:    infra.ResourceStatusFree,
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
				LastUsed:  time.Now().UTC().Format(time.RFC3339),
				VolumeID:  volumeID,
			}

			_, err := infra.CreateVolumeWithTags(ctx, env.Clients, env.ResourceGroupName, volumeName, infra.DefaultVolumeSizeGB, tags)
			require.NoError(t, err, "should create volume with custom role without error")

			volumes[i].volumeID = volumeID
			volumes[i].name = volumeName
		}
	}

	test.LogTestProgress(t, "finding volumes by role")

	// Find volumes by role (filter by test suffix for isolation)
	volumeRoleNames, err := infra.FindVolumesByRole(ctx, env.Clients, env.ResourceGroupName, infra.ResourceRoleVolume, env.Suffix)
	require.NoError(t, err, "should find volumes by role without error")

	// Should find exactly 2 volumes with "volume" role
	assert.Len(t, volumeRoleNames, 2, "should find exactly 2 volumes with volume role")

	// Verify the correct volumes are found
	expectedNames := []string{volumes[0].name, volumes[1].name}
	for _, expectedName := range expectedNames {
		assert.Contains(t, volumeRoleNames, expectedName, "should find volume %s", expectedName)
	}

	// Find temp volumes (filter by test suffix for isolation)
	tempVolumeNames, err := infra.FindVolumesByRole(ctx, env.Clients, env.ResourceGroupName, "temp", env.Suffix)
	require.NoError(t, err, "should find temp volumes without error")
	assert.Len(t, tempVolumeNames, 1, "should find exactly 1 temp volume")
	assert.Contains(t, tempVolumeNames, volumes[2].name, "should find temp volume")

	// Find non-existent role (filter by test suffix for isolation)
	nonExistentNames, err := infra.FindVolumesByRole(ctx, env.Clients, env.ResourceGroupName, "nonexistent", env.Suffix)
	require.NoError(t, err, "should handle non-existent role without error")
	assert.Len(t, nonExistentNames, 0, "should find no volumes with non-existent role")

	test.LogTestProgress(t, "cleaning up test volumes")

	// Clean up all volumes
	for _, vol := range volumes {
		err := infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, vol.name)
		assert.NoError(t, err, "should delete volume %s without error", vol.name)
	}
}

func TestUpdateVolumeStatus(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "creating volume for status update test")

	// Create initial volume
	config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	require.NoError(t, err, "should create volume without error")

	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Verify initial status
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should retrieve volume")
	assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "volume should initially be free")

	test.LogTestProgress(t, "updating volume status to attached")

	// Update status to attached
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusAttached)
	require.NoError(t, err, "should update volume status without error")

	// Verify status was updated
	disk, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should retrieve volume after status update")
	assert.Equal(t, infra.ResourceStatusAttached, *disk.Tags[infra.TagKeyStatus], "volume status should be updated to attached")

	// Verify last used timestamp was updated
	assert.NotEmpty(t, *disk.Tags[infra.TagKeyLastUsed], "last used timestamp should be updated")

	test.LogTestProgress(t, "updating volume status back to free")

	// Update status back to free
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusFree)
	require.NoError(t, err, "should update volume status back to free without error")

	// Verify final status
	disk, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should retrieve volume after final status update")
	assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "volume status should be back to free")

	// Clean up
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
	require.NoError(t, err, "should delete volume without error")
}

func TestVolumeAttachmentToInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "setting up network infrastructure for instance creation")

	// First create network infrastructure (required for VM creation)
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	test.LogTestProgress(t, "creating volume for attachment test")

	// Create volume
	config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	require.NoError(t, err, "should create volume without error")

	test.LogTestProgress(t, "creating volume for attachment test", "volumeID", volumeID)

	// Create instance (VM) for attachment
	// Load SSH public key using the same function as production
	_, sshPublicKey, err := sshutil.LoadKeyPair("/home/ubuntu/.ssh/id_ed25519")
	require.NoError(t, err, "should load SSH key")

	vmConfig := &infra.VMConfig{
		AdminUsername: infra.AdminUsername,
		SSHPublicKey:  sshPublicKey,
		VMSize:        "Standard_B2s", // Smaller size for faster testing
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
	require.NoError(t, err, "should create instance without error")

	test.LogTestProgress(t, "creating instance for attachment test", "instanceID", instanceID)

	// Get resource names using the actual instance ID
	namer := env.GetResourceNamer()
	vmName := namer.BoxVMName(instanceID)
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Wait for VM to be fully provisioned
	test.LogTestProgress(t, "waiting for instance to be fully provisioned")
	err = env.WaitForResource(ctx, vmName, func() (bool, error) {
		vm, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
		if err != nil {
			return false, err
		}
		return vm.Properties.ProvisioningState != nil && *vm.Properties.ProvisioningState == "Succeeded", nil
	})
	require.NoError(t, err, "instance should be fully provisioned")

	test.LogTestProgress(t, "attaching volume to instance")

	// Attach volume to instance
	err = infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeID)
	require.NoError(t, err, "should attach volume to instance without error")

	test.LogTestProgress(t, "verifying volume attachment")

	// Verify attachment by checking VM configuration
	vm, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
	require.NoError(t, err, "should retrieve VM after attachment")
	require.NotNil(t, vm.Properties.StorageProfile, "VM should have storage profile")
	require.NotNil(t, vm.Properties.StorageProfile.DataDisks, "VM should have data disks")
	assert.Len(t, vm.Properties.StorageProfile.DataDisks, 1, "VM should have exactly one data disk attached")

	dataDisk := vm.Properties.StorageProfile.DataDisks[0]
	assert.Equal(t, volumeName, *dataDisk.Name, "attached disk should have correct name")
	assert.Equal(t, armcompute.DiskCreateOptionTypesAttach, *dataDisk.CreateOption, "disk should be attached type")
	assert.Equal(t, int32(0), *dataDisk.Lun, "disk should be attached at LUN 0")

	// Verify volume can still be retrieved
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "should still be able to retrieve attached volume")
	assert.Equal(t, volumeName, *disk.Name, "attached volume should have correct name")

	test.LogTestProgress(t, "cleaning up instance and volume")

	// Clean up: delete instance (which will also detach the volume)
	err = infra.DeleteInstance(ctx, env.Clients, env.ResourceGroupName, vmName)
	require.NoError(t, err, "should delete instance without error")

	// Clean up volume
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
	require.NoError(t, err, "should delete volume without error")
}

func TestVolumeLifecycle(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	volumeID := uuid.New().String()
	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	test.LogTestProgress(t, "testing complete volume lifecycle", "volumeID", volumeID)

	// 1. Create volume
	tags := infra.VolumeTags{
		Role:     infra.ResourceRoleVolume,
		Status:   infra.ResourceStatusFree,
		VolumeID: volumeID,
	}

	volumeInfo, err := infra.CreateVolumeWithTags(ctx, env.Clients, env.ResourceGroupName, volumeName, infra.DefaultVolumeSizeGB, tags)
	require.NoError(t, err, "step 1: should create volume")
	assert.Equal(t, infra.ResourceStatusFree, volumeInfo.Tags.Status, "volume should start as free")

	// 2. Update to attached status
	test.LogTestProgress(t, "step 2: updating to attached status")
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusAttached)
	require.NoError(t, err, "step 2: should update to attached")

	// 3. Verify status change
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "step 3: should retrieve volume")
	assert.Equal(t, infra.ResourceStatusAttached, *disk.Tags[infra.TagKeyStatus], "step 3: volume should be attached")

	// 4. Find volume by role
	test.LogTestProgress(t, "step 4: finding volume by role")
	volumeNames, err := infra.FindVolumesByRole(ctx, env.Clients, env.ResourceGroupName, infra.ResourceRoleVolume, env.Suffix)
	require.NoError(t, err, "step 4: should find volumes by role")
	assert.Contains(t, volumeNames, volumeName, "step 4: should find our volume")

	// 5. Update back to free
	test.LogTestProgress(t, "step 5: updating back to free status")
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusFree)
	require.NoError(t, err, "step 5: should update back to free")

	// 6. Verify final status
	disk, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "step 6: should retrieve volume")
	assert.Equal(t, infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus], "step 6: volume should be free")

	// 7. Delete volume
	test.LogTestProgress(t, "step 7: deleting volume")
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
	require.NoError(t, err, "step 7: should delete volume")

	// 8. Verify deletion
	_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	assert.Error(t, err, "step 8: should not be able to retrieve deleted volume")

	test.LogTestProgress(t, "volume lifecycle test completed successfully")
}

func TestVolumeErrorHandling(t *testing.T) {
	test.RequireCategory(t, test.CategoryIntegration)
	t.Parallel()

	env := test.SetupTestEnvironment(t, test.CategoryIntegration)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing volume error handling scenarios")

	// Test 1: Delete non-existent volume (should not error)
	err := infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, "non-existent-volume")
	assert.NoError(t, err, "deleting non-existent volume should not error")

	// Test 2: Delete volume with empty name (should be handled gracefully)
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, "")
	assert.NoError(t, err, "deleting volume with empty name should not error")

	// Test 3: Update status of non-existent volume (should error)
	err = infra.UpdateVolumeStatus(ctx, env.Clients, "non-existent-volume-id", infra.ResourceStatusAttached)
	assert.Error(t, err, "updating status of non-existent volume should error")

	// Test 4: Find volumes in non-existent resource group (should handle gracefully)
	volumes, err := infra.FindVolumesByRole(ctx, env.Clients, "non-existent-rg", infra.ResourceRoleVolume)
	assert.Error(t, err, "finding volumes in non-existent resource group should error")
	assert.Nil(t, volumes, "should return nil volumes for non-existent resource group")

	test.LogTestProgress(t, "error handling tests completed")
}
