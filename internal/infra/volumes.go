package infra

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/google/uuid"
)

// VolumeTags represents searchable metadata for volume disks.
// These tags are used to track volume status and lifecycle.
type VolumeTags struct {
	Role      string // volume, temp, golden
	Status    string // free, attached
	CreatedAt string
	LastUsed  string
	VolumeID  string
}

// VolumeConfig represents configuration for creating a volume
type VolumeConfig struct {
	DiskSize int32
}

// VolumeInfo contains information about a created volume
type VolumeInfo struct {
	Name       string
	ResourceID string
	Location   string
	SizeGB     int32
	VolumeID   string
	Tags       VolumeTags
}

// CreateVolume creates a new empty managed disk volume with standard configuration.
// This is a simplified version that uses a VolumeConfig and generates appropriate defaults.
// It returns the volume ID and any error encountered.
func CreateVolume(ctx context.Context, clients *AzureClients, config *VolumeConfig) (string, error) {
	volumeID := uuid.New().String()
	namer := NewResourceNamer(clients.Suffix)
	volumeName := namer.VolumePoolDiskName(volumeID)

	now := time.Now().UTC()
	tags := VolumeTags{
		Role:      ResourceRoleVolume,
		Status:    ResourceStatusFree,
		CreatedAt: now.Format(time.RFC3339),
		LastUsed:  now.Format(time.RFC3339),
		VolumeID:  volumeID,
	}

	_, err := CreateVolumeWithTags(ctx, clients, clients.ResourceGroupName, volumeName, config.DiskSize, tags)
	if err != nil {
		return "", err
	}

	return volumeID, nil
}

// CreateVolumeWithTags creates a new empty managed disk volume with proper tagging.
// This creates a standard empty volume that can be used for temporary purposes
// or as a base for QEMU setup. It returns volume information and any error encountered.
func CreateVolumeWithTags(ctx context.Context, clients *AzureClients, resourceGroupName, volumeName string, sizeGB int32, tags VolumeTags) (*VolumeInfo, error) {
	now := time.Now().UTC()
	if tags.VolumeID == "" {
		tags.VolumeID = uuid.New().String()
	}
	if tags.CreatedAt == "" {
		tags.CreatedAt = now.Format(time.RFC3339)
	}
	if tags.LastUsed == "" {
		tags.LastUsed = now.Format(time.RFC3339)
	}

	slog.Info("Creating volume", "name", volumeName, "sizeGB", sizeGB, "role", tags.Role)

	diskParams := armcompute.Disk{
		Location: to.Ptr(Location),
		Properties: &armcompute.DiskProperties{
			DiskSizeGB: to.Ptr(sizeGB),
			CreationData: &armcompute.CreationData{
				CreateOption: to.Ptr(armcompute.DiskCreateOptionEmpty),
			},
		},
		Tags: volumeTagsToMap(tags),
	}

	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroupName, volumeName, diskParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting volume creation: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating volume: %w", err)
	}

	return &VolumeInfo{
		Name:       *result.Name,
		ResourceID: *result.ID,
		Location:   *result.Location,
		SizeGB:     *result.Properties.DiskSizeGB,
		VolumeID:   tags.VolumeID,
		Tags:       tags,
	}, nil
}

// CreateVolumeFromSnapshot creates a new managed disk volume from an existing snapshot.
// This is used to create user volumes from golden snapshots or restore from backups.
// It returns volume information and any error encountered.
func CreateVolumeFromSnapshot(ctx context.Context, clients *AzureClients, resourceGroupName, volumeName, snapshotID string, tags VolumeTags) (*VolumeInfo, error) {
	now := time.Now().UTC()
	if tags.VolumeID == "" {
		tags.VolumeID = uuid.New().String()
	}
	if tags.CreatedAt == "" {
		tags.CreatedAt = now.Format(time.RFC3339)
	}
	if tags.LastUsed == "" {
		tags.LastUsed = now.Format(time.RFC3339)
	}

	slog.Info("Creating volume from snapshot", "snapshotID", snapshotID, "volumeName", volumeName, "role", tags.Role)

	diskParams := armcompute.Disk{
		Location: to.Ptr(Location),
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: to.Ptr(snapshotID),
			},
		},
		Tags: volumeTagsToMap(tags),
	}

	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.DisksClient.BeginCreateOrUpdate(ctx, resourceGroupName, volumeName, diskParams, nil)
	if err != nil {
		return nil, fmt.Errorf("starting volume creation from snapshot: %w", err)
	}

	result, err := poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return nil, fmt.Errorf("creating volume from snapshot: %w", err)
	}

	return &VolumeInfo{
		Name:       *result.Name,
		ResourceID: *result.ID,
		Location:   *result.Location,
		SizeGB:     *result.Properties.DiskSizeGB,
		VolumeID:   tags.VolumeID,
		Tags:       tags,
	}, nil
}

// DeleteVolume completely removes a managed disk volume.
// This function handles cleanup for temporary volumes, user volumes, or any managed disk.
// It returns an error if the deletion fails.
func DeleteVolume(ctx context.Context, clients *AzureClients, resourceGroupName, volumeName string) error {
	if volumeName == "" {
		slog.Warn("Volume name is empty, skipping deletion")
		return nil
	}

	slog.Info("Deleting volume", "name", volumeName)

	pollOptions := &runtime.PollUntilDoneOptions{
		Frequency: 2 * time.Second,
	}

	poller, err := clients.DisksClient.BeginDelete(ctx, resourceGroupName, volumeName, nil)
	if err != nil {
		return fmt.Errorf("starting volume deletion: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, pollOptions)
	if err != nil {
		return fmt.Errorf("deleting volume: %w", err)
	}

	slog.Info("Successfully deleted volume", "name", volumeName)
	return nil
}

// FindVolumesByRole returns volume names matching the given role tag.
// It filters disks based on their role tag and returns their names for further operations.
// If suffix is provided, it only returns volumes whose names contain that suffix.
func FindVolumesByRole(ctx context.Context, clients *AzureClients, resourceGroupName, role string, suffix ...string) ([]string, error) {
	var volumes []string

	pager := clients.DisksClient.NewListByResourceGroupPager(resourceGroupName, nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing volumes: %w", err)
		}

		for _, disk := range page.Value {
			if disk.Tags != nil {
				if roleTag, exists := disk.Tags[TagKeyRole]; exists && *roleTag == role {
					// If suffix filter is provided, only include volumes with that suffix in the name
					if len(suffix) > 0 && !strings.Contains(*disk.Name, suffix[0]) {
						continue
					}
					volumes = append(volumes, *disk.Name)
				}
			}
		}
	}

	return volumes, nil
}

// volumeTagsToMap converts VolumeTags struct to Azure tags map format
func volumeTagsToMap(tags VolumeTags) map[string]*string {
	return map[string]*string{
		TagKeyRole:     to.Ptr(tags.Role),
		TagKeyStatus:   to.Ptr(tags.Status),
		TagKeyCreated:  to.Ptr(tags.CreatedAt),
		TagKeyLastUsed: to.Ptr(tags.LastUsed),
		"volume_id":    to.Ptr(tags.VolumeID),
	}
}

// UpdateVolumeStatus updates the status tag of a volume
func UpdateVolumeStatus(ctx context.Context, clients *AzureClients, volumeID, status string) error {
	namer := NewResourceNamer(clients.Suffix)
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Get current volume
	volume, err := clients.DisksClient.Get(ctx, clients.ResourceGroupName, volumeName, nil)
	if err != nil {
		return fmt.Errorf("failed to get volume for status update: %w", err)
	}

	// Update status tag
	if volume.Tags == nil {
		volume.Tags = make(map[string]*string)
	}
	volume.Tags[TagKeyStatus] = to.Ptr(status)
	volume.Tags[TagKeyLastUsed] = to.Ptr(time.Now().UTC().Format(time.RFC3339))

	// Update the volume
	poller, err := clients.DisksClient.BeginCreateOrUpdate(ctx, clients.ResourceGroupName, volumeName, volume.Disk, nil)
	if err != nil {
		return fmt.Errorf("failed to start volume status update: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return fmt.Errorf("failed to update volume status: %w", err)
	}

	return nil
}
