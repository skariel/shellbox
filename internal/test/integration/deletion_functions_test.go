package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestDeletionFunctions(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing individual deletion functions")

	t.Run("DeleteDisk", func(t *testing.T) {
		test.LogTestProgress(t, "testing DeleteDisk function")

		// Create a volume first
		config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
		volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
		require.NoError(t, err, "should create test volume")

		namer := env.GetResourceNamer()
		volumeName := namer.VolumePoolDiskName(volumeID)

		// Verify volume exists
		disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
		require.NoError(t, err, "test volume should exist")
		assert.Equal(t, volumeName, *disk.Name, "volume should have correct name")

		test.LogTestProgress(t, "deleting disk using DeleteDisk function", "diskName", volumeName)

		// Test DeleteDisk function
		infra.DeleteDisk(ctx, env.Clients, env.ResourceGroupName, volumeName, "test disk")

		// Verify disk is deleted (allow some time for deletion to complete)
		time.Sleep(5 * time.Second)
		_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
		assert.Error(t, err, "disk should be deleted after DeleteDisk call")

		test.LogTestProgress(t, "DeleteDisk test completed")
	})

	t.Run("DeleteVM_with_DeleteNIC_and_DeleteNSG", func(t *testing.T) {
		test.LogTestProgress(t, "testing DeleteVM, DeleteNIC, and DeleteNSG functions")

		// Create network infrastructure first
		infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

		// Create an instance to get VM, NIC, and NSG
		vmConfig := &infra.VMConfig{
			VMSize:        infra.VMSize,
			AdminUsername: infra.AdminUsername,
			SSHPublicKey:  "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC7... test-key",
		}

		instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
		require.NoError(t, err, "should create test instance")

		namer := env.GetResourceNamer()
		vmName := namer.BoxVMName(instanceID)
		nicName := namer.BoxNICName(instanceID)
		nsgName := namer.BoxNSGName(instanceID)

		// Verify all resources exist
		vm, err := env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
		require.NoError(t, err, "test VM should exist")
		assert.Equal(t, vmName, *vm.Name, "VM should have correct name")

		nic, err := env.Clients.NICClient.Get(ctx, env.ResourceGroupName, nicName, nil)
		require.NoError(t, err, "test NIC should exist")
		assert.Equal(t, nicName, *nic.Name, "NIC should have correct name")

		nsg, err := env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, nsgName, nil)
		require.NoError(t, err, "test NSG should exist")
		assert.Equal(t, nsgName, *nsg.Name, "NSG should have correct name")

		test.LogTestProgress(t, "testing individual deletion functions",
			"vmName", vmName, "nicName", nicName, "nsgName", nsgName)

		// Test DeleteVM function (this also deletes associated disks)
		infra.DeleteVM(ctx, env.Clients, env.ResourceGroupName, vmName, true)

		// Test DeleteNIC function
		infra.DeleteNIC(ctx, env.Clients, env.ResourceGroupName, nicName, *nic.ID)

		// Test DeleteNSG function
		infra.DeleteNSG(ctx, env.Clients, env.ResourceGroupName, nsgName)

		// Allow some time for deletion to complete
		time.Sleep(10 * time.Second)

		// Verify all resources are deleted
		_, err = env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
		assert.Error(t, err, "VM should be deleted after DeleteVM call")

		_, err = env.Clients.NICClient.Get(ctx, env.ResourceGroupName, nicName, nil)
		assert.Error(t, err, "NIC should be deleted after DeleteNIC call")

		_, err = env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, nsgName, nil)
		assert.Error(t, err, "NSG should be deleted after DeleteNSG call")

		test.LogTestProgress(t, "individual deletion functions test completed")
	})
}
