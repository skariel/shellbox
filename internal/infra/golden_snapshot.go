package infra

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os/exec"
	"shellbox/internal/sshutil"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

// QEMUScriptConfig holds configuration for generating QEMU initialization scripts
type QEMUScriptConfig struct {
	SSHPublicKey  string
	WorkingDir    string // "~" for home directory, "/mnt/userdata" for data volume
	SSHPort       int
	MountDataDisk bool // Whether to mount and format a data disk first
}

// GenerateQEMUInitScript creates a QEMU initialization script with the given configuration
func GenerateQEMUInitScript(config QEMUScriptConfig) (string, error) {
	var mountSection string
	if config.MountDataDisk {
		mountSection = `
# Wait for data disk to be available
while [ ! -e /dev/disk/azure/scsi1/lun0 ]; do
    echo "Waiting for data disk..."
    sleep 5
done

# Format and mount data disk
sudo mkfs.ext4 /dev/disk/azure/scsi1/lun0
sudo mkdir -p /mnt/userdata
sudo mount /dev/disk/azure/scsi1/lun0 /mnt/userdata
echo '/dev/disk/azure/scsi1/lun0 /mnt/userdata ext4 defaults 0 2' | sudo tee -a /etc/fstab
`
	}

	var ownershipSection string
	if config.WorkingDir == "/mnt/userdata" {
		ownershipSection = `
# Set ownership for data volume
sudo chown -R $USER:$USER /mnt/userdata/
`
	}

	script := fmt.Sprintf(`#!/bin/bash

echo "\$nrconf{restart} = 'a';" | sudo tee /etc/needrestart/conf.d/50-autorestart.conf
%s
# Install QEMU and dependencies
sudo apt update
sudo apt install qemu-utils qemu-system-x86 qemu-kvm qemu-system libvirt-daemon-system libvirt-clients bridge-utils genisoimage whois libguestfs-tools socat -y

sudo usermod -aG kvm,libvirt $USER
sudo systemctl enable --now libvirtd

# Create QEMU environment
mkdir -p %s/qemu-disks %s/qemu-memory
%s
# Download and prepare Ubuntu image
cd %s/
wget https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
qemu-img convert -f qcow2 -O qcow2 ubuntu-24.04-server-cloudimg-amd64.img qemu-disks/ubuntu-base.qcow2
qemu-img resize qemu-disks/ubuntu-base.qcow2 64G

# Create cloud-init configuration for SSH access
cat > user-data << 'EOFMARKER'
#cloud-config
hostname: ubuntu
users:
  - name: ubuntu
    ssh_authorized_keys:
      - '%s'
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
package_update: true
packages:
  - openssh-server
ssh_pwauth: false
ssh:
  install-server: yes
  permit_root_login: false
  password_authentication: false
EOFMARKER

cat > meta-data << 'EOFMARKER'
instance-id: ubuntu-inst-1
local-hostname: ubuntu
EOFMARKER

genisoimage -output qemu-disks/cloud-init.iso -volid cidata -joliet -rock user-data meta-data

# Start QEMU VM with SSH-ready configuration and monitor socket
sudo qemu-system-x86_64 \
   -enable-kvm \
   -m 24G \
   -mem-path %s/qemu-memory/ubuntu-mem \
   -smp 8 \
   -cpu host \
   -drive file=%s/qemu-disks/ubuntu-base.qcow2,format=qcow2 \
   -cdrom %s/qemu-disks/cloud-init.iso \
   -nographic \
   -monitor unix:/tmp/qemu-monitor.sock,server,nowait \
   -nic user,model=virtio,hostfwd=tcp::%d-:22,dns=8.8.8.8`,
		mountSection,
		config.WorkingDir, config.WorkingDir,
		ownershipSection,
		config.WorkingDir,
		config.SSHPublicKey,
		config.WorkingDir,
		config.WorkingDir, config.WorkingDir,
		config.SSHPort)

	return base64.StdEncoding.EncodeToString([]byte(script)), nil
}

// GoldenSnapshotInfo contains information about the created golden snapshots and image
type GoldenSnapshotInfo struct {
	// Data volume snapshot information
	DataSnapshotName       string
	DataSnapshotResourceID string
	// OS image information (created directly from OS disk)
	OSImageName       string
	OSImageResourceID string
	// Common fields
	Location    string
	CreatedTime time.Time
	DataSizeGB  int32
	OSSizeGB    int32
}

// CreateGoldenSnapshotIfNotExists creates golden resources containing a pre-configured QEMU environment.
// This creates a data volume snapshot (for user volumes) and a custom VM image (for fast instance creation).
// The function is idempotent - it will find and return existing resources rather than creating duplicates.
// Golden resources are stored in a persistent resource group to avoid recreation between deployments.
func CreateGoldenSnapshotIfNotExists(ctx context.Context, clients *AzureClients) (*GoldenSnapshotInfo, error) {
	// Load SSH key
	_, sshPublicKey, err := sshutil.LoadKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	// Ensure the persistent resource group exists
	if err := ensureGoldenSnapshotResourceGroup(ctx, clients); err != nil {
		return nil, fmt.Errorf("failed to ensure golden snapshot resource group: %w", err)
	}

	// Generate content-based snapshot names for this QEMU configuration
	dataSnapshotName, osSnapshotName, err := generateGoldenSnapshotNames(sshPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate snapshot names: %w", err)
	}

	// Check if golden data snapshot and OS image already exist in the persistent resource group
	osImageName := fmt.Sprintf("%s-image", osSnapshotName)
	slog.Info("Checking for existing golden data snapshot and OS image", "dataSnapshot", dataSnapshotName, "osImage", osImageName, "resourceGroup", GoldenSnapshotResourceGroup)

	dataSnapshot, dataErr := clients.SnapshotsClient.Get(ctx, GoldenSnapshotResourceGroup, dataSnapshotName, nil)
	osImage, imageErr := clients.ImagesClient.Get(ctx, GoldenSnapshotResourceGroup, osImageName, nil)

	if dataErr == nil && imageErr == nil {
		slog.Info("Found existing golden data snapshot and OS image", "dataSnapshot", dataSnapshotName, "osImage", osImageName)
		return &GoldenSnapshotInfo{
			DataSnapshotName:       *dataSnapshot.Name,
			DataSnapshotResourceID: *dataSnapshot.ID,
			OSImageName:            *osImage.Name,
			OSImageResourceID:      *osImage.ID,
			Location:               *dataSnapshot.Location,
			CreatedTime:            *dataSnapshot.Properties.TimeCreated,
			DataSizeGB:             *dataSnapshot.Properties.DiskSizeGB,
			OSSizeGB:               *osImage.Properties.StorageProfile.OSDisk.DiskSizeGB,
		}, nil
	}

	slog.Info("Golden resources not found, creating new ones", "dataSnapshot", dataSnapshotName, "osImage", osImageName)
	// Create temporary box VM with data volume for QEMU setup
	tempBoxName := fmt.Sprintf("temp-golden-%d", time.Now().Unix())
	slog.Info("Creating temporary box VM", "tempBoxName", tempBoxName)

	tempBox, err := createBoxWithDataVolume(ctx, clients, clients.ResourceGroupName, tempBoxName, sshPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary box for golden snapshot: %w", err)
	}

	// Wait for the VM to be ready and QEMU setup to complete
	slog.Info("Waiting for QEMU setup to complete on temporary box")
	if err := waitForQEMUSetup(ctx, clients, tempBox); err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteInstance(ctx, clients, clients.ResourceGroupName, tempBoxName); cleanupErr != nil {
			slog.Warn("Failed to cleanup temporary box during error recovery", "error", cleanupErr)
		}
		return nil, fmt.Errorf("failed waiting for QEMU setup: %w", err)
	}

	// Generalize the VM before image creation
	slog.Info("Generalizing VM before image creation", "vmName", tempBoxName)
	if err := GeneralizeVM(ctx, clients, clients.ResourceGroupName, tempBoxName); err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteInstance(ctx, clients, clients.ResourceGroupName, tempBoxName); cleanupErr != nil {
			slog.Warn("Failed to cleanup temporary box during error recovery", "error", cleanupErr)
		}
		return nil, fmt.Errorf("failed to generalize VM before image creation: %w", err)
	}

	// Create data snapshot and OS image from the VM in the persistent resource group
	slog.Info("Creating data snapshot and OS image from VM")
	snapshotInfo, err := createSnapshotsFromVM(ctx, clients, GoldenSnapshotResourceGroup, dataSnapshotName, osSnapshotName, tempBox)
	if err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteInstance(ctx, clients, clients.ResourceGroupName, tempBoxName); cleanupErr != nil {
			slog.Warn("Failed to cleanup temporary box during error recovery", "error", cleanupErr)
		}
		return nil, fmt.Errorf("failed to create snapshots: %w", err)
	}

	// Cleanup temporary resources
	slog.Info("Cleaning up temporary resources")
	if err := DeleteInstance(ctx, clients, clients.ResourceGroupName, tempBoxName); err != nil {
		slog.Warn("Failed to cleanup temporary box", "tempBoxName", tempBoxName, "error", err)
		// Don't fail the operation - snapshots were created successfully
	}

	slog.Info("Golden resources created successfully", "dataSnapshot", dataSnapshotName, "osImage", osImageName)
	return snapshotInfo, nil
}

// tempBoxInfo holds information about a temporary box created for golden snapshot
type tempBoxInfo struct {
	VMName     string
	DataDiskID string
	OSDiskID   string
	PrivateIP  string
	PublicIP   string
	NICName    string
	NSGName    string
	DiskName   string
}

// createBoxWithDataVolume creates a temporary box VM with a data volume for QEMU setup
func createBoxWithDataVolume(ctx context.Context, clients *AzureClients, resourceGroupName, vmName, sshPublicKey string) (*tempBoxInfo, error) {
	namer := NewResourceNamer(ExtractSuffix(resourceGroupName))

	// Create data volume using golden-specific tagging
	dataDiskName := fmt.Sprintf("%s-data", vmName)
	now := time.Now().UTC()

	// Use separate disk creation to avoid pool tag namespace
	diskParams := armcompute.Disk{
		Location: to.Ptr(Location),
		Properties: &armcompute.DiskProperties{
			DiskSizeGB: to.Ptr(int32(DefaultVolumeSizeGB)),
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
			},
		},
		Tags: map[string]*string{
			GoldenTagKeyRole:    to.Ptr(GoldenRoleTempDataDisk),
			GoldenTagKeyPurpose: to.Ptr("golden-snapshot-creation"),
			GoldenTagKeyCreated: to.Ptr(now.Format(time.RFC3339)),
			GoldenTagKeyStage:   to.Ptr("creating"),
		},
	}

	// Create data disk directly with golden-specific tags
	poller, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroupName, dataDiskName, diskParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start data volume creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create data volume: %w", err)
	}

	volumeInfo := &VolumeInfo{
		Name:       *result.Name,
		ResourceID: *result.ID,
		Location:   *result.Location,
		SizeGB:     *result.Properties.DiskSizeGB,
	}

	// Use existing box creation functions but with a custom boxID
	boxID := vmName // Use vmName as boxID for temp box
	nsgName := namer.BoxNSGName(boxID)
	nicName := namer.BoxNICName(boxID)

	// Create NSG using existing function
	nsgResult, err := createInstanceNSG(ctx, clients, nsgName)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG: %w", err)
	}

	// Create NIC using existing function
	nicResult, err := createInstanceNIC(ctx, clients, nicName, nsgResult.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create NIC: %w", err)
	}

	// Create VM with data disk attached using modified function
	vmResult, err := createBoxVMWithDataDisk(ctx, clients, resourceGroupName, vmName, *nicResult.ID, volumeInfo.ResourceID, sshPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	// Extract OS disk ID from VM properties
	osDiskID := ""
	if vmResult.Properties != nil && vmResult.Properties.StorageProfile != nil &&
		vmResult.Properties.StorageProfile.OSDisk != nil &&
		vmResult.Properties.StorageProfile.OSDisk.ManagedDisk != nil &&
		vmResult.Properties.StorageProfile.OSDisk.ManagedDisk.ID != nil {
		osDiskID = *vmResult.Properties.StorageProfile.OSDisk.ManagedDisk.ID
	}

	return &tempBoxInfo{
		VMName:     vmName,
		DataDiskID: volumeInfo.ResourceID,
		OSDiskID:   osDiskID,
		PrivateIP:  *nicResult.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
		NICName:    nicName,
		NSGName:    nsgName,
		DiskName:   dataDiskName,
	}, nil
}

// waitForQEMUSetup waits for the QEMU VM to be accessible via SSH on port 2222
func waitForQEMUSetup(ctx context.Context, _ *AzureClients, tempBox *tempBoxInfo) error {
	slog.Info("Waiting for QEMU VM to be SSH-ready", "vmName", tempBox.VMName, "privateIP", tempBox.PrivateIP)

	// First, wait for SSH connectivity to the QEMU VM
	slog.Info("Testing SSH connectivity to QEMU VM", "port", BoxSSHPort)
	err := RetryOperation(ctx, func(ctx context.Context) error {
		// Test SSH connection directly to the QEMU VM from bastion
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "ConnectTimeout=5",
			"-o", "StrictHostKeyChecking=no",
			"-i", sshutil.SSHKeyPath,
			"-p", fmt.Sprintf("%d", BoxSSHPort),
			fmt.Sprintf("%s@%s", SystemUserUbuntu, tempBox.PrivateIP),
			"echo 'SSH test successful'")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("QEMU VM SSH not yet ready: %w: %s", err, string(output))
		}
		return nil
	}, GoldenVMSetupTimeout, 30*time.Second, "QEMU VM SSH connectivity")
	if err != nil {
		return err
	}

	// SSH is ready, now save the VM state
	slog.Info("QEMU VM SSH confirmed working, saving VM state")

	// Check if any snapshots exist already
	checkCmd := `echo "info snapshots" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock`
	checkOutput, _ := sshutil.ExecuteCommandWithOutput(ctx, checkCmd, AdminUsername, tempBox.PrivateIP)
	slog.Info("Current snapshots before save", "output", checkOutput)

	// Save snapshot - wait for completion by checking the output
	saveCmd := `echo "savevm ssh-ready" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock`
	saveOutput, saveErr := sshutil.ExecuteCommandWithOutput(ctx, saveCmd, AdminUsername, tempBox.PrivateIP)
	slog.Info("Savevm command sent", "output", saveOutput, "error", saveErr)

	// Wait for snapshot to be saved by repeatedly checking snapshots list
	err = RetryOperation(ctx, func(ctx context.Context) error {
		checkCmd := `echo "info snapshots" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock`
		checkOutput, _ := sshutil.ExecuteCommandWithOutput(ctx, checkCmd, AdminUsername, tempBox.PrivateIP)
		if !strings.Contains(checkOutput, "ssh-ready") {
			return fmt.Errorf("snapshot 'ssh-ready' not yet visible in QEMU")
		}
		slog.Info("Snapshot confirmed in QEMU", "output", checkOutput)
		return nil
	}, 60*time.Second, 3*time.Second, "QEMU snapshot save")
	if err != nil {
		return fmt.Errorf("failed to save QEMU snapshot: %w", err)
	}

	// Attempt to quit QEMU gracefully (but don't wait for it)
	quitCmd := `echo "quit" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock`
	quitOutput, quitErr := sshutil.ExecuteCommandWithOutput(ctx, quitCmd, AdminUsername, tempBox.PrivateIP)
	slog.Info("Quit command sent", "output", quitOutput, "error", quitErr)

	// Force kill QEMU to ensure we can access the qcow2 file
	_ = sshutil.ExecuteCommand(ctx, `sudo pkill -9 qemu-system-x86_64 || true`, AdminUsername, tempBox.PrivateIP)

	// Brief pause to ensure filesystem sync
	time.Sleep(2 * time.Second)

	// Now verify snapshot using qemu-img
	verifyCmd := `sudo qemu-img snapshot -l /mnt/userdata/qemu-disks/ubuntu-base.qcow2`
	verifyOutput, verifyErr := sshutil.ExecuteCommandWithOutput(ctx, verifyCmd, AdminUsername, tempBox.PrivateIP)

	if verifyErr != nil {
		return fmt.Errorf("failed to verify QEMU snapshot with qemu-img: %w", verifyErr)
	}

	// Check if output contains our snapshot
	if !strings.Contains(verifyOutput, "ssh-ready") {
		slog.Error("Snapshot 'ssh-ready' not found in qcow2 file", "output", verifyOutput)
		return fmt.Errorf("snapshot 'ssh-ready' was not saved to qcow2 file")
	}

	slog.Info("QEMU snapshot verified successfully", "snapshots", verifyOutput)
	return nil
}

func createSnapshotsFromVM(ctx context.Context, clients *AzureClients, resourceGroupName, dataSnapshotName, osSnapshotName string, tempBox *tempBoxInfo) (*GoldenSnapshotInfo, error) {
	osImageName := fmt.Sprintf("%s-image", osSnapshotName)
	slog.Info("Creating data snapshot and OS image", "dataSnapshot", dataSnapshotName, "osImage", osImageName, "dataDiskID", tempBox.DataDiskID, "osDiskID", tempBox.OSDiskID)

	// Create data disk snapshot
	dataSnapshot, err := clients.SnapshotsClient.BeginCreateOrUpdate(ctx, resourceGroupName, dataSnapshotName, armcompute.Snapshot{
		Location: to.Ptr(Location),
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(tempBox.DataDiskID),
			},
		},
		Tags: map[string]*string{
			GoldenTagKeyRole:    to.Ptr(GoldenRoleSnapshot),
			GoldenTagKeyPurpose: to.Ptr("qemu-data-volume"),
			GoldenTagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
			GoldenTagKeyStage:   to.Ptr("ready"),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create data snapshot: %w", err)
	}

	// Wait for data snapshot to complete
	dataResult, err := dataSnapshot.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for data snapshot creation: %w", err)
	}

	// Create a custom VM image directly from the OS disk (no intermediate snapshot needed)
	slog.Info("Creating custom VM image from OS disk", "imageName", osImageName, "osDiskID", tempBox.OSDiskID)

	imageParams := armcompute.Image{
		Location: to.Ptr(Location),
		Properties: &armcompute.ImageProperties{
			StorageProfile: &armcompute.ImageStorageProfile{
				OSDisk: &armcompute.ImageOSDisk{
					OSType:  to.Ptr(armcompute.OperatingSystemTypesLinux),
					OSState: to.Ptr(armcompute.OperatingSystemStateTypesGeneralized),
					ManagedDisk: &armcompute.SubResource{
						ID: to.Ptr(tempBox.OSDiskID),
					},
				},
			},
		},
		Tags: map[string]*string{
			GoldenTagKeyRole:    to.Ptr(GoldenRoleImage),
			GoldenTagKeyPurpose: to.Ptr("qemu-os-image"),
			GoldenTagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
			GoldenTagKeyStage:   to.Ptr("ready"),
		},
	}

	imagePoller, err := clients.ImagesClient.BeginCreateOrUpdate(ctx, resourceGroupName, osImageName, imageParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create custom image: %w", err)
	}

	imageResult, err := imagePoller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for custom image creation: %w", err)
	}

	return &GoldenSnapshotInfo{
		DataSnapshotName:       *dataResult.Name,
		DataSnapshotResourceID: *dataResult.ID,
		OSImageName:            *imageResult.Name,
		OSImageResourceID:      *imageResult.ID,
		Location:               *dataResult.Location,
		CreatedTime:            *dataResult.Properties.TimeCreated,
		DataSizeGB:             *dataResult.Properties.DiskSizeGB,
		OSSizeGB:               *imageResult.Properties.StorageProfile.OSDisk.DiskSizeGB,
	}, nil
}

func createBoxVMWithDataDisk(ctx context.Context, clients *AzureClients, resourceGroupName, vmName, nicID, dataDiskID, sshPublicKey string) (*armcompute.VirtualMachine, error) {
	initScript, err := generateDataVolumeInitScript(ctx, clients, sshPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate data volume init script: %w", err)
	}

	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(Location),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(VMSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr(VMPublisher),
					Offer:     to.Ptr(VMOffer),
					SKU:       to.Ptr(VMSku),
					Version:   to.Ptr(VMVersion),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(fmt.Sprintf("%s-os", vmName)),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
				DataDisks: []*armcompute.DataDisk{
					{
						Name:         to.Ptr(ExtractDiskNameFromID(dataDiskID)),
						CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesAttach),
						Lun:          to.Ptr[int32](0),
						ManagedDisk: &armcompute.ManagedDiskParameters{
							ID: to.Ptr(dataDiskID),
						},
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(fmt.Sprintf("temp-%s", vmName)),
				AdminUsername: to.Ptr(AdminUsername),
				CustomData:    to.Ptr(initScript),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", AdminUsername)),
								KeyData: to.Ptr(sshPublicKey),
							},
						},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(nicID),
					},
				},
			},
		},
		Tags: map[string]*string{
			GoldenTagKeyRole:    to.Ptr(GoldenRoleTempVM),
			GoldenTagKeyPurpose: to.Ptr("golden-snapshot-creation"),
			GoldenTagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
			GoldenTagKeyStage:   to.Ptr("creating"),
		},
	}

	poller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, resourceGroupName, vmName, vmParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting VM creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating VM: %w", err)
	}

	return &result.VirtualMachine, nil
}

// generateDataVolumeInitScript creates an init script that sets up and starts QEMU VM on the data volume
func generateDataVolumeInitScript(_ context.Context, _ *AzureClients, sshPublicKey string) (string, error) {
	// Use unified QEMU script generation with data volume configuration
	config := QEMUScriptConfig{
		SSHPublicKey:  sshPublicKey,
		WorkingDir:    "/mnt/userdata",
		SSHPort:       BoxSSHPort,
		MountDataDisk: true,
	}

	scriptContent, err := GenerateQEMUInitScript(config)
	if err != nil {
		return "", fmt.Errorf("failed to generate unified QEMU script: %w", err)
	}

	// Return the script as-is - SSH connectivity test is sufficient
	return scriptContent, nil
}

// extractDiskNameFromID extracts the disk name from a full Azure resource ID
func ExtractDiskNameFromID(diskID string) string {
	parts := strings.Split(diskID, "/")
	return parts[len(parts)-1]
}

// extractSuffix extracts the suffix from a resource group name
func ExtractSuffix(resourceGroupName string) string {
	// Assumes resource group name format: "shellbox-<suffix>"
	const prefix = "shellbox-"
	if len(resourceGroupName) > len(prefix) {
		return resourceGroupName[len(prefix):]
	}
	return resourceGroupName
}

// ensureGoldenSnapshotResourceGroup creates the persistent resource group for golden snapshots if it doesn't exist
func ensureGoldenSnapshotResourceGroup(ctx context.Context, clients *AzureClients) error {
	slog.Info("Ensuring persistent resource group exists", "resourceGroup", GoldenSnapshotResourceGroup)

	// Check if resource group already exists
	_, err := clients.ResourceClient.Get(ctx, GoldenSnapshotResourceGroup, nil)
	if err == nil {
		slog.Info("Persistent resource group already exists", "resourceGroup", GoldenSnapshotResourceGroup)
		return nil
	}

	// Create the resource group
	slog.Info("Creating persistent resource group", "resourceGroup", GoldenSnapshotResourceGroup)
	_, err = clients.ResourceClient.CreateOrUpdate(ctx, GoldenSnapshotResourceGroup, armresources.ResourceGroup{
		Location: to.Ptr(Location),
		Tags: map[string]*string{
			GoldenTagKeyPurpose: to.Ptr("golden-snapshots"),
			GoldenTagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
			GoldenTagKeyRole:    to.Ptr("persistent-resource-group"),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create persistent resource group: %w", err)
	}

	slog.Info("Created persistent resource group", "resourceGroup", GoldenSnapshotResourceGroup)
	return nil
}

// generateGoldenSnapshotNames creates content-based names for the golden snapshots
// This allows us to detect when the QEMU configuration changes and new snapshots are needed
func generateGoldenSnapshotNames(sshPublicKey string) (dataSnapshotName, imageName string, err error) {
	// Generate a sample QEMU script to hash its content
	config := QEMUScriptConfig{
		SSHPublicKey:  sshPublicKey,
		WorkingDir:    "/mnt/userdata",
		SSHPort:       BoxSSHPort,
		MountDataDisk: true,
	}

	scriptContent, err := GenerateQEMUInitScript(config)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate script for hashing: %w", err)
	}

	// Hash the script content to create a unique identifier
	hasher := sha256.New()
	hasher.Write([]byte(scriptContent))
	hash := hex.EncodeToString(hasher.Sum(nil))[:12] // Use first 12 chars

	dataSnapshotName = fmt.Sprintf("golden-qemu-data-%s", hash)
	imageName = fmt.Sprintf("golden-qemu-os-%s", hash)

	return dataSnapshotName, imageName, nil
}
