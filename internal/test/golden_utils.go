package test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"testing"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v7"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/google/uuid"
)

// GoldenConfig holds configuration information for golden tests
type GoldenConfig struct {
	ResourceGroupName string
	Suffix            string
	Location          string
}

// SetupTest creates test environment and returns clients, config, and cleanup function
func SetupTest(t testing.TB, category Category) (*infra.AzureClients, *GoldenConfig, func()) {
	env := SetupTestEnvironment(t.(*testing.T), category)

	testConfig := &GoldenConfig{
		ResourceGroupName: env.ResourceGroupName,
		Suffix:            env.Suffix,
		Location:          env.Config.Location,
	}

	return env.Clients, testConfig, env.Cleanup
}

// DecodeBase64Script decodes a base64-encoded script and returns the content
func DecodeBase64Script(script string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(script)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 script: %w", err)
	}
	return string(decoded), nil
}

// GenerateSnapshotName generates a content-based snapshot name for testing
func GenerateSnapshotName() (string, error) {
	// Since generateGoldenSnapshotName is not exported, we'll implement a test version
	config := infra.QEMUScriptConfig{
		SSHPublicKey:  "sample-key-for-hashing",
		WorkingDir:    "/mnt/userdata",
		SSHPort:       infra.BoxSSHPort,
		MountDataDisk: true,
	}

	scriptContent, err := infra.GenerateQEMUInitScript(config)
	if err != nil {
		return "", fmt.Errorf("failed to generate script for hashing: %w", err)
	}

	// Hash the script content to create a unique identifier
	hasher := sha256.New()
	hasher.Write([]byte(scriptContent))
	hash := hex.EncodeToString(hasher.Sum(nil))[:12] // Use first 12 chars

	return fmt.Sprintf("golden-qemu-%s", hash), nil
}

// EnsureGoldenResourceGroup ensures the golden snapshot resource group exists
func EnsureGoldenResourceGroup(ctx context.Context, clients *infra.AzureClients) error {
	// Implement the same logic as the unexported function
	slog.Info("Ensuring persistent resource group exists", "resourceGroup", infra.GoldenSnapshotResourceGroup)

	// Check if resource group already exists
	_, err := clients.ResourceClient.Get(ctx, infra.GoldenSnapshotResourceGroup, nil)
	if err == nil {
		slog.Info("Persistent resource group already exists", "resourceGroup", infra.GoldenSnapshotResourceGroup)
		return nil
	}

	// Create the resource group
	slog.Info("Creating persistent resource group", "resourceGroup", infra.GoldenSnapshotResourceGroup)
	_, err = clients.ResourceClient.CreateOrUpdate(ctx, infra.GoldenSnapshotResourceGroup, armresources.ResourceGroup{
		Location: to.Ptr(infra.Location),
		Tags: map[string]*string{
			infra.GoldenTagKeyPurpose: to.Ptr("golden-snapshots"),
			infra.GoldenTagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
			infra.GoldenTagKeyRole:    to.Ptr("persistent-resource-group"),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create persistent resource group: %w", err)
	}

	slog.Info("Created persistent resource group", "resourceGroup", infra.GoldenSnapshotResourceGroup)
	return nil
}

// TempBoxInfo holds information about a temporary box created for golden snapshot
type TempBoxInfo struct {
	VMName     string
	DataDiskID string
	PrivateIP  string
	PublicIP   string
	NICName    string
	NSGName    string
	DiskName   string
}

// CreateTempBox creates a temporary VM for golden snapshot preparation
func CreateTempBox(ctx context.Context, clients *infra.AzureClients, resourceGroup, vmName string) (*TempBoxInfo, error) {
	// Implement simplified version for testing
	namer := infra.NewResourceNamer("test")

	// Create data volume
	dataDiskName := fmt.Sprintf("%s-data", vmName)
	now := time.Now().UTC()

	diskParams := armcompute.Disk{
		Location: to.Ptr(infra.Location),
		Properties: &armcompute.DiskProperties{
			DiskSizeGB: to.Ptr(int32(infra.DefaultVolumeSizeGB)),
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
			},
		},
		Tags: map[string]*string{
			infra.GoldenTagKeyRole:    to.Ptr("temp-data-disk"),
			infra.GoldenTagKeyPurpose: to.Ptr("golden-snapshot-creation"),
			infra.GoldenTagKeyCreated: to.Ptr(now.Format(time.RFC3339)),
			infra.GoldenTagKeyStage:   to.Ptr("creating"),
		},
	}

	poller, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroup, dataDiskName, diskParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start data volume creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create data volume: %w", err)
	}

	// Create instance for temp box (simplified)
	instance, err := CreateTestInstance(ctx, clients, resourceGroup, vmName)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp box instance: %w", err)
	}

	return &TempBoxInfo{
		VMName:     vmName,
		DataDiskID: *result.ID,
		PrivateIP:  instance.PrivateIP,
		NICName:    namer.BoxNICName(vmName),
		NSGName:    namer.BoxNSGName(vmName),
		DiskName:   dataDiskName,
	}, nil
}

// CreateTestVolume creates a test volume with the specified size
func CreateTestVolume(ctx context.Context, clients *infra.AzureClients, resourceGroup, volumeName string, sizeGB int32) (*infra.VolumeInfo, error) {
	tags := infra.VolumeTags{
		Role:      "test-volume",
		Status:    "free",
		VolumeID:  uuid.New().String(),
		CreatedAt: time.Now().Format(time.RFC3339),
		LastUsed:  time.Now().Format(time.RFC3339),
	}

	diskParams := armcompute.Disk{
		Location: to.Ptr(infra.Location),
		Properties: &armcompute.DiskProperties{
			DiskSizeGB: to.Ptr(sizeGB),
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
			},
		},
		Tags: volumeTagsToMap(tags),
	}

	poller, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroup, volumeName, diskParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start volume creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	return &infra.VolumeInfo{
		Name:       *result.Name,
		ResourceID: *result.ID,
		Location:   *result.Location,
		SizeGB:     *result.Properties.DiskSizeGB,
		VolumeID:   tags.VolumeID,
		Tags:       tags,
	}, nil
}

// CreateSnapshotFromVolume creates a snapshot from an existing volume
func CreateSnapshotFromVolume(ctx context.Context, clients *infra.AzureClients, resourceGroup, snapshotName, sourceResourceID string) (*infra.GoldenSnapshotInfo, error) {
	snapshotParams := armcompute.Snapshot{
		Location: to.Ptr(infra.Location),
		Properties: &armcompute.SnapshotProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(sourceResourceID),
			},
		},
		Tags: map[string]*string{
			infra.GoldenTagKeyRole:    to.Ptr("test-snapshot"),
			infra.GoldenTagKeyPurpose: to.Ptr("test-snapshot-creation"),
			infra.GoldenTagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
			infra.GoldenTagKeyStage:   to.Ptr("ready"),
		},
	}

	poller, err := clients.SnapshotsClient.BeginCreateOrUpdate(ctx, resourceGroup, snapshotName, snapshotParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start snapshot creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w", err)
	}

	return &infra.GoldenSnapshotInfo{
		Name:         *result.Name,
		ResourceID:   *result.ID,
		Location:     *result.Location,
		CreatedTime:  *result.Properties.TimeCreated,
		SizeGB:       *result.Properties.DiskSizeGB,
		SourceDiskID: sourceResourceID,
	}, nil
}

// WaitForQEMUSSH waits for SSH to become available on a QEMU instance
func WaitForQEMUSSH(ctx context.Context, instanceIP string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Test SSH connectivity
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "ConnectTimeout=5",
			"-o", "StrictHostKeyChecking=no",
			"-o", "BatchMode=yes",
			"-p", fmt.Sprintf("%d", port),
			fmt.Sprintf("ubuntu@%s", instanceIP),
			"echo 'SSH test'")

		if err := cmd.Run(); err == nil {
			return nil // SSH is working
		}

		// Wait before retrying
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("timeout waiting for SSH on %s:%d", instanceIP, port)
}

// ExecuteQEMUCommand executes a command on a QEMU instance via SSH
func ExecuteQEMUCommand(ctx context.Context, instanceIP string, port int, command string) error {
	sshCmd := exec.CommandContext(ctx, "ssh",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("ubuntu@%s", instanceIP),
		command)

	if err := sshCmd.Run(); err != nil {
		return fmt.Errorf("failed to execute command on QEMU: %w", err)
	}

	return nil
}

// ExecuteQEMUCommandWithOutput executes a command on a QEMU instance and returns output
func ExecuteQEMUCommandWithOutput(ctx context.Context, instanceIP string, port int, command string) (string, error) {
	sshCmd := exec.CommandContext(ctx, "ssh",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("ubuntu@%s", instanceIP),
		command)

	output, err := sshCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to execute command on QEMU: %w", err)
	}

	return string(output), nil
}

// GenerateTestResourceName generates a unique test resource name with a prefix
func GenerateTestResourceName(prefix string) string {
	// Generate random suffix
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		slog.Warn("Failed to generate random bytes, using timestamp", "error", err)
		return fmt.Sprintf("%s-%d", prefix, time.Now().Unix())
	}

	// Convert to hex and ensure valid Azure resource name
	suffix := fmt.Sprintf("%x", bytes)
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

// CreateTestInstance creates a test VM instance
func CreateTestInstance(ctx context.Context, clients *infra.AzureClients, resourceGroup, instanceName string) (*InstanceInfo, error) {
	// Use the existing instance creation logic but simplified for testing
	namer := infra.NewResourceNamer("test")

	// Create NSG for the instance
	nsgName := namer.BoxNSGName(instanceName)
	nsgResult, err := createTestInstanceNSG(ctx, clients, nsgName)
	if err != nil {
		return nil, fmt.Errorf("failed to create NSG: %w", err)
	}

	// Create NIC for the instance
	nicName := namer.BoxNICName(instanceName)
	nicResult, err := createTestInstanceNIC(ctx, clients, nicName, nsgResult.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create NIC: %w", err)
	}

	// Load SSH key
	_, sshPublicKey, err := sshutil.LoadKeyPair(infra.BastionSSHKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	// Create VM
	vmParams := armcompute.VirtualMachine{
		Location: to.Ptr(infra.Location),
		Properties: &armcompute.VirtualMachineProperties{
			HardwareProfile: &armcompute.HardwareProfile{
				VMSize: to.Ptr(armcompute.VirtualMachineSizeTypes(infra.VMSize)),
			},
			StorageProfile: &armcompute.StorageProfile{
				ImageReference: &armcompute.ImageReference{
					Publisher: to.Ptr(infra.VMPublisher),
					Offer:     to.Ptr(infra.VMOffer),
					SKU:       to.Ptr(infra.VMSku),
					Version:   to.Ptr(infra.VMVersion),
				},
				OSDisk: &armcompute.OSDisk{
					Name:         to.Ptr(fmt.Sprintf("%s-os", instanceName)),
					CreateOption: to.Ptr(armcompute.DiskCreateOptionTypesFromImage),
					ManagedDisk: &armcompute.ManagedDiskParameters{
						StorageAccountType: to.Ptr(armcompute.StorageAccountTypesPremiumLRS),
					},
				},
			},
			OSProfile: &armcompute.OSProfile{
				ComputerName:  to.Ptr(instanceName),
				AdminUsername: to.Ptr(infra.AdminUsername),
				LinuxConfiguration: &armcompute.LinuxConfiguration{
					DisablePasswordAuthentication: to.Ptr(true),
					SSH: &armcompute.SSHConfiguration{
						PublicKeys: []*armcompute.SSHPublicKey{
							{
								Path:    to.Ptr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", infra.AdminUsername)),
								KeyData: to.Ptr(sshPublicKey),
							},
						},
					},
				},
			},
			NetworkProfile: &armcompute.NetworkProfile{
				NetworkInterfaces: []*armcompute.NetworkInterfaceReference{
					{
						ID: to.Ptr(*nicResult.ID),
					},
				},
			},
		},
		Tags: map[string]*string{
			infra.TagKeyRole:    to.Ptr("test-instance"),
			infra.TagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}

	poller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, resourceGroup, instanceName, vmParams, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start VM creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM: %w", err)
	}

	return &InstanceInfo{
		Name:       *result.Name,
		ResourceID: *result.ID,
		Location:   *result.Location,
		PrivateIP:  *nicResult.Properties.IPConfigurations[0].Properties.PrivateIPAddress,
		Status:     "free",
	}, nil
}

// InstanceInfo contains information about a created instance (for testing)
type InstanceInfo struct {
	Name       string
	ResourceID string
	Location   string
	PrivateIP  string
	Status     string
}

// volumeTagsToMap converts VolumeTags struct to Azure tags map format
func volumeTagsToMap(tags infra.VolumeTags) map[string]*string {
	return map[string]*string{
		infra.TagKeyRole:     to.Ptr(tags.Role),
		infra.TagKeyStatus:   to.Ptr(tags.Status),
		infra.TagKeyCreated:  to.Ptr(tags.CreatedAt),
		infra.TagKeyLastUsed: to.Ptr(tags.LastUsed),
		"volume_id":          to.Ptr(tags.VolumeID),
	}
}

// createTestInstanceNSG creates a simplified NSG for testing
func createTestInstanceNSG(ctx context.Context, clients *infra.AzureClients, nsgName string) (*armnetwork.SecurityGroup, error) {
	nsgParams := armnetwork.SecurityGroup{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: []*armnetwork.SecurityRule{
				{
					Name: to.Ptr("AllowSSH"),
					Properties: &armnetwork.SecurityRulePropertiesFormat{
						Protocol:                 to.Ptr(armnetwork.SecurityRuleProtocolTCP),
						SourceAddressPrefix:      to.Ptr("*"),
						SourcePortRange:          to.Ptr("*"),
						DestinationAddressPrefix: to.Ptr("*"),
						DestinationPortRange:     to.Ptr("22"),
						Access:                   to.Ptr(armnetwork.SecurityRuleAccessAllow),
						Priority:                 to.Ptr(int32(1000)),
						Direction:                to.Ptr(armnetwork.SecurityRuleDirectionInbound),
					},
				},
			},
		},
		Tags: map[string]*string{
			infra.TagKeyRole:    to.Ptr("test-nsg"),
			infra.TagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}

	poller, err := clients.NSGClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, nsgName, nsgParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting NSG creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating NSG: %w", err)
	}

	return &result.SecurityGroup, nil
}

// createTestInstanceNIC creates a simplified NIC for testing
func createTestInstanceNIC(ctx context.Context, clients *infra.AzureClients, nicName string, nsgID *string) (*armnetwork.Interface, error) {
	nicParams := armnetwork.Interface{
		Location: to.Ptr(infra.Location),
		Properties: &armnetwork.InterfacePropertiesFormat{
			NetworkSecurityGroup: &armnetwork.SecurityGroup{
				ID: nsgID,
			},
			IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
				{
					Name: to.Ptr("ipconfig1"),
					Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
						Subnet: &armnetwork.Subnet{
							ID: to.Ptr(clients.BoxesSubnetID),
						},
						PrivateIPAllocationMethod: to.Ptr(armnetwork.IPAllocationMethodDynamic),
					},
				},
			},
		},
		Tags: map[string]*string{
			infra.TagKeyRole:    to.Ptr("test-nic"),
			infra.TagKeyCreated: to.Ptr(time.Now().Format(time.RFC3339)),
		},
	}

	poller, err := clients.NICClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, nicName, nicParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting NIC creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating NIC: %w", err)
	}

	return &result.Interface, nil
}

// GenerateQEMUResumeCommand generates a QEMU resume command for testing
func GenerateQEMUResumeCommand(workingDir string, port int) string {
	return fmt.Sprintf(`
# Mount data disk if not already mounted
if ! mountpoint -q /mnt/userdata; then
    sudo mkdir -p /mnt/userdata
    sudo mount /dev/disk/azure/scsi1/lun0 /mnt/userdata
fi

# Resume QEMU VM from saved state
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
   -monitor unix:/tmp/qemu-monitor.sock,server,nowait \
   -nic user,model=virtio,hostfwd=tcp::%d-:22,dns=8.8.8.8 \
   -loadvm ssh-ready &
`, workingDir, workingDir, workingDir, port)
}

// GenerateQEMUSaveStateCommand generates a QEMU save state command for testing
func GenerateQEMUSaveStateCommand(stateName string) string {
	return fmt.Sprintf(`echo -e "savevm %s\nquit" | sudo socat - UNIX-CONNECT:/tmp/qemu-monitor.sock`, stateName)
}

// GenerateQEMULoadStateCommand generates a QEMU load state command for testing
func GenerateQEMULoadStateCommand(workingDir string, port int, stateName string) string {
	return fmt.Sprintf(`
# Mount data disk if not already mounted
if ! mountpoint -q /mnt/userdata; then
    sudo mkdir -p /mnt/userdata
    sudo mount /dev/disk/azure/scsi1/lun0 /mnt/userdata
fi

# Resume QEMU VM from specified state
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
   -monitor unix:/tmp/qemu-monitor.sock,server,nowait \
   -nic user,model=virtio,hostfwd=tcp::%d-:22,dns=8.8.8.8 \
   -loadvm %s &
`, workingDir, workingDir, workingDir, port, stateName)
}

// MockQEMUManager provides test-only QEMU operations for golden tests
type MockQEMUManager struct {
	clients *infra.AzureClients
}

// NewMockQEMUManager creates a mock QEMU manager for testing
func NewMockQEMUManager(clients *infra.AzureClients) *MockQEMUManager {
	return &MockQEMUManager{
		clients: clients,
	}
}

// SaveState simulates saving QEMU state for testing
func (m *MockQEMUManager) SaveState(_ context.Context, instanceIP, stateName string) error {
	// For testing, we'll simulate the save command
	saveCmd := GenerateQEMUSaveStateCommand(stateName)
	slog.Debug("Simulating QEMU save state", "stateName", stateName, "instanceIP", instanceIP)
	slog.Debug("QEMU save command", "command", saveCmd)

	// In a real implementation, this would execute the command via SSH
	// For testing, we'll just validate the parameters and return success/error
	if instanceIP == "" || stateName == "" {
		return fmt.Errorf("invalid parameters: instanceIP=%s, stateName=%s", instanceIP, stateName)
	}

	// Simulate errors for specific test scenarios
	if instanceIP == "10.0.0.1" && stateName == "test-state" {
		return fmt.Errorf("QEMU not running on instance %s", instanceIP)
	}

	return nil
}

// StartQEMUWithVolume simulates starting QEMU for testing
func (m *MockQEMUManager) StartQEMUWithVolume(_ context.Context, instanceIP, volumeID string) error {
	slog.Debug("Simulating QEMU start with volume", "volumeID", volumeID, "instanceIP", instanceIP)

	if instanceIP == "" || volumeID == "" {
		return fmt.Errorf("invalid parameters: instanceIP=%s, volumeID=%s", instanceIP, volumeID)
	}

	return nil
}

// StopQEMU simulates stopping QEMU for testing
func (m *MockQEMUManager) StopQEMU(_ context.Context, instanceIP string) error {
	slog.Debug("Simulating QEMU stop", "instanceIP", instanceIP)

	if instanceIP == "" {
		return fmt.Errorf("invalid instanceIP: %s", instanceIP)
	}

	return nil
}

// DetachVolumeFromInstance detaches a volume from an instance for testing
func DetachVolumeFromInstance(ctx context.Context, clients *infra.AzureClients, instanceID, volumeID string) error {
	// Extract instance name from resource ID
	parts := strings.Split(instanceID, "/")
	instanceName := parts[len(parts)-1]

	parts = strings.Split(volumeID, "/")
	resourceGroupName := ""
	for i, part := range parts {
		if part == "resourceGroups" && i+1 < len(parts) {
			resourceGroupName = parts[i+1]
			break
		}
	}

	// Get current VM configuration
	vm, err := clients.ComputeClient.Get(ctx, resourceGroupName, instanceName, nil)
	if err != nil {
		return fmt.Errorf("failed to get VM for volume detachment: %w", err)
	}

	// Remove the specified data disk
	var newDataDisks []*armcompute.DataDisk
	for _, disk := range vm.Properties.StorageProfile.DataDisks {
		if disk.ManagedDisk != nil && disk.ManagedDisk.ID != nil && *disk.ManagedDisk.ID != volumeID {
			newDataDisks = append(newDataDisks, disk)
		}
	}

	// Update VM with new data disk configuration
	vm.Properties.StorageProfile.DataDisks = newDataDisks

	poller, err := clients.ComputeClient.BeginCreateOrUpdate(ctx, resourceGroupName, instanceName, vm.VirtualMachine, nil)
	if err != nil {
		return fmt.Errorf("failed to start VM update for volume detachment: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, &infra.DefaultPollOptions)
	if err != nil {
		return fmt.Errorf("failed to detach volume from VM: %w", err)
	}

	return nil
}
