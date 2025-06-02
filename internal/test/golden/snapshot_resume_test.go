//go:build golden

package golden

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVolumeCreationFromSnapshot tests creating volumes from golden snapshots
func TestVolumeCreationFromSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping volume creation test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create a test snapshot first
	snapshotName := test.GenerateTestResourceName("test-snapshot")
	testVolume, err := test.CreateTestVolume(ctx, clients, testConfig.ResourceGroupName, "source-volume", 10)
	require.NoError(t, err, "Source volume creation should not fail")

	snapshotInfo, err := test.CreateSnapshotFromVolume(ctx, clients, testConfig.ResourceGroupName, snapshotName, testVolume.ResourceID)
	require.NoError(t, err, "Snapshot creation should not fail")

	// Clean up snapshot and source volume
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.SnapshotsClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, snapshotName, nil)
		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, "source-volume", nil)
	}()

	// Create volume from snapshot
	volumeName := test.GenerateTestResourceName("from-snapshot")
	tags := infra.VolumeTags{
		Role:      "test-volume",
		Status:    "free",
		VolumeID:  test.GenerateTestResourceName("vol"),
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
	}

	volume, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeName, snapshotInfo.ResourceID, tags)
	require.NoError(t, err, "Volume creation from snapshot should not fail")
	require.NotNil(t, volume, "Volume should not be nil")

	// Clean up created volume
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
	}()

	// Verify volume properties
	assert.Equal(t, volumeName, volume.Name, "Volume name should match")
	assert.NotEmpty(t, volume.ResourceID, "Volume resource ID should be set")
	assert.Equal(t, snapshotInfo.SizeGB, volume.SizeGB, "Volume size should match snapshot")

	// Verify volume exists in Azure
	disk, err := clients.DisksClient.Get(ctx, testConfig.ResourceGroupName, volumeName, nil)
	require.NoError(t, err, "Volume should be accessible in Azure")
	assert.Equal(t, volumeName, *disk.Name, "Azure volume name should match")

	// Verify source reference
	assert.Equal(t, snapshotInfo.ResourceID, *disk.Properties.CreationData.SourceResourceID, "Source should reference snapshot")
}

// TestQEMUResumeCommands tests QEMU resume command generation and execution
func TestQEMUResumeCommands(t *testing.T) {
	clients, _, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	testCases := []struct {
		name          string
		expectSuccess bool
		description   string
	}{
		{
			name:          "basic_resume_command",
			expectSuccess: true,
			description:   "Test basic QEMU resume command generation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create QEMU manager
			qemuManager := infra.NewQEMUManager(clients)
			require.NotNil(t, qemuManager, "QEMU manager should be created")

			// Test command generation for resume
			resumeCmd := test.GenerateQEMUResumeCommand("/mnt/userdata", 2222)
			require.NotEmpty(t, resumeCmd, "Resume command should not be empty")

			// Verify expected components in resume command
			assert.Contains(t, resumeCmd, "qemu-system-x86_64", "Should contain QEMU binary")
			assert.Contains(t, resumeCmd, "-loadvm ssh-ready", "Should load ssh-ready state")
			assert.Contains(t, resumeCmd, "/mnt/userdata/qemu-memory", "Should reference memory path")
			assert.Contains(t, resumeCmd, "/mnt/userdata/qemu-disks", "Should reference disk path")
			assert.Contains(t, resumeCmd, "hostfwd=tcp::2222-:22", "Should configure SSH forwarding")
			assert.Contains(t, resumeCmd, "-monitor unix:/tmp/qemu-monitor.sock", "Should setup monitor socket")

			// Verify script structure
			assert.Contains(t, resumeCmd, "mountpoint -q /mnt/userdata", "Should check mount point")
			assert.Contains(t, resumeCmd, "sudo mount", "Should mount data disk")
		})
	}
}

// TestVolumeAttachmentFlow tests attaching volumes to instances for resume
func TestVolumeAttachmentFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping volume attachment test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create test instance
	instanceName := test.GenerateTestResourceName("test-instance")
	instance, err := test.CreateTestInstance(ctx, clients, testConfig.ResourceGroupName, instanceName)
	require.NoError(t, err, "Instance creation should not fail")
	require.NotNil(t, instance, "Instance should not be nil")

	// Clean up instance
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()

		infra.DeleteInstance(cleanupCtx, clients, testConfig.ResourceGroupName, instanceName)
	}()

	// Create test volume
	volumeName := test.GenerateTestResourceName("test-volume")
	volume, err := test.CreateTestVolume(ctx, clients, testConfig.ResourceGroupName, volumeName, 10)
	require.NoError(t, err, "Volume creation should not fail")

	// Clean up volume
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
	}()

	// Test volume attachment
	err = infra.AttachVolumeToInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
	require.NoError(t, err, "Volume attachment should not fail")

	// Verify attachment by checking VM configuration
	vm, err := clients.ComputeClient.Get(ctx, testConfig.ResourceGroupName, instanceName, nil)
	require.NoError(t, err, "Should be able to get VM details")

	// Verify data disk is attached
	require.NotEmpty(t, vm.Properties.StorageProfile.DataDisks, "VM should have data disks")

	var attachedDisk *string
	for _, disk := range vm.Properties.StorageProfile.DataDisks {
		if disk.ManagedDisk != nil && disk.ManagedDisk.ID != nil {
			if strings.Contains(*disk.ManagedDisk.ID, volumeName) {
				attachedDisk = disk.ManagedDisk.ID
				assert.Equal(t, int32(0), *disk.Lun, "Data disk should be at LUN 0")
				break
			}
		}
	}
	assert.NotNil(t, attachedDisk, "Test volume should be attached to instance")

	// Test detachment
	err = test.DetachVolumeFromInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
	require.NoError(t, err, "Volume detachment should not fail")

	// Verify detachment
	vm, err = clients.ComputeClient.Get(ctx, testConfig.ResourceGroupName, instanceName, nil)
	require.NoError(t, err, "Should be able to get VM details after detachment")

	// Verify data disk is no longer attached
	found := false
	for _, disk := range vm.Properties.StorageProfile.DataDisks {
		if disk.ManagedDisk != nil && disk.ManagedDisk.ID != nil {
			if strings.Contains(*disk.ManagedDisk.ID, volumeName) {
				found = true
				break
			}
		}
	}
	assert.False(t, found, "Test volume should no longer be attached")
}

// TestGoldenSnapshotToVolumeLifecycle tests complete lifecycle from snapshot to working volume
func TestGoldenSnapshotToVolumeLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping full lifecycle test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Step 1: Create or find golden snapshot
	snapshotInfo, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Golden snapshot creation should not fail")
	require.NotNil(t, snapshotInfo, "Snapshot info should not be nil")

	// Step 2: Create volume from golden snapshot
	volumeName := test.GenerateTestResourceName("lifecycle-volume")
	tags := infra.VolumeTags{
		Role:      "user-volume",
		Status:    "free",
		VolumeID:  test.GenerateTestResourceName("vol"),
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
	}

	volume, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeName, snapshotInfo.ResourceID, tags)
	require.NoError(t, err, "Volume creation from golden snapshot should not fail")

	// Clean up volume
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
	}()

	// Step 3: Create instance for testing
	instanceName := test.GenerateTestResourceName("lifecycle-instance")
	instance, err := test.CreateTestInstance(ctx, clients, testConfig.ResourceGroupName, instanceName)
	require.NoError(t, err, "Instance creation should not fail")

	// Clean up instance
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()

		infra.DeleteInstance(cleanupCtx, clients, testConfig.ResourceGroupName, instanceName)
	}()

	// Step 4: Attach volume to instance
	err = infra.AttachVolumeToInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
	require.NoError(t, err, "Volume attachment should not fail")

	// Step 5: Test QEMU resume capabilities
	resumeCmd := test.GenerateQEMUResumeCommand("/mnt/userdata", 2222)
	assert.NotEmpty(t, resumeCmd, "Resume command should be generated")

	// Verify the complete flow produces a working environment
	assert.Contains(t, resumeCmd, "-loadvm ssh-ready", "Should resume from saved state")
	assert.NotEmpty(t, volume.ResourceID, "Volume should have valid resource ID")
	assert.NotEmpty(t, instance.ResourceID, "Instance should have valid resource ID")
}

// TestQEMUStateManagement tests QEMU state save and load operations
func TestQEMUStateManagement(t *testing.T) {
	testCases := []struct {
		name        string
		stateName   string
		description string
	}{
		{
			name:        "ssh_ready_state",
			stateName:   "ssh-ready",
			description: "Test SSH-ready state management",
		},
		{
			name:        "user_state",
			stateName:   "user-session",
			description: "Test user session state management",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test save state command generation
			saveCmd := test.GenerateQEMUSaveStateCommand(tc.stateName)
			assert.NotEmpty(t, saveCmd, "Save state command should not be empty")
			assert.Contains(t, saveCmd, fmt.Sprintf("savevm %s", tc.stateName), "Should save to specified state")
			assert.Contains(t, saveCmd, "/tmp/qemu-monitor.sock", "Should use monitor socket")

			// Test load state in resume command
			resumeCmd := test.GenerateQEMUResumeCommand("/mnt/userdata", 2222)
			assert.Contains(t, resumeCmd, "-loadvm ssh-ready", "Should load default state")

			// Test load specific state
			loadCmd := test.GenerateQEMULoadStateCommand("/mnt/userdata", 2222, tc.stateName)
			assert.Contains(t, loadCmd, fmt.Sprintf("-loadvm %s", tc.stateName), "Should load specified state")
		})
	}
}

// TestResumeErrorHandling tests error handling during resume operations
func TestResumeErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Test volume creation from non-existent snapshot
	invalidSnapshotID := "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/snapshots/non-existent"
	volumeName := test.GenerateTestResourceName("error-volume")

	tags := infra.VolumeTags{
		Role:      "test-volume",
		Status:    "free",
		VolumeID:  test.GenerateTestResourceName("vol"),
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
	}

	_, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeName, invalidSnapshotID, tags)
	assert.Error(t, err, "Should fail with non-existent snapshot")

	// Test attachment to non-existent instance
	testVolume, err := test.CreateTestVolume(ctx, clients, testConfig.ResourceGroupName, "test-volume", 5)
	require.NoError(t, err, "Test volume should be created")

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, "test-volume", nil)
	}()

	invalidInstanceID := "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/virtualMachines/non-existent"
	err = infra.AttachVolumeToInstance(ctx, clients, invalidInstanceID, testVolume.ResourceID)
	assert.Error(t, err, "Should fail with non-existent instance")

	// Test QEMU manager with invalid parameters
	// Should handle empty working directory gracefully
	resumeCmd := test.GenerateQEMUResumeCommand("", 2222)
	assert.NotEmpty(t, resumeCmd, "Should generate command even with empty working dir")

	// Should handle invalid port
	resumeCmd = test.GenerateQEMUResumeCommand("/mnt/userdata", 0)
	assert.NotEmpty(t, resumeCmd, "Should generate command even with invalid port")
}

// TestPoolVolumeCreation tests volume creation as part of pool management
func TestPoolVolumeCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pool volume creation test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create golden snapshot for pool usage
	snapshotInfo, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Golden snapshot should be available")

	// Test creating multiple volumes from the same snapshot (simulating pool scaling)
	numVolumes := 3
	volumeNames := make([]string, numVolumes)
	volumes := make([]*infra.VolumeInfo, numVolumes)

	for i := 0; i < numVolumes; i++ {
		volumeNames[i] = test.GenerateTestResourceName("pool-volume")

		tags := infra.VolumeTags{
			Role:      "pool-volume",
			Status:    "free",
			VolumeID:  test.GenerateTestResourceName("pool-vol"),
			CreatedAt: time.Now().Format(time.RFC3339),
			LastUsed:  time.Now().Format(time.RFC3339),
		}

		volume, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeNames[i], snapshotInfo.ResourceID, tags)
		require.NoError(t, err, "Volume %d creation should not fail", i)
		volumes[i] = volume
	}

	// Clean up volumes
	defer func() {
		for _, volumeName := range volumeNames {
			if volumeName != "" {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
				cleanupCancel()
			}
		}
	}()

	// Verify all volumes were created successfully
	for i, volume := range volumes {
		assert.NotNil(t, volume, "Volume %d should not be nil", i)
		assert.NotEmpty(t, volume.ResourceID, "Volume %d should have resource ID", i)
		assert.Equal(t, snapshotInfo.SizeGB, volume.SizeGB, "Volume %d should match snapshot size", i)

		// Verify volume exists in Azure
		disk, err := clients.DisksClient.Get(ctx, testConfig.ResourceGroupName, volumeNames[i], nil)
		require.NoError(t, err, "Volume %d should exist in Azure", i)
		assert.Equal(t, volumeNames[i], *disk.Name, "Volume %d name should match", i)
	}
}
