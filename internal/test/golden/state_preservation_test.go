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

// TestQEMUStatePreservation tests that QEMU VM state is properly preserved across suspend/resume cycles
func TestQEMUStatePreservation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping state preservation test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create test infrastructure
	instanceName := test.GenerateTestResourceName("state-test-instance")
	instance, err := test.CreateTestInstance(ctx, clients, testConfig.ResourceGroupName, instanceName)
	require.NoError(t, err, "Instance creation should not fail")

	// Clean up instance
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()

		infra.DeleteInstance(cleanupCtx, clients, testConfig.ResourceGroupName, instanceName)
	}()

	// Create volume from golden snapshot
	snapshotInfo, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Golden snapshot should be available")

	volumeName := test.GenerateTestResourceName("state-test-volume")
	tags := infra.VolumeTags{
		Role:      "state-test",
		Status:    "attached",
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
		VolumeID:  "test-volume-id",
	}

	volume, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeName, snapshotInfo.ResourceID, tags)
	require.NoError(t, err, "Volume creation should not fail")

	// Clean up volume
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
	}()

	// Attach volume to instance
	err = infra.AttachVolumeToInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
	require.NoError(t, err, "Volume attachment should not fail")

	// Get instance IP for SSH operations
	instanceIP, err := infra.GetInstancePrivateIP(ctx, clients, instanceName)
	require.NoError(t, err, "Should be able to get instance IP")

	// Start QEMU and verify it reaches SSH-ready state
	qemuManager := infra.NewQEMUManager(clients)
	err = qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
	require.NoError(t, err, "QEMU should start successfully")

	// Test 1: Verify SSH connectivity to QEMU VM
	t.Run("verify_ssh_connectivity", func(t *testing.T) {
		err := test.WaitForQEMUSSH(ctx, instanceIP, 2222, 5*time.Minute)
		require.NoError(t, err, "QEMU VM should be SSH accessible")
	})

	// Test 2: Create test data in QEMU filesystem
	testData := map[string]string{
		"/home/ubuntu/test_file.txt":   "This is persistent test data",
		"/home/ubuntu/state_test.json": `{"test": "data", "timestamp": "` + time.Now().Format(time.RFC3339) + `"}`,
		"/home/ubuntu/memory_test.sh":  "#!/bin/bash\necho 'Memory test script executed successfully'",
		"/tmp/session_marker.txt":      "Session marker for state preservation test",
	}

	t.Run("create_test_data", func(t *testing.T) {
		for filePath, content := range testData {
			createCmd := fmt.Sprintf("echo '%s' > %s", content, filePath)
			err := test.ExecuteQEMUCommand(ctx, instanceIP, 2222, createCmd)
			require.NoError(t, err, "Should be able to create test file: %s", filePath)
		}

		// Make script executable
		err := test.ExecuteQEMUCommand(ctx, instanceIP, 2222, "chmod +x /home/ubuntu/memory_test.sh")
		require.NoError(t, err, "Should be able to make script executable")
	})

	// Test 3: Create memory state (running processes, environment variables)
	t.Run("create_memory_state", func(t *testing.T) {
		// Set environment variables
		envCmd := "export TEST_VAR='preservation_test' && echo $TEST_VAR > /tmp/env_test.txt"
		err := test.ExecuteQEMUCommand(ctx, instanceIP, 2222, envCmd)
		require.NoError(t, err, "Should be able to set environment variables")

		// Start background process
		bgCmd := "nohup sleep 300 > /tmp/bg_process.log 2>&1 & echo $! > /tmp/bg_pid.txt"
		err = test.ExecuteQEMUCommand(ctx, instanceIP, 2222, bgCmd)
		require.NoError(t, err, "Should be able to start background process")
	})

	// Test 4: Save QEMU state
	t.Run("save_qemu_state", func(t *testing.T) {
		mockManager := test.NewMockQEMUManager(clients)
		err := mockManager.SaveState(ctx, instanceIP, "test-state")
		require.NoError(t, err, "Should be able to save QEMU state")

		// Verify state was saved
		checkCmd := "ls -la /mnt/userdata/qemu-memory/"
		output, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, checkCmd)
		require.NoError(t, err, "Should be able to check memory directory")
		assert.Contains(t, output, "ubuntu-mem", "Memory file should exist")
	})

	// Test 5: Stop QEMU gracefully
	t.Run("stop_qemu", func(t *testing.T) {
		err := qemuManager.StopQEMU(ctx, instanceIP)
		require.NoError(t, err, "Should be able to stop QEMU")

		// Verify QEMU is no longer running
		time.Sleep(10 * time.Second) // Allow time for shutdown
		err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 30*time.Second)
		assert.Error(t, err, "QEMU should no longer be accessible after stop")
	})

	// Test 6: Resume QEMU from saved state
	t.Run("resume_qemu_from_state", func(t *testing.T) {
		err := qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
		require.NoError(t, err, "Should be able to resume QEMU")

		// Wait for SSH to be available again
		err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 5*time.Minute)
		require.NoError(t, err, "QEMU should be SSH accessible after resume")
	})

	// Test 7: Verify filesystem data preservation
	t.Run("verify_filesystem_preservation", func(t *testing.T) {
		for filePath, expectedContent := range testData {
			output, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, fmt.Sprintf("cat %s", filePath))
			require.NoError(t, err, "Should be able to read file: %s", filePath)
			assert.Contains(t, output, expectedContent, "File content should be preserved: %s", filePath)
		}

		// Verify script is still executable
		output, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, "/home/ubuntu/memory_test.sh")
		require.NoError(t, err, "Should be able to execute preserved script")
		assert.Contains(t, output, "Memory test script executed successfully", "Script should execute correctly")
	})

	// Test 8: Verify memory state preservation (limited scope)
	t.Run("verify_memory_state_preservation", func(t *testing.T) {
		// Check if background process information was preserved
		output, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, "cat /tmp/bg_pid.txt")
		require.NoError(t, err, "Should be able to read background process PID")
		assert.NotEmpty(t, strings.TrimSpace(output), "Background process PID should be recorded")

		// Check environment variable file
		output, err = test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, "cat /tmp/env_test.txt")
		require.NoError(t, err, "Should be able to read environment test file")
		assert.Contains(t, output, "preservation_test", "Environment variable should be preserved in file")
	})
}

// TestFilesystemPersistence tests that filesystem changes persist across volume operations
func TestFilesystemPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping filesystem persistence test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create test infrastructure
	instanceName := test.GenerateTestResourceName("fs-test-instance")
	instance, err := test.CreateTestInstance(ctx, clients, testConfig.ResourceGroupName, instanceName)
	require.NoError(t, err, "Instance creation should not fail")

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()

		infra.DeleteInstance(cleanupCtx, clients, testConfig.ResourceGroupName, instanceName)
	}()

	// Create volume from golden snapshot
	snapshotInfo, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Golden snapshot should be available")

	volumeName := test.GenerateTestResourceName("fs-test-volume")
	tags := infra.VolumeTags{
		Role:      "filesystem-test",
		Status:    "attached",
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
		VolumeID:  "fs-test-volume",
	}

	volume, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeName, snapshotInfo.ResourceID, tags)
	require.NoError(t, err, "Volume creation should not fail")

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
	}()

	// Get instance IP
	instanceIP, err := infra.GetInstancePrivateIP(ctx, clients, instanceName)
	require.NoError(t, err, "Should be able to get instance IP")

	// Test multiple attach/detach cycles
	for cycle := 1; cycle <= 3; cycle++ {
		t.Run(fmt.Sprintf("cycle_%d", cycle), func(t *testing.T) {
			// Attach volume
			err := infra.AttachVolumeToInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
			require.NoError(t, err, "Volume attachment should not fail")

			// Start QEMU
			qemuManager := infra.NewQEMUManager(clients)
			err = qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
			require.NoError(t, err, "QEMU should start successfully")

			// Wait for SSH
			err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 3*time.Minute)
			require.NoError(t, err, "QEMU should become SSH accessible")

			// Create cycle-specific data
			cycleFile := fmt.Sprintf("/home/ubuntu/cycle_%d.txt", cycle)
			cycleData := fmt.Sprintf("Data from cycle %d - %s", cycle, time.Now().Format(time.RFC3339))

			createCmd := fmt.Sprintf("echo '%s' > %s", cycleData, cycleFile)
			err = test.ExecuteQEMUCommand(ctx, instanceIP, 2222, createCmd)
			require.NoError(t, err, "Should be able to create cycle file")

			// Verify all previous cycle files still exist
			for prevCycle := 1; prevCycle < cycle; prevCycle++ {
				prevFile := fmt.Sprintf("/home/ubuntu/cycle_%d.txt", prevCycle)
				output, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, fmt.Sprintf("cat %s", prevFile))
				require.NoError(t, err, "Previous cycle file should exist: %s", prevFile)
				assert.Contains(t, output, fmt.Sprintf("Data from cycle %d", prevCycle), "Previous cycle data should be preserved")
			}

			// Stop QEMU
			err = qemuManager.StopQEMU(ctx, instanceIP)
			require.NoError(t, err, "Should be able to stop QEMU")

			// Detach volume
			err = test.DetachVolumeFromInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
			require.NoError(t, err, "Volume detachment should not fail")

			// Wait between cycles
			time.Sleep(5 * time.Second)
		})
	}
}

// TestVolumeDataIntegrity tests data integrity across different volume operations
func TestVolumeDataIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping data integrity test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	clients, testConfig, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Create test infrastructure
	instanceName := test.GenerateTestResourceName("integrity-instance")
	instance, err := test.CreateTestInstance(ctx, clients, testConfig.ResourceGroupName, instanceName)
	require.NoError(t, err, "Instance creation should not fail")

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cleanupCancel()

		infra.DeleteInstance(cleanupCtx, clients, testConfig.ResourceGroupName, instanceName)
	}()

	// Create volume from golden snapshot
	snapshotInfo, err := infra.CreateGoldenSnapshotIfNotExists(ctx, clients, testConfig.ResourceGroupName, testConfig.Suffix)
	require.NoError(t, err, "Golden snapshot should be available")

	volumeName := test.GenerateTestResourceName("integrity-volume")
	tags := infra.VolumeTags{
		Role:      "integrity-test",
		Status:    "attached",
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
		VolumeID:  "integrity-volume",
	}

	volume, err := infra.CreateVolumeFromSnapshot(ctx, clients, testConfig.ResourceGroupName, volumeName, snapshotInfo.ResourceID, tags)
	require.NoError(t, err, "Volume creation should not fail")

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cleanupCancel()

		clients.DisksClient.BeginDelete(cleanupCtx, testConfig.ResourceGroupName, volumeName, nil)
	}()

	// Attach volume and start QEMU
	err = infra.AttachVolumeToInstance(ctx, clients, instance.ResourceID, volume.ResourceID)
	require.NoError(t, err, "Volume attachment should not fail")

	instanceIP, err := infra.GetInstancePrivateIP(ctx, clients, instanceName)
	require.NoError(t, err, "Should be able to get instance IP")

	qemuManager := infra.NewQEMUManager(clients)
	err = qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
	require.NoError(t, err, "QEMU should start successfully")

	err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 3*time.Minute)
	require.NoError(t, err, "QEMU should become SSH accessible")

	// Test 1: Create large test file and verify integrity
	t.Run("large_file_integrity", func(t *testing.T) {
		// Create a large file with known pattern
		createCmd := "dd if=/dev/zero of=/home/ubuntu/large_test.dat bs=1M count=100"
		err := test.ExecuteQEMUCommand(ctx, instanceIP, 2222, createCmd)
		require.NoError(t, err, "Should be able to create large file")

		// Get file checksum
		checksumCmd := "sha256sum /home/ubuntu/large_test.dat"
		checksum1, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, checksumCmd)
		require.NoError(t, err, "Should be able to calculate checksum")

		// Stop and restart QEMU
		err = qemuManager.StopQEMU(ctx, instanceIP)
		require.NoError(t, err, "Should be able to stop QEMU")

		err = qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
		require.NoError(t, err, "Should be able to restart QEMU")

		err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 3*time.Minute)
		require.NoError(t, err, "QEMU should become SSH accessible after restart")

		// Verify file integrity
		checksum2, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, checksumCmd)
		require.NoError(t, err, "Should be able to recalculate checksum")

		assert.Equal(t, strings.TrimSpace(checksum1), strings.TrimSpace(checksum2), "File checksum should remain identical")
	})

	// Test 2: Directory structure preservation
	t.Run("directory_structure_preservation", func(t *testing.T) {
		// Create complex directory structure
		dirs := []string{
			"/home/ubuntu/test_project/src/main",
			"/home/ubuntu/test_project/src/utils",
			"/home/ubuntu/test_project/tests/unit",
			"/home/ubuntu/test_project/tests/integration",
			"/home/ubuntu/test_project/docs",
		}

		for _, dir := range dirs {
			err := test.ExecuteQEMUCommand(ctx, instanceIP, 2222, fmt.Sprintf("mkdir -p %s", dir))
			require.NoError(t, err, "Should be able to create directory: %s", dir)

			// Create test file in each directory
			testFile := fmt.Sprintf("%s/test.txt", dir)
			err = test.ExecuteQEMUCommand(ctx, instanceIP, 2222, fmt.Sprintf("echo 'Test file in %s' > %s", dir, testFile))
			require.NoError(t, err, "Should be able to create test file: %s", testFile)
		}

		// Save state and restart
		mockManager := test.NewMockQEMUManager(clients)
		err := mockManager.SaveState(ctx, instanceIP, "structure-test")
		require.NoError(t, err, "Should be able to save state")

		err = qemuManager.StopQEMU(ctx, instanceIP)
		require.NoError(t, err, "Should be able to stop QEMU")

		err = qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
		require.NoError(t, err, "Should be able to restart QEMU")

		err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 3*time.Minute)
		require.NoError(t, err, "QEMU should become SSH accessible")

		// Verify directory structure
		for _, dir := range dirs {
			// Check directory exists
			checkCmd := fmt.Sprintf("test -d %s && echo 'exists' || echo 'missing'", dir)
			output, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, checkCmd)
			require.NoError(t, err, "Should be able to check directory: %s", dir)
			assert.Contains(t, output, "exists", "Directory should exist: %s", dir)

			// Check test file exists and has correct content
			testFile := fmt.Sprintf("%s/test.txt", dir)
			output, err = test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, fmt.Sprintf("cat %s", testFile))
			require.NoError(t, err, "Should be able to read test file: %s", testFile)
			assert.Contains(t, output, fmt.Sprintf("Test file in %s", dir), "File content should be preserved")
		}
	})

	// Test 3: Binary file preservation
	t.Run("binary_file_preservation", func(t *testing.T) {
		// Create binary file with random data
		createBinaryCmd := "dd if=/dev/urandom of=/home/ubuntu/binary_test.bin bs=1024 count=10"
		err := test.ExecuteQEMUCommand(ctx, instanceIP, 2222, createBinaryCmd)
		require.NoError(t, err, "Should be able to create binary file")

		// Get binary file checksum
		checksumCmd := "md5sum /home/ubuntu/binary_test.bin"
		checksum1, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, checksumCmd)
		require.NoError(t, err, "Should be able to calculate binary checksum")

		// Restart QEMU
		err = qemuManager.StopQEMU(ctx, instanceIP)
		require.NoError(t, err, "Should be able to stop QEMU")

		err = qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID)
		require.NoError(t, err, "Should be able to restart QEMU")

		err = test.WaitForQEMUSSH(ctx, instanceIP, 2222, 3*time.Minute)
		require.NoError(t, err, "QEMU should become SSH accessible")

		// Verify binary file integrity
		checksum2, err := test.ExecuteQEMUCommandWithOutput(ctx, instanceIP, 2222, checksumCmd)
		require.NoError(t, err, "Should be able to recalculate binary checksum")

		assert.Equal(t, strings.TrimSpace(checksum1), strings.TrimSpace(checksum2), "Binary file should remain identical")
	})
}

// TestStatePreservationErrorHandling tests error scenarios in state preservation
func TestStatePreservationErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	clients, _, cleanup := test.SetupTest(t, test.CategoryGolden)
	defer cleanup()

	// Test with invalid volume operations
	t.Run("invalid_volume_operations", func(t *testing.T) {
		qemuManager := infra.NewQEMUManager(clients)

		// Test starting QEMU with non-existent volume
		invalidVolumeID := "/subscriptions/test/resourceGroups/test/providers/Microsoft.Compute/disks/non-existent"
		err := qemuManager.StartQEMUWithVolume(ctx, "10.0.0.1", invalidVolumeID)
		assert.Error(t, err, "Should fail with non-existent volume")

		// Test stopping QEMU when not running
		err = qemuManager.StopQEMU(ctx, "10.0.0.1")
		assert.Error(t, err, "Should fail when QEMU is not running")
	})

	// Test state save/load error conditions
	t.Run("state_save_load_errors", func(t *testing.T) {
		_ = infra.NewQEMUManager(clients) // qemuManager not used in this subtest

		// Test saving state when QEMU is not running
		mockManager := test.NewMockQEMUManager(clients)
		err := mockManager.SaveState(ctx, "10.0.0.1", "test-state")
		assert.Error(t, err, "Should fail to save state when QEMU not running")

		// Test loading non-existent state
		loadCmd := test.GenerateQEMULoadStateCommand("/mnt/userdata", 2222, "non-existent-state")
		assert.Contains(t, loadCmd, "-loadvm non-existent-state", "Should generate load command even for non-existent state")
	})

	// Test command generation with invalid parameters
	t.Run("invalid_command_parameters", func(t *testing.T) {
		_ = infra.NewQEMUManager(clients) // qemuManager not used in this subtest

		// Test with various invalid parameters
		tests := []struct {
			workingDir string
			port       int
			desc       string
		}{
			{"", 2222, "empty working directory"},
			{"/mnt/userdata", 0, "invalid port"},
			{"/nonexistent", 2222, "nonexistent directory"},
		}

		for _, tt := range tests {
			cmd := test.GenerateQEMUResumeCommand(tt.workingDir, tt.port)
			assert.NotEmpty(t, cmd, "Should generate command even with %s", tt.desc)
		}
	})
}
