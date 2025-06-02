//go:build golden

package golden

import (
	"context"
	"testing"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoldenSnapshotCreation tests the complete golden snapshot creation workflow
func TestGoldenSnapshotCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping golden snapshot test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Setup test environment
	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Test creation of golden snapshot
	snapshotInfo, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Failed to create golden snapshot")
	require.NotNil(t, snapshotInfo, "Snapshot info should not be nil")

	// Validate snapshot properties
	assert.NotEmpty(t, snapshotInfo.Name, "Snapshot name should not be empty")
	assert.NotEmpty(t, snapshotInfo.ResourceID, "Snapshot resource ID should not be empty")
	assert.NotEmpty(t, snapshotInfo.Location, "Snapshot location should not be empty")
	assert.False(t, snapshotInfo.CreatedTime.IsZero(), "Snapshot creation time should be set")
	assert.Greater(t, snapshotInfo.SizeGB, int32(0), "Snapshot size should be positive")

	// Verify snapshot naming follows content-based pattern
	assert.Contains(t, snapshotInfo.Name, "golden-qemu-", "Snapshot name should follow content-based pattern")

	// Test idempotency - calling again should return the same snapshot
	snapshotInfo2, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Second call should not fail")
	assert.Equal(t, snapshotInfo.Name, snapshotInfo2.Name, "Should return same snapshot on second call")
	assert.Equal(t, snapshotInfo.ResourceID, snapshotInfo2.ResourceID, "Resource ID should be identical")
}

// TestQEMUScriptGeneration tests the QEMU initialization script generation
func TestQEMUScriptGeneration(t *testing.T) {
	testCases := []struct {
		name     string
		config   infra.QEMUScriptConfig
		contains []string
	}{
		{
			name: "basic_configuration",
			config: infra.QEMUScriptConfig{
				SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAA...",
				WorkingDir:    "~",
				SSHPort:       2222,
				MountDataDisk: false,
			},
			contains: []string{
				"sudo apt install qemu-utils",
				"ssh-rsa AAAAB3NzaC1yc2EAAAA...",
				"hostfwd=tcp::2222-:22",
				"ubuntu-24.04-server-cloudimg-amd64.img",
			},
		},
		{
			name: "data_disk_configuration",
			config: infra.QEMUScriptConfig{
				SSHPublicKey:  "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...",
				WorkingDir:    "/mnt/userdata",
				SSHPort:       2222,
				MountDataDisk: true,
			},
			contains: []string{
				"mkfs.ext4 /dev/disk/azure/scsi1/lun0",
				"sudo mount /dev/disk/azure/scsi1/lun0 /mnt/userdata",
				"sudo chown -R $USER:$USER /mnt/userdata/",
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...",
				"/mnt/userdata/qemu-disks",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			script, err := infra.GenerateQEMUInitScript(tc.config)
			require.NoError(t, err, "Script generation should not fail")
			require.NotEmpty(t, script, "Generated script should not be empty")

			// Decode base64 script content
			scriptContent, err := test.DecodeBase64Script(script)
			require.NoError(t, err, "Should be able to decode base64 script")

			// Verify expected content is present
			for _, expected := range tc.contains {
				assert.Contains(t, scriptContent, expected, "Script should contain expected content")
			}

			// Verify script structure
			assert.Contains(t, scriptContent, "#!/bin/bash", "Script should have shebang")
			assert.Contains(t, scriptContent, "qemu-system-x86_64", "Script should contain QEMU command")
		})
	}
}

// TestGoldenSnapshotNaming tests the content-based naming system
func TestGoldenSnapshotNaming(t *testing.T) {
	// Generate two identical configurations
	name1, err := test.GenerateSnapshotName()
	require.NoError(t, err, "First name generation should not fail")

	name2, err := test.GenerateSnapshotName()
	require.NoError(t, err, "Second name generation should not fail")

	// Names should be identical for identical configurations
	assert.Equal(t, name1, name2, "Identical configurations should produce identical names")

	// Verify naming pattern
	assert.Contains(t, name1, "golden-qemu-", "Name should follow golden-qemu- pattern")
	assert.Len(t, name1, len("golden-qemu-")+12, "Name should include 12-character hash")
}

// TestGoldenSnapshotResourceGroup tests the persistent resource group management
func TestGoldenSnapshotResourceGroup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource group test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clients, _, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Test resource group creation/verification
	err := test.EnsureGoldenResourceGroup(ctx, clients)
	require.NoError(t, err, "Resource group creation should not fail")

	// Verify resource group exists and has correct properties
	rg, err := clients.ResourceClient.Get(ctx, infra.GoldenSnapshotResourceGroup, nil)
	require.NoError(t, err, "Resource group should exist after creation")

	assert.Equal(t, infra.GoldenSnapshotResourceGroup, *rg.Name, "Resource group name should match")
	assert.NotNil(t, rg.Tags, "Resource group should have tags")
	assert.Equal(t, "golden-snapshots", *rg.Tags[infra.GoldenTagKeyPurpose], "Purpose tag should be set")
}

// TestTempBoxCreation tests the temporary VM creation for golden snapshot preparation
func TestTempBoxCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping temp box test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	clients, _, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Ensure golden resource group exists
	err := test.EnsureGoldenResourceGroup(ctx, clients)
	require.NoError(t, err, "Golden resource group should be available")

	// Create temporary box for testing
	tempBoxName := test.GenerateTestResourceName("temp-test")
	tempBox, err := test.CreateTempBox(ctx, clients, infra.GoldenSnapshotResourceGroup, tempBoxName)
	require.NoError(t, err, "Temp box creation should not fail")
	require.NotNil(t, tempBox, "Temp box info should not be nil")

	// Verify temp box properties
	assert.Equal(t, tempBoxName, tempBox.VMName, "VM name should match")
	assert.NotEmpty(t, tempBox.DataDiskID, "Data disk ID should be set")
	assert.NotEmpty(t, tempBox.PrivateIP, "Private IP should be assigned")
	assert.NotEmpty(t, tempBox.NICName, "NIC name should be set")
	assert.NotEmpty(t, tempBox.NSGName, "NSG name should be set")

	// Clean up temp box
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()

		err := infra.DeleteInstance(cleanupCtx, clients, infra.GoldenSnapshotResourceGroup, tempBoxName)
		if err != nil {
			t.Logf("Warning: failed to cleanup temp box: %v", err)
		}
	}()

	// Verify VM was created correctly
	vm, err := clients.ComputeClient.Get(ctx, infra.GoldenSnapshotResourceGroup, tempBoxName, nil)
	require.NoError(t, err, "VM should be accessible")
	assert.Equal(t, tempBoxName, *vm.Name, "VM name should match")

	// Verify data disk is attached
	require.NotEmpty(t, vm.Properties.StorageProfile.DataDisks, "VM should have data disk attached")
	assert.Equal(t, int32(0), *vm.Properties.StorageProfile.DataDisks[0].Lun, "Data disk should be at LUN 0")
}

// TestSnapshotFromVolume tests creating a snapshot from a data volume
func TestSnapshotFromVolume(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping snapshot creation test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create a test volume first
	volumeName := test.GenerateTestResourceName("test-volume")
	volume, err := test.CreateTestVolume(ctx, clients, testConfig.ResourceGroupName, volumeName, 10)
	require.NoError(t, err, "Test volume creation should not fail")
	require.NotNil(t, volume, "Volume should not be nil")

	// Clean up volume
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		_, err := clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
		if err != nil {
			t.Logf("Warning: failed to cleanup test volume: %v", err)
		}
	}()

	// Create snapshot from volume
	snapshotName := test.GenerateTestResourceName("test-snapshot")
	snapshotInfo, err := test.CreateSnapshotFromVolume(ctx, clients, testConfig.ResourceGroupName, snapshotName, volume.ResourceID)
	require.NoError(t, err, "Snapshot creation should not fail")
	require.NotNil(t, snapshotInfo, "Snapshot info should not be nil")

	// Clean up snapshot
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		_, err := clients.SnapshotsClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, snapshotName, nil)
		if err != nil {
			t.Logf("Warning: failed to cleanup test snapshot: %v", err)
		}
	}()

	// Verify snapshot properties
	assert.Equal(t, snapshotName, snapshotInfo.Name, "Snapshot name should match")
	assert.NotEmpty(t, snapshotInfo.ResourceID, "Snapshot resource ID should be set")
	assert.Equal(t, volume.ResourceID, snapshotInfo.SourceDiskID, "Source disk ID should match")
	assert.Equal(t, volume.SizeGB, snapshotInfo.SizeGB, "Snapshot size should match source")

	// Verify snapshot exists in Azure
	snapshot, err := clients.SnapshotsClient.Get(ctx, testConfig.ResourceGroupName, snapshotName, nil)
	require.NoError(t, err, "Snapshot should be accessible in Azure")
	assert.Equal(t, snapshotName, *snapshot.Name, "Azure snapshot name should match")
}

// TestGoldenSnapshotErrorHandling tests error handling scenarios
func TestGoldenSnapshotErrorHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	clients, _, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Test with invalid configuration - should handle gracefully
	config := infra.QEMUScriptConfig{
		SSHPublicKey:  "", // Invalid empty key
		WorkingDir:    "",
		SSHPort:       0,
		MountDataDisk: false,
	}

	script, err := infra.GenerateQEMUInitScript(config)
	// Should not fail even with empty values (script generation is template-based)
	require.NoError(t, err, "Script generation should handle empty values")
	assert.NotEmpty(t, script, "Script should be generated even with empty config")

	// Test snapshot creation with non-existent resource group
	invalidRG := "non-existent-resource-group-12345"
	_, err = test.CreateSnapshotFromVolume(ctx, clients, invalidRG, "test-snapshot", "invalid-disk-id")
	assert.Error(t, err, "Should fail with non-existent resource group")
}

// TestConcurrentSnapshotOperations tests concurrent snapshot operations don't interfere
func TestConcurrentSnapshotOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent operations test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create multiple volumes concurrently
	numVolumes := 3
	volumeNames := make([]string, numVolumes)
	volumes := make([]*infra.VolumeInfo, numVolumes)

	// Create volumes
	for i := 0; i < numVolumes; i++ {
		volumeNames[i] = test.GenerateTestResourceName("concurrent-vol")
		volume, err := test.CreateTestVolume(ctx, clients, testConfig.ResourceGroupName, volumeNames[i], 5)
		require.NoError(t, err, "Volume %d creation should not fail", i)
		volumes[i] = volume
	}

	// Clean up volumes
	defer func() {
		for i, volumeName := range volumeNames {
			if volumeName != "" {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				_, err := clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
				cleanupCancel()
				if err != nil {
					t.Logf("Warning: failed to cleanup volume %d: %v", i, err)
				}
			}
		}
	}()

	// Create snapshots concurrently
	snapshotResults := make(chan error, numVolumes)
	snapshotNames := make([]string, numVolumes)

	for i := 0; i < numVolumes; i++ {
		snapshotNames[i] = test.GenerateTestResourceName("concurrent-snap")
		go func(index int) {
			_, err := test.CreateSnapshotFromVolume(ctx, clients, testConfig.ResourceGroupName, snapshotNames[index], volumes[index].ResourceID)
			snapshotResults <- err
		}(i)
	}

	// Wait for all snapshots to complete
	var errors []error
	for i := 0; i < numVolumes; i++ {
		if err := <-snapshotResults; err != nil {
			errors = append(errors, err)
		}
	}

	// Clean up snapshots
	defer func() {
		for i, snapshotName := range snapshotNames {
			if snapshotName != "" {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Minute)
				_, err := clients.SnapshotsClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, snapshotName, nil)
				cleanupCancel()
				if err != nil {
					t.Logf("Warning: failed to cleanup snapshot %d: %v", i, err)
				}
			}
		}
	}()

	// Verify all snapshots were created successfully
	assert.Empty(t, errors, "All concurrent snapshot operations should succeed")

	// Verify all snapshots exist
	for i, snapshotName := range snapshotNames {
		snapshot, err := clients.SnapshotsClient.Get(ctx, testConfig.ResourceGroupName, snapshotName, nil)
		assert.NoError(t, err, "Snapshot %d should exist", i)
		if err == nil {
			assert.Equal(t, snapshotName, *snapshot.Name, "Snapshot %d name should match", i)
		}
	}
}
