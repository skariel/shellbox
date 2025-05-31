package infra

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"shellbox/internal/sshutil"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

// GoldenSnapshotInfo contains information about the created golden snapshot
type GoldenSnapshotInfo struct {
	Name         string
	ResourceID   string
	Location     string
	CreatedTime  time.Time
	SizeGB       int32
	SourceDiskID string
}

// CreateGoldenSnapshotIfNotExists creates a golden snapshot containing a pre-configured QEMU environment.
// This snapshot serves as the base for all user volumes, ensuring consistent and fast provisioning.
// The function is idempotent - it will find and return existing snapshots rather than creating duplicates.
func CreateGoldenSnapshotIfNotExists(ctx context.Context, clients *AzureClients, resourceGroupName, location string) (*GoldenSnapshotInfo, error) {
	namer := NewResourceNamer(extractSuffix(resourceGroupName))
	snapshotName := namer.GoldenSnapshotName()

	// Check if golden snapshot already exists
	log.Printf("Checking for existing golden snapshot: %s", snapshotName)
	existing, err := clients.SnapshotsClient.Get(ctx, resourceGroupName, snapshotName, nil)
	if err == nil {
		log.Printf("Found existing golden snapshot: %s", snapshotName)
		return &GoldenSnapshotInfo{
			Name:        *existing.Name,
			ResourceID:  *existing.ID,
			Location:    *existing.Location,
			CreatedTime: *existing.Properties.TimeCreated,
			SizeGB:      *existing.Properties.DiskSizeGB,
		}, nil
	}

	log.Printf("Golden snapshot not found, creating new one: %s", snapshotName)

	// Create temporary box VM with data volume for QEMU setup
	tempBoxName := fmt.Sprintf("temp-golden-%d", time.Now().Unix())
	log.Printf("Creating temporary box VM: %s", tempBoxName)

	tempBox, err := createBoxWithDataVolume(ctx, clients, resourceGroupName, location, tempBoxName)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary box for golden snapshot: %w", err)
	}

	// Wait for the VM to be ready and QEMU setup to complete
	log.Printf("Waiting for QEMU setup to complete on temporary box...")
	if err := waitForQEMUSetup(ctx, clients, tempBox); err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteBox(ctx, clients, resourceGroupName, tempBoxName); cleanupErr != nil {
			log.Printf("Warning: failed to cleanup temporary box during error recovery: %v", cleanupErr)
		}
		return nil, fmt.Errorf("failed waiting for QEMU setup: %w", err)
	}

	// Create snapshot from the data volume
	log.Printf("Creating snapshot from data volume...")
	snapshotInfo, err := createSnapshotFromDataVolume(ctx, clients, resourceGroupName, location, snapshotName, tempBox.DataDiskID)
	if err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteBox(ctx, clients, resourceGroupName, tempBoxName); cleanupErr != nil {
			log.Printf("Warning: failed to cleanup temporary box during error recovery: %v", cleanupErr)
		}
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Cleanup temporary resources
	log.Printf("Cleaning up temporary resources...")
	if err := DeleteBox(ctx, clients, resourceGroupName, tempBoxName); err != nil {
		log.Printf("Warning: failed to cleanup temporary box %s: %v", tempBoxName, err)
		// Don't fail the operation - snapshot was created successfully
	}

	log.Printf("Golden snapshot created successfully: %s", snapshotName)
	return snapshotInfo, nil
}

// tempBoxInfo holds information about a temporary box created for golden snapshot
type tempBoxInfo struct {
	VMName     string
	DataDiskID string
	PrivateIP  string
	PublicIP   string
	NICName    string
	NSGName    string
	DiskName   string
}

// createBoxWithDataVolume creates a temporary box VM with a data volume for QEMU setup
func createBoxWithDataVolume(ctx context.Context, clients *AzureClients, resourceGroupName, location, vmName string) (*tempBoxInfo, error) {
	namer := NewResourceNamer(extractSuffix(resourceGroupName))

	// Create data disk first
	dataDiskName := fmt.Sprintf("%s-data", vmName)
	log.Printf("Creating data disk: %s", dataDiskName)

	dataDisk, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroupName, dataDiskName, armcompute.Disk{
		Location: to.Ptr(location),
		Properties: &armcompute.DiskProperties{
			DiskSizeGB: to.Ptr[int32](DefaultVolumeSizeGB),
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
			},
		},
		Tags: map[string]*string{
			TagKeyRole:    to.Ptr(ResourceRoleVolume),
			TagKeyStatus:  to.Ptr("temp"),
			TagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create data disk: %w", err)
	}

	diskResult, err := dataDisk.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for data disk creation: %w", err)
	}

	// Use existing box creation functions but with a custom boxID
	boxID := vmName // Use vmName as boxID for temp box
	nsgName := namer.BoxNSGName(boxID)
	nicName := namer.BoxNICName(boxID)

	// Create NSG using existing function
	nsgResult, err := createBoxNSG(ctx, clients, nsgName)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG: %w", err)
	}

	// Create NIC using existing function
	nicResult, err := createBoxNIC(ctx, clients, nicName, nsgResult.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create NIC: %w", err)
	}

	// Load SSH key for the temporary VM
	_, sshPublicKey, err := sshutil.LoadKeyPair(BastionSSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	// Create VM with data disk attached using modified function
	_, err = createBoxVMWithDataDisk(ctx, clients, resourceGroupName, location, vmName, *nicResult.ID, *diskResult.ID, sshPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	return &tempBoxInfo{
		VMName:     vmName,
		DataDiskID: *diskResult.ID,
		PrivateIP:  *nicResult.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
		NICName:    nicName,
		NSGName:    nsgName,
		DiskName:   dataDiskName,
	}, nil
}

// waitForQEMUSetup waits for the VM to be accessible via SSH and QEMU setup to complete
func waitForQEMUSetup(ctx context.Context, _ *AzureClients, tempBox *tempBoxInfo) error {
	log.Printf("Waiting for QEMU setup to complete on %s (%s)...", tempBox.VMName, tempBox.PrivateIP)

	// Use existing retry pattern to verify QEMU setup completion
	return RetryOperation(ctx, func(ctx context.Context) error {
		// Check if the QEMU setup completion marker file exists
		err := sshutil.ExecuteCommand(ctx,
			"test -f /mnt/userdata/qemu-setup-complete",
			AdminUsername,
			tempBox.PrivateIP)
		if err != nil {
			return fmt.Errorf("QEMU setup not yet complete: %w", err)
		}
		log.Printf("QEMU setup verified complete on %s", tempBox.VMName)
		return nil
	}, 10*time.Minute, 30*time.Second, "QEMU setup completion")
}

// createSnapshotFromDataVolume creates a snapshot from the specified data volume
func createSnapshotFromDataVolume(ctx context.Context, clients *AzureClients, resourceGroupName, location, snapshotName, dataDiskID string) (*GoldenSnapshotInfo, error) {
	log.Printf("Creating snapshot %s from disk %s", snapshotName, dataDiskID)

	snapshot, err := clients.SnapshotsClient.BeginCreateOrUpdate(ctx, resourceGroupName, snapshotName, armcompute.Snapshot{
		Location: to.Ptr(location),
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(dataDiskID),
			},
		},
		Tags: map[string]*string{
			TagKeyRole:    to.Ptr("golden-snapshot"),
			TagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	result, err := snapshot.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed waiting for snapshot creation: %w", err)
	}

	return &GoldenSnapshotInfo{
		Name:         *result.Name,
		ResourceID:   *result.ID,
		Location:     *result.Location,
		CreatedTime:  *result.Properties.TimeCreated,
		SizeGB:       *result.Properties.DiskSizeGB,
		SourceDiskID: dataDiskID,
	}, nil
}

// createBoxVMWithDataDisk creates a VM with both OS and data disks attached
func createBoxVMWithDataDisk(ctx context.Context, clients *AzureClients, resourceGroupName, location, vmName, nicID, dataDiskID, sshPublicKey string) (*armcompute.VirtualMachine, error) {
	// Generate initialization script for data volume setup
	initScript, err := generateDataVolumeInitScript()
	if err != nil {
		return nil, fmt.Errorf("failed to generate data volume init script: %w", err)
	}

	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(location),
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
						Name:         to.Ptr(extractDiskNameFromID(dataDiskID)),
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
			TagKeyRole:    to.Ptr("temp"),
			TagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
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

// generateDataVolumeInitScript creates an init script that sets up QEMU on the data volume
func generateDataVolumeInitScript() (string, error) {
	script := `#!/bin/bash

echo "\$nrconf{restart} = 'a';" | sudo tee /etc/needrestart/conf.d/50-autorestart.conf

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

# Install QEMU and dependencies
sudo apt update
sudo apt install qemu-utils qemu-system-x86 qemu-kvm qemu-system libvirt-daemon-system libvirt-clients bridge-utils genisoimage whois libguestfs-tools -y

sudo usermod -aG kvm,libvirt $USER
sudo systemctl enable --now libvirtd

# Create QEMU environment on data volume
sudo mkdir -p /mnt/userdata/qemu-disks /mnt/userdata/qemu-memory
sudo chown -R $USER:$USER /mnt/userdata/

# Download and prepare Ubuntu image on data volume
cd /mnt/userdata/
wget https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
cp ubuntu-24.04-server-cloudimg-amd64.img qemu-disks/ubuntu-base.qcow2
qemu-img resize qemu-disks/ubuntu-base.qcow2 16G

# Mark setup complete
touch /mnt/userdata/qemu-setup-complete
`

	return base64.StdEncoding.EncodeToString([]byte(script)), nil
}

// extractDiskNameFromID extracts the disk name from a full Azure resource ID
func extractDiskNameFromID(diskID string) string {
	parts := strings.Split(diskID, "/")
	return parts[len(parts)-1]
}

// extractSuffix extracts the suffix from a resource group name
func extractSuffix(resourceGroupName string) string {
	// Assumes resource group name format: "shellbox-<suffix>"
	const prefix = "shellbox-"
	if len(resourceGroupName) > len(prefix) {
		return resourceGroupName[len(prefix):]
	}
	return resourceGroupName
}
