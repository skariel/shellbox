package infra

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
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
sudo apt install qemu-utils qemu-system-x86 qemu-kvm qemu-system libvirt-daemon-system libvirt-clients bridge-utils genisoimage whois libguestfs-tools -y

sudo usermod -aG kvm,libvirt $USER
sudo systemctl enable --now libvirtd

# Create QEMU environment
mkdir -p %s/qemu-disks %s/qemu-memory
%s
# Download and prepare Ubuntu image
cd %s/
wget https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
cp ubuntu-24.04-server-cloudimg-amd64.img qemu-disks/ubuntu-base.qcow2
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

# Start QEMU VM with SSH-ready configuration
sudo qemu-system-x86_64 \
   -enable-kvm \
   -m 24G \
   -mem-prealloc \
   -mem-path %s/qemu-memory/ubuntu-mem \
   -smp 8 \
   -cpu host \
   -drive file=%s/qemu-disks/ubuntu-base.qcow2,format=qcow2 \
   -drive file=%s/qemu-disks/cloud-init.iso,format=raw \
   -nographic \
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
// Golden snapshots are stored in a persistent resource group to avoid recreation between deployments.
func CreateGoldenSnapshotIfNotExists(ctx context.Context, clients *AzureClients, resourceGroupName, _ string) (*GoldenSnapshotInfo, error) {
	// Ensure the persistent resource group exists
	if err := ensureGoldenSnapshotResourceGroup(ctx, clients); err != nil {
		return nil, fmt.Errorf("failed to ensure golden snapshot resource group: %w", err)
	}

	// Generate content-based snapshot name for this QEMU configuration
	snapshotName, err := generateGoldenSnapshotName()
	if err != nil {
		return nil, fmt.Errorf("failed to generate snapshot name: %w", err)
	}

	// Check if golden snapshot already exists in the persistent resource group
	log.Printf("Checking for existing golden snapshot: %s in %s", snapshotName, GoldenSnapshotResourceGroup)
	existing, err := clients.SnapshotsClient.Get(ctx, GoldenSnapshotResourceGroup, snapshotName, nil)
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

	tempBox, err := createBoxWithDataVolume(ctx, clients, resourceGroupName, tempBoxName)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary box for golden snapshot: %w", err)
	}

	// Wait for the VM to be ready and QEMU setup to complete
	log.Printf("Waiting for QEMU setup to complete on temporary box...")
	if err := waitForQEMUSetup(ctx, clients, tempBox); err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteInstance(ctx, clients, resourceGroupName, tempBoxName); cleanupErr != nil {
			log.Printf("Warning: failed to cleanup temporary box during error recovery: %v", cleanupErr)
		}
		return nil, fmt.Errorf("failed waiting for QEMU setup: %w", err)
	}

	// Create snapshot from the data volume in the persistent resource group
	log.Printf("Creating snapshot from data volume...")
	snapshotInfo, err := createSnapshotFromDataVolume(ctx, clients, GoldenSnapshotResourceGroup, snapshotName, tempBox.DataDiskID)
	if err != nil {
		// Cleanup temp resources on failure
		if cleanupErr := DeleteInstance(ctx, clients, resourceGroupName, tempBoxName); cleanupErr != nil {
			log.Printf("Warning: failed to cleanup temporary box during error recovery: %v", cleanupErr)
		}
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	// Cleanup temporary resources
	log.Printf("Cleaning up temporary resources...")
	if err := DeleteInstance(ctx, clients, resourceGroupName, tempBoxName); err != nil {
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
func createBoxWithDataVolume(ctx context.Context, clients *AzureClients, resourceGroupName, vmName string) (*tempBoxInfo, error) {
	namer := NewResourceNamer(extractSuffix(resourceGroupName))

	// Create data disk first
	dataDiskName := fmt.Sprintf("%s-data", vmName)
	log.Printf("Creating data disk: %s", dataDiskName)

	dataDisk, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroupName, dataDiskName, armcompute.Disk{
		Location: to.Ptr(Location),
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
	nsgResult, err := createInstanceNSG(ctx, clients, nsgName)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG: %w", err)
	}

	// Create NIC using existing function
	nicResult, err := createInstanceNIC(ctx, clients, nicName, nsgResult.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create NIC: %w", err)
	}

	// Load SSH key for the temporary VM
	_, sshPublicKey, err := sshutil.LoadKeyPair(BastionSSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	// Create VM with data disk attached using modified function
	_, err = createBoxVMWithDataDisk(ctx, clients, resourceGroupName, vmName, *nicResult.ID, *diskResult.ID, sshPublicKey)
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

// waitForQEMUSetup waits for the QEMU VM to be accessible via SSH on port 2222
func waitForQEMUSetup(ctx context.Context, _ *AzureClients, tempBox *tempBoxInfo) error {
	log.Printf("Waiting for QEMU VM to be SSH-ready on %s (%s)...", tempBox.VMName, tempBox.PrivateIP)

	// Test SSH connectivity to the QEMU VM - this is the definitive test
	log.Printf("Testing SSH connectivity to QEMU VM on port %d...", BoxSSHPort)
	return RetryOperation(ctx, func(ctx context.Context) error {
		// Test SSH connection directly to the QEMU VM from bastion
		// We need to execute this test from the bastion, not from within the instance
		// Since sshutil.ExecuteCommand is for remote execution, let's execute locally
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "ConnectTimeout=5",
			"-o", "StrictHostKeyChecking=no",
			"-p", fmt.Sprintf("%d", BoxSSHPort),
			fmt.Sprintf("ubuntu@%s", tempBox.PrivateIP),
			"echo 'SSH test successful'")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("QEMU VM SSH not yet ready: %w: %s", err, string(output))
		}

		// Stop the QEMU VM to preserve the SSH-ready state
		log.Printf("QEMU VM SSH confirmed working, stopping VM to preserve state...")
		stopErr := sshutil.ExecuteCommand(ctx,
			"sudo pkill qemu-system-x86_64",
			AdminUsername,
			tempBox.PrivateIP)
		if stopErr != nil {
			log.Printf("Warning: failed to stop QEMU VM: %v", stopErr)
		}

		log.Printf("QEMU VM SSH-ready state prepared on %s", tempBox.VMName)
		return nil
	}, 15*time.Minute, 30*time.Second, "QEMU VM SSH connectivity")
}

// createSnapshotFromDataVolume creates a snapshot from the specified data volume
func createSnapshotFromDataVolume(ctx context.Context, clients *AzureClients, resourceGroupName, snapshotName, dataDiskID string) (*GoldenSnapshotInfo, error) {
	log.Printf("Creating snapshot %s from disk %s", snapshotName, dataDiskID)

	snapshot, err := clients.SnapshotsClient.BeginCreateOrUpdate(ctx, resourceGroupName, snapshotName, armcompute.Snapshot{
		Location: to.Ptr(Location),
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
func createBoxVMWithDataDisk(ctx context.Context, clients *AzureClients, resourceGroupName, vmName, nicID, dataDiskID, sshPublicKey string) (*armcompute.VirtualMachine, error) {
	// Generate initialization script for data volume setup
	initScript, err := generateDataVolumeInitScript()
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

// generateDataVolumeInitScript creates an init script that sets up and starts QEMU VM on the data volume
func generateDataVolumeInitScript() (string, error) {
	// Load SSH key for the golden snapshot QEMU VM
	_, sshPublicKey, err := sshutil.LoadKeyPair(BastionSSHKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to load SSH key: %w", err)
	}

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

// ensureGoldenSnapshotResourceGroup creates the persistent resource group for golden snapshots if it doesn't exist
func ensureGoldenSnapshotResourceGroup(ctx context.Context, clients *AzureClients) error {
	log.Printf("Ensuring persistent resource group exists: %s", GoldenSnapshotResourceGroup)

	// Check if resource group already exists
	_, err := clients.ResourceClient.Get(ctx, GoldenSnapshotResourceGroup, nil)
	if err == nil {
		log.Printf("Persistent resource group already exists: %s", GoldenSnapshotResourceGroup)
		return nil
	}

	// Create the resource group
	log.Printf("Creating persistent resource group: %s", GoldenSnapshotResourceGroup)
	_, err = clients.ResourceClient.CreateOrUpdate(ctx, GoldenSnapshotResourceGroup, armresources.ResourceGroup{
		Location: to.Ptr(Location),
		Tags: map[string]*string{
			"purpose": to.Ptr("golden-snapshots"),
			"created": to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}, nil)

	if err != nil {
		return fmt.Errorf("failed to create persistent resource group: %w", err)
	}

	log.Printf("Created persistent resource group: %s", GoldenSnapshotResourceGroup)
	return nil
}

// generateGoldenSnapshotName creates a content-based name for the golden snapshot
// This allows us to detect when the QEMU configuration changes and a new snapshot is needed
func generateGoldenSnapshotName() (string, error) {
	// Generate a sample QEMU script to hash its content
	config := QEMUScriptConfig{
		SSHPublicKey:  "sample-key-for-hashing",
		WorkingDir:    "/mnt/userdata",
		SSHPort:       BoxSSHPort,
		MountDataDisk: true,
	}

	scriptContent, err := GenerateQEMUInitScript(config)
	if err != nil {
		return "", fmt.Errorf("failed to generate script for hashing: %w", err)
	}

	// Hash the script content to create a unique identifier
	hasher := sha256.New()
	hasher.Write([]byte(scriptContent))
	hash := hex.EncodeToString(hasher.Sum(nil))[:12] // Use first 12 chars

	return fmt.Sprintf("golden-qemu-%s", hash), nil
}
