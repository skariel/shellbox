package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/google/uuid"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
	"shellbox/internal/test"
)

func TestVolumeCreationAndDeletion(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
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
			testVolumeCreationAndDeletionCase(ctx, t, env, tc)
		})
	}
}

func testVolumeCreationAndDeletionCase(ctx context.Context, t *testing.T, env *test.Environment, tc struct {
	name   string
	sizeGB int32
	useAPI bool
},
) {
	volumeID := uuid.New().String()
	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	test.LogTestProgress(t, "creating volume", "volumeID", volumeID, "name", volumeName, "sizeGB", tc.sizeGB, "useAPI", tc.useAPI)

	var finalVolumeName string
	var err error

	if tc.useAPI {
		finalVolumeName, _, err = createVolumeWithAPI(ctx, env, tc.sizeGB, namer)
	} else {
		finalVolumeName, err = createVolumeWithTags(ctx, env, volumeID, tc.sizeGB, namer)
	}

	if err != nil {
		t.Fatalf("should create volume without error: %v", err)
	}

	verifyVolumeRetrievalAndTags(ctx, t, env, finalVolumeName, volumeID, tc.sizeGB, tc.useAPI)
	verifyVolumeDeletion(ctx, t, env, finalVolumeName, tc.sizeGB)
}

func createVolumeWithAPI(ctx context.Context, env *test.Environment, sizeGB int32, namer *infra.ResourceNamer) (string, string, error) {
	config := &infra.VolumeConfig{DiskSize: sizeGB}
	returnedVolumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	if err != nil {
		return "", "", err
	}
	if returnedVolumeID == "" {
		return "", "", fmt.Errorf("volume ID should be returned")
	}
	volumeName := namer.VolumePoolDiskName(returnedVolumeID)
	return volumeName, returnedVolumeID, nil
}

func createVolumeWithTags(ctx context.Context, env *test.Environment, volumeID string, sizeGB int32, namer *infra.ResourceNamer) (string, error) {
	volumeName := namer.VolumePoolDiskName(volumeID)
	tags := infra.VolumeTags{
		Role:     infra.ResourceRoleVolume,
		Status:   infra.ResourceStatusFree,
		VolumeID: volumeID,
	}
	volumeInfo, err := infra.CreateVolumeWithTags(ctx, env.Clients, env.ResourceGroupName, volumeName, sizeGB, tags)
	if err != nil {
		return "", err
	}

	if err := validateVolumeInfo(volumeInfo, volumeName, volumeID, sizeGB); err != nil {
		return "", err
	}
	return volumeName, nil
}

func validateVolumeInfo(volumeInfo *infra.VolumeInfo, expectedName, expectedVolumeID string, expectedSizeGB int32) error {
	if volumeInfo.Name != expectedName {
		return fmt.Errorf("volume should have correct name: expected %s, got %s", expectedName, volumeInfo.Name)
	}
	if volumeInfo.SizeGB != expectedSizeGB {
		return fmt.Errorf("volume should have correct size: expected %d, got %d", expectedSizeGB, volumeInfo.SizeGB)
	}
	if volumeInfo.Location != infra.Location {
		return fmt.Errorf("volume should be in correct location: expected %s, got %s", infra.Location, volumeInfo.Location)
	}
	if volumeInfo.ResourceID == "" {
		return fmt.Errorf("volume should have resource ID")
	}
	if volumeInfo.VolumeID != expectedVolumeID {
		return fmt.Errorf("volume should have correct volume ID: expected %s, got %s", expectedVolumeID, volumeInfo.VolumeID)
	}

	return validateVolumeTags(&volumeInfo.Tags)
}

func validateVolumeTags(tags *infra.VolumeTags) error {
	if tags.Role != infra.ResourceRoleVolume {
		return fmt.Errorf("volume should have correct role tag: expected %s, got %s", infra.ResourceRoleVolume, tags.Role)
	}
	if tags.Status != infra.ResourceStatusFree {
		return fmt.Errorf("volume should have correct status tag: expected %s, got %s", infra.ResourceStatusFree, tags.Status)
	}
	if tags.CreatedAt == "" {
		return fmt.Errorf("volume should have created timestamp")
	}
	if tags.LastUsed == "" {
		return fmt.Errorf("volume should have last used timestamp")
	}
	return nil
}

func verifyVolumeRetrievalAndTags(ctx context.Context, t *testing.T, env *test.Environment, volumeName, volumeID string, sizeGB int32, useAPI bool) {
	test.LogTestProgress(t, "verifying volume can be retrieved")

	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("should be able to retrieve created volume: %v", err)
	}
	if *disk.Name != volumeName {
		t.Errorf("retrieved volume should have correct name: expected %s, got %s", volumeName, *disk.Name)
	}
	if *disk.Properties.DiskSizeGB != sizeGB {
		t.Errorf("retrieved volume should have correct size: expected %d, got %d", sizeGB, *disk.Properties.DiskSizeGB)
	}

	if disk.Tags == nil {
		t.Fatalf("volume should have tags")
	}
	if *disk.Tags[infra.TagKeyRole] != infra.ResourceRoleVolume {
		t.Errorf("volume should have correct role tag: expected %s, got %s", infra.ResourceRoleVolume, *disk.Tags[infra.TagKeyRole])
	}
	if !useAPI {
		verifySpecificTags(t, disk.Tags, volumeID)
	}
}

func verifySpecificTags(t *testing.T, tags map[string]*string, volumeID string) {
	if *tags[infra.TagKeyStatus] != infra.ResourceStatusFree {
		t.Errorf("volume should have correct status tag: expected %s, got %s", infra.ResourceStatusFree, *tags[infra.TagKeyStatus])
	}
	if *tags[infra.TagKeyVolumeID] != volumeID {
		t.Errorf("volume should have correct volume ID tag: expected %s, got %s", volumeID, *tags[infra.TagKeyVolumeID])
	}
}

func verifyVolumeDeletion(ctx context.Context, t *testing.T, env *test.Environment, volumeName string, sizeGB int32) {
	test.LogTestProgress(t, "testing volume deletion")

	err := infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
	if err != nil {
		t.Fatalf("should delete volume without error: %v", err)
	}

	_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err == nil {
		t.Errorf("should not be able to retrieve deleted volume")
	}

	test.LogTestProgress(t, "volume test completed", "size", sizeGB)
}

type testVolumeSpec struct {
	volumeID string
	role     string
	name     string
}

func createTestVolume(ctx context.Context, env *test.Environment, volumeSpec *testVolumeSpec) error {
	namer := env.GetResourceNamer()

	if volumeSpec.role == infra.ResourceRoleVolume {
		config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
		volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
		if err != nil {
			return err
		}
		volumeSpec.volumeID = volumeID
		volumeSpec.name = namer.VolumePoolDiskName(volumeID)
		return nil
	}

	volumeID := uuid.New().String()
	volumeName := namer.VolumePoolDiskName(volumeID)
	tags := infra.VolumeTags{
		Role:      volumeSpec.role,
		Status:    infra.ResourceStatusFree,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		LastUsed:  time.Now().UTC().Format(time.RFC3339),
		VolumeID:  volumeID,
	}

	_, err := infra.CreateVolumeWithTags(ctx, env.Clients, env.ResourceGroupName, volumeName, infra.DefaultVolumeSizeGB, tags)
	if err != nil {
		return err
	}

	volumeSpec.volumeID = volumeID
	volumeSpec.name = volumeName
	return nil
}

func verifyVolumesByRole(ctx context.Context, t *testing.T, env *test.Environment, role string, expectedCount int, expectedNames []string) {
	foundNames, err := infra.FindVolumesByRole(ctx, env.Clients, env.ResourceGroupName, role, env.Suffix)
	if err != nil {
		t.Fatalf("should find volumes by role %s without error: %v", role, err)
	}

	if len(foundNames) != expectedCount {
		t.Errorf("should find exactly %d volumes with %s role, got %d", expectedCount, role, len(foundNames))
	}

	for _, expectedName := range expectedNames {
		found := false
		for _, volumeName := range foundNames {
			if volumeName == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("should find volume %s", expectedName)
		}
	}
}

func TestFindVolumesByRole(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "creating multiple volumes with different roles")

	volumes := []testVolumeSpec{
		{role: infra.ResourceRoleVolume},
		{role: infra.ResourceRoleVolume},
		{role: "temp"},
	}

	for i := range volumes {
		if err := createTestVolume(ctx, env, &volumes[i]); err != nil {
			t.Fatalf("should create volume without error: %v", err)
		}
	}

	test.LogTestProgress(t, "finding volumes by role")

	verifyVolumesByRole(ctx, t, env, infra.ResourceRoleVolume, 2, []string{volumes[0].name, volumes[1].name})
	verifyVolumesByRole(ctx, t, env, "temp", 1, []string{volumes[2].name})
	verifyVolumesByRole(ctx, t, env, "nonexistent", 0, []string{})

	test.LogTestProgress(t, "cleaning up test volumes")

	for _, vol := range volumes {
		if err := infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, vol.name); err != nil {
			t.Errorf("should delete volume %s without error: %v", vol.name, err)
		}
	}
}

func TestUpdateVolumeStatus(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "creating volume for status update test")

	// Create initial volume
	config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	if err != nil {
		t.Fatalf("should create volume without error: %v", err)
	}

	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Verify initial status
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("should retrieve volume: %v", err)
	}
	if *disk.Tags[infra.TagKeyStatus] != infra.ResourceStatusFree {
		t.Errorf("volume should initially be free: expected %s, got %s", infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus])
	}

	test.LogTestProgress(t, "updating volume status to attached")

	// Update status to attached
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusAttached)
	if err != nil {
		t.Fatalf("should update volume status without error: %v", err)
	}

	// Verify status was updated
	disk, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("should retrieve volume after status update: %v", err)
	}
	if *disk.Tags[infra.TagKeyStatus] != infra.ResourceStatusAttached {
		t.Errorf("volume status should be updated to attached: expected %s, got %s", infra.ResourceStatusAttached, *disk.Tags[infra.TagKeyStatus])
	}

	// Verify last used timestamp was updated
	if *disk.Tags[infra.TagKeyLastUsed] == "" {
		t.Errorf("last used timestamp should be updated")
	}

	test.LogTestProgress(t, "updating volume status back to free")

	// Update status back to free
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusFree)
	if err != nil {
		t.Fatalf("should update volume status back to free without error: %v", err)
	}

	// Verify final status
	disk, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("should retrieve volume after final status update: %v", err)
	}
	if *disk.Tags[infra.TagKeyStatus] != infra.ResourceStatusFree {
		t.Errorf("volume status should be back to free: expected %s, got %s", infra.ResourceStatusFree, *disk.Tags[infra.TagKeyStatus])
	}

	// Clean up
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
	if err != nil {
		t.Fatalf("should delete volume without error: %v", err)
	}
}

func setupNetworkForTesting(ctx context.Context, t *testing.T, env *test.Environment) {
	test.LogTestProgress(t, "setting up network infrastructure for instance creation")
	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)
}

func createTestVolumeForAttachment(ctx context.Context, t *testing.T, env *test.Environment) string {
	test.LogTestProgress(t, "creating volume for attachment test")
	config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	if err != nil {
		t.Fatalf("should create volume without error: %v", err)
	}
	test.LogTestProgress(t, "creating volume for attachment test", "volumeID", volumeID)
	return volumeID
}

func createTestInstanceForAttachment(ctx context.Context, t *testing.T, env *test.Environment) string {
	_, sshPublicKey, err := sshutil.LoadKeyPair("/home/ubuntu/.ssh/id_ed25519")
	if err != nil {
		t.Fatalf("should load SSH key: %v", err)
	}

	vmConfig := &infra.VMConfig{
		AdminUsername: infra.AdminUsername,
		SSHPublicKey:  sshPublicKey,
		VMSize:        "Standard_B2s",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
	if err != nil {
		t.Fatalf("should create instance without error: %v", err)
	}
	test.LogTestProgress(t, "creating instance for attachment test", "instanceID", instanceID)
	return instanceID
}

func waitForInstanceProvisioning(ctx context.Context, t *testing.T, env *test.Environment, vmName string) {
	test.LogTestProgress(t, "waiting for instance to be fully provisioned")
	err := env.WaitForResource(ctx, vmName, func() (bool, error) {
		vm, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
		if err != nil {
			return false, err
		}
		return vm.Properties.ProvisioningState != nil && *vm.Properties.ProvisioningState == "Succeeded", nil
	})
	if err != nil {
		t.Fatalf("instance should be fully provisioned: %v", err)
	}
}

func verifyVolumeAttachment(ctx context.Context, t *testing.T, env *test.Environment, vmName, volumeName string) {
	test.LogTestProgress(t, "verifying volume attachment")

	vm, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
	if err != nil {
		t.Fatalf("should retrieve VM after attachment: %v", err)
	}
	if vm.Properties.StorageProfile == nil {
		t.Fatal("VM should have storage profile")
	}
	if vm.Properties.StorageProfile.DataDisks == nil {
		t.Fatal("VM should have data disks")
	}
	if len(vm.Properties.StorageProfile.DataDisks) != 1 {
		t.Errorf("VM should have exactly one data disk attached, got %d", len(vm.Properties.StorageProfile.DataDisks))
	}

	dataDisk := vm.Properties.StorageProfile.DataDisks[0]
	if *dataDisk.Name != volumeName {
		t.Errorf("attached disk should have correct name, expected %q, got %q", volumeName, *dataDisk.Name)
	}
	if *dataDisk.CreateOption != armcompute.DiskCreateOptionTypesAttach {
		t.Errorf("disk should be attached type, expected %v, got %v", armcompute.DiskCreateOptionTypesAttach, *dataDisk.CreateOption)
	}
	if *dataDisk.Lun != 0 {
		t.Errorf("disk should be attached at LUN 0, got %d", *dataDisk.Lun)
	}

	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("should still be able to retrieve attached volume: %v", err)
	}
	if *disk.Name != volumeName {
		t.Errorf("attached volume should have correct name, expected %q, got %q", volumeName, *disk.Name)
	}
}

func cleanupInstanceAndVolume(ctx context.Context, t *testing.T, env *test.Environment, vmName, volumeName string) {
	test.LogTestProgress(t, "cleaning up instance and volume")

	if err := infra.DeleteInstance(ctx, env.Clients, env.ResourceGroupName, vmName); err != nil {
		t.Fatalf("should delete instance without error: %v", err)
	}

	if err := infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName); err != nil {
		t.Fatalf("should delete volume without error: %v", err)
	}
}

func TestVolumeAttachmentToInstance(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	setupNetworkForTesting(ctx, t, env)
	volumeID := createTestVolumeForAttachment(ctx, t, env)
	instanceID := createTestInstanceForAttachment(ctx, t, env)

	namer := env.GetResourceNamer()
	vmName := namer.BoxVMName(instanceID)
	volumeName := namer.VolumePoolDiskName(volumeID)

	waitForInstanceProvisioning(ctx, t, env, vmName)

	test.LogTestProgress(t, "attaching volume to instance")
	if err := infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeID); err != nil {
		t.Fatalf("should attach volume to instance without error: %v", err)
	}

	verifyVolumeAttachment(ctx, t, env, vmName, volumeName)
	cleanupInstanceAndVolume(ctx, t, env, vmName, volumeName)
}

func TestVolumeLifecycle(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
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
	if err != nil {
		t.Fatalf("step 1: should create volume: %v", err)
	}
	if volumeInfo.Tags.Status != infra.ResourceStatusFree {
		t.Errorf("volume should start as free, got %q", volumeInfo.Tags.Status)
	}

	// 2. Update to attached status
	test.LogTestProgress(t, "step 2: updating to attached status")
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusAttached)
	if err != nil {
		t.Fatalf("step 2: should update to attached: %v", err)
	}

	// 3. Verify status change
	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("step 3: should retrieve volume: %v", err)
	}
	if *disk.Tags[infra.TagKeyStatus] != infra.ResourceStatusAttached {
		t.Errorf("step 3: volume should be attached, got %q", *disk.Tags[infra.TagKeyStatus])
	}

	// 4. Find volume by role
	test.LogTestProgress(t, "step 4: finding volume by role")
	volumeNames, err := infra.FindVolumesByRole(ctx, env.Clients, env.ResourceGroupName, infra.ResourceRoleVolume, env.Suffix)
	if err != nil {
		t.Fatalf("step 4: should find volumes by role: %v", err)
	}
	found := false
	for _, name := range volumeNames {
		if name == volumeName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("step 4: should find our volume %q in list %v", volumeName, volumeNames)
	}

	// 5. Update back to free
	test.LogTestProgress(t, "step 5: updating back to free status")
	err = infra.UpdateVolumeStatus(ctx, env.Clients, volumeID, infra.ResourceStatusFree)
	if err != nil {
		t.Fatalf("step 5: should update back to free: %v", err)
	}

	// 6. Verify final status
	disk, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("step 6: should retrieve volume: %v", err)
	}
	if *disk.Tags[infra.TagKeyStatus] != infra.ResourceStatusFree {
		t.Errorf("step 6: volume should be free, got %q", *disk.Tags[infra.TagKeyStatus])
	}

	// 7. Delete volume
	test.LogTestProgress(t, "step 7: deleting volume")
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, volumeName)
	if err != nil {
		t.Fatalf("step 7: should delete volume: %v", err)
	}

	// 8. Verify deletion
	_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err == nil {
		t.Error("step 8: should not be able to retrieve deleted volume")
	}

	test.LogTestProgress(t, "volume lifecycle test completed successfully")
}

func TestVolumeErrorHandling(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing volume error handling scenarios")

	// Test 1: Delete non-existent volume (should not error)
	err := infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, "non-existent-volume")
	if err != nil {
		t.Errorf("deleting non-existent volume should not error: %v", err)
	}

	// Test 2: Delete volume with empty name (should be handled gracefully)
	err = infra.DeleteVolume(ctx, env.Clients, env.ResourceGroupName, "")
	if err != nil {
		t.Errorf("deleting volume with empty name should not error: %v", err)
	}

	// Test 3: Update status of non-existent volume (should error)
	err = infra.UpdateVolumeStatus(ctx, env.Clients, "non-existent-volume-id", infra.ResourceStatusAttached)
	if err == nil {
		t.Error("updating status of non-existent volume should error")
	}

	// Test 4: Find volumes in non-existent resource group (should handle gracefully)
	volumes, err := infra.FindVolumesByRole(ctx, env.Clients, "non-existent-rg", infra.ResourceRoleVolume)
	if err == nil {
		t.Error("finding volumes in non-existent resource group should error")
	}
	if volumes != nil {
		t.Errorf("should return nil volumes for non-existent resource group, got %v", volumes)
	}

	test.LogTestProgress(t, "error handling tests completed")
}
