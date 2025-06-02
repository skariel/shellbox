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

func TestNewQEMUManager(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	// Create QEMU manager
	qm := infra.NewQEMUManager(env.Clients)
	assert.NotNil(t, qm, "should create QEMU manager")
}

func TestQEMUManagerWithMockInstance(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create instance for QEMU testing
	config := &infra.VMConfig{
		VMSize:        "Standard_D4s_v3", // Larger size needed for QEMU
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance for QEMU testing")

	// Get instance private IP
	instanceIP, err := infra.GetInstancePrivateIP(ctx, env.Clients, instanceID)
	require.NoError(t, err, "should get instance IP")

	qm := infra.NewQEMUManager(env.Clients)

	t.Run("StartQEMUWithVolume without volume should fail gracefully", func(t *testing.T) {
		// This should fail because no volume is attached and QEMU setup is not complete
		err := qm.StartQEMUWithVolume(ctx, instanceIP, "test-volume")
		assert.Error(t, err, "should fail when trying to start QEMU without proper setup")
	})

	t.Run("StopQEMU should handle non-running QEMU gracefully", func(t *testing.T) {
		// This should not error even if QEMU is not running
		err := qm.StopQEMU(ctx, instanceIP)
		assert.NoError(t, err, "should handle stopping non-running QEMU gracefully")
	})
}

func TestQEMUManagerErrorHandling(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	qm := infra.NewQEMUManager(env.Clients)

	tests := []struct {
		name       string
		instanceIP string
		operation  func() error
	}{
		{
			name:       "StartQEMUWithVolume with invalid IP",
			instanceIP: "192.168.1.999",
			operation: func() error {
				return qm.StartQEMUWithVolume(ctx, "192.168.1.999", "test-volume")
			},
		},
		{
			name:       "StopQEMU with invalid IP",
			instanceIP: "192.168.1.999",
			operation: func() error {
				return qm.StopQEMU(ctx, "192.168.1.999")
			},
		},
		{
			name:       "StartQEMUWithVolume with unreachable IP",
			instanceIP: "10.255.255.254",
			operation: func() error {
				return qm.StartQEMUWithVolume(ctx, "10.255.255.254", "test-volume")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.operation()
			assert.Error(t, err, "should fail with invalid/unreachable IP")
		})
	}
}

// TestQEMULifecycleSimulation tests QEMU operations without actual QEMU setup
// This validates the QEMU manager's command construction and error handling
func TestQEMULifecycleSimulation(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create instance for testing
	config := &infra.VMConfig{
		VMSize:        "Standard_D4s_v3",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	instanceIP, err := infra.GetInstancePrivateIP(ctx, env.Clients, instanceID)
	require.NoError(t, err, "should get instance IP")

	// Create and attach a volume
	volumeID, err := infra.CreateVolume(ctx, env.Clients, &infra.VolumeConfig{
		DiskSize: 64, // Larger volume for QEMU testing
	})
	require.NoError(t, err, "should create volume")

	err = infra.AttachVolumeToInstance(ctx, env.Clients, instanceID, volumeID)
	require.NoError(t, err, "should attach volume to instance")

	qm := infra.NewQEMUManager(env.Clients)

	// Test QEMU startup (will fail due to missing QEMU setup, but validates command execution)
	t.Run("QEMU startup command execution", func(t *testing.T) {
		// This will fail because the instance doesn't have QEMU installed/configured
		// But it validates that the QEMU manager can execute commands on the instance
		err := qm.StartQEMUWithVolume(ctx, instanceIP, volumeID)

		// We expect this to fail because QEMU is not actually set up
		assert.Error(t, err, "should fail due to missing QEMU setup")

		// Error should be related to command execution, not connection issues
		assert.Contains(t, err.Error(), "failed to start QEMU", "should fail with QEMU-specific error")
	})

	// Test QEMU stop command
	t.Run("QEMU stop command execution", func(t *testing.T) {
		// Stop should not error even if QEMU was never started
		err := qm.StopQEMU(ctx, instanceIP)
		assert.NoError(t, err, "stop command should complete without error")
	})
}

func TestQEMUManagerTimeouts(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	// Test with very short timeout to validate timeout handling
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create instance")

	instanceIP, err := infra.GetInstancePrivateIP(ctx, env.Clients, instanceID)
	require.NoError(t, err, "should get instance IP")

	qm := infra.NewQEMUManager(env.Clients)

	// Use a very short timeout context for QEMU operations
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shortCancel()

	// This should timeout quickly since QEMU setup takes much longer
	err = qm.StartQEMUWithVolume(shortCtx, instanceIP, "test-volume")
	assert.Error(t, err, "should timeout with short context")
}

func TestQEMUManagerValidation(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	qm := infra.NewQEMUManager(env.Clients)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tests := []struct {
		name       string
		instanceIP string
		volumeID   string
		expectErr  bool
	}{
		{
			name:       "empty instance IP",
			instanceIP: "",
			volumeID:   "test-volume",
			expectErr:  true,
		},
		{
			name:       "empty volume ID",
			instanceIP: "10.1.0.4",
			volumeID:   "",
			expectErr:  true,
		},
		{
			name:       "both empty",
			instanceIP: "",
			volumeID:   "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := qm.StartQEMUWithVolume(ctx, tt.instanceIP, tt.volumeID)
			if tt.expectErr {
				assert.Error(t, err, "should fail with invalid parameters")
			} else {
				// Note: even with valid parameters, this will likely fail due to missing QEMU setup
				// but we're testing parameter validation here
			}
		})
	}
}

// TestQEMUManagerConcurrency tests concurrent QEMU operations
func TestQEMUManagerConcurrency(t *testing.T) {
	test.RequireCategory(t, test.CategoryCompute)

	env := test.SetupTestEnvironment(t, test.CategoryCompute)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Create two instances for concurrent testing
	config := &infra.VMConfig{
		VMSize:        "Standard_B2s",
		AdminUsername: "shellbox",
		SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7VF...",
	}

	instanceID1, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create first instance")

	instanceID2, err := infra.CreateInstance(ctx, env.Clients, config)
	require.NoError(t, err, "should create second instance")

	instanceIP1, err := infra.GetInstancePrivateIP(ctx, env.Clients, instanceID1)
	require.NoError(t, err, "should get first instance IP")

	instanceIP2, err := infra.GetInstancePrivateIP(ctx, env.Clients, instanceID2)
	require.NoError(t, err, "should get second instance IP")

	qm := infra.NewQEMUManager(env.Clients)

	// Test concurrent stop operations (should not interfere with each other)
	t.Run("concurrent stop operations", func(t *testing.T) {
		done1 := make(chan error, 1)
		done2 := make(chan error, 1)

		// Start concurrent stop operations
		go func() {
			done1 <- qm.StopQEMU(ctx, instanceIP1)
		}()

		go func() {
			done2 <- qm.StopQEMU(ctx, instanceIP2)
		}()

		// Wait for both to complete
		err1 := <-done1
		err2 := <-done2

		// Both should complete without error (even though QEMU isn't running)
		assert.NoError(t, err1, "first stop operation should complete")
		assert.NoError(t, err2, "second stop operation should complete")
	})
}
