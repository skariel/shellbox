package integration

import (
	"context"
	"testing"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
	"shellbox/internal/test"
)

// Helper functions to reduce cyclomatic complexity in deletion tests
func testDeleteDisk(ctx context.Context, t *testing.T, env *test.Environment) {
	t.Helper()
	test.LogTestProgress(t, "testing DeleteDisk function")

	config := &infra.VolumeConfig{DiskSize: infra.DefaultVolumeSizeGB}
	volumeID, err := infra.CreateVolume(ctx, env.Clients, config)
	if err != nil {
		t.Fatalf("should create test volume: %v", err)
	}

	namer := env.GetResourceNamer()
	volumeName := namer.VolumePoolDiskName(volumeID)

	disk, err := env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err != nil {
		t.Fatalf("test volume should exist: %v", err)
	}
	if *disk.Name != volumeName {
		t.Errorf("volume should have correct name: expected %s, got %s", volumeName, *disk.Name)
	}

	test.LogTestProgress(t, "deleting disk using DeleteDisk function", "diskName", volumeName)
	infra.DeleteDisk(ctx, env.Clients, env.ResourceGroupName, volumeName, "test disk")

	time.Sleep(5 * time.Second)
	_, err = env.Clients.DisksClient.Get(ctx, env.ResourceGroupName, volumeName, nil)
	if err == nil {
		t.Errorf("disk should be deleted after DeleteDisk call")
	}

	test.LogTestProgress(t, "DeleteDisk test completed")
}

func waitForVMDeletion(ctx context.Context, clients *infra.AzureClients, resourceGroupName, vmName string) {
	for i := 0; i < 30; i++ {
		_, err := clients.ComputeClient.Get(ctx, resourceGroupName, vmName, nil)
		if err != nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
	time.Sleep(10 * time.Second)
}

func verifyResourceDeleted(t *testing.T, err error, resourceType string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s should be deleted", resourceType)
	}
}

func testDeleteVMWithNICAndNSG(ctx context.Context, t *testing.T, env *test.Environment) {
	t.Helper()
	test.LogTestProgress(t, "testing DeleteVM, DeleteNIC, and DeleteNSG functions")

	infra.CreateNetworkInfrastructure(ctx, env.Clients, env.Config.UseAzureCLI)

	_, sshPublicKey, err := sshutil.LoadKeyPair("/home/ubuntu/.ssh/id_ed25519")
	if err != nil {
		t.Fatalf("should load SSH key: %v", err)
	}

	vmConfig := &infra.VMConfig{
		VMSize:        infra.VMSize,
		AdminUsername: infra.AdminUsername,
		SSHPublicKey:  sshPublicKey,
	}

	instanceID, err := infra.CreateInstance(ctx, env.Clients, vmConfig)
	if err != nil {
		t.Fatalf("should create test instance: %v", err)
	}

	namer := env.GetResourceNamer()
	vmName := namer.BoxVMName(instanceID)
	nicName := namer.BoxNICName(instanceID)
	nsgName := namer.BoxNSGName(instanceID)

	nic, err := env.Clients.NICClient.Get(ctx, env.ResourceGroupName, nicName, nil)
	if err != nil {
		t.Fatalf("test NIC should exist: %v", err)
	}

	test.LogTestProgress(t, "testing individual deletion functions",
		"vmName", vmName, "nicName", nicName, "nsgName", nsgName)

	infra.DeleteVM(ctx, env.Clients, env.ResourceGroupName, vmName, true)

	test.LogTestProgress(t, "waiting for VM deletion to complete")
	waitForVMDeletion(ctx, env.Clients, env.ResourceGroupName, vmName)

	infra.DeleteNIC(ctx, env.Clients, env.ResourceGroupName, nicName, *nic.ID)
	infra.DeleteNSG(ctx, env.Clients, env.ResourceGroupName, nsgName)

	_, err = env.Clients.ComputeClient.Get(ctx, env.ResourceGroupName, vmName, nil)
	verifyResourceDeleted(t, err, "VM")

	_, err = env.Clients.NICClient.Get(ctx, env.ResourceGroupName, nicName, nil)
	verifyResourceDeleted(t, err, "NIC")

	_, err = env.Clients.NSGClient.Get(ctx, env.ResourceGroupName, nsgName, nil)
	verifyResourceDeleted(t, err, "NSG")

	test.LogTestProgress(t, "individual deletion functions test completed")
}

func TestDeletionFunctions(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing individual deletion functions")

	t.Run("DeleteDisk", func(t *testing.T) {
		testDeleteDisk(ctx, t, env)
	})

	t.Run("DeleteVM_with_DeleteNIC_and_DeleteNSG", func(t *testing.T) {
		testDeleteVMWithNICAndNSG(ctx, t, env)
	})
}
