package infra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// AllocatedResources represents resources allocated to a user session
type AllocatedResources struct {
	InstanceID string
	VolumeID   string
	InstanceIP string
}

// ResourceAllocator manages dynamic allocation of instances and volumes
type ResourceAllocator struct {
	clients         *AzureClients
	resourceQueries *ResourceGraphQueries
	qemuManager     *QEMUManager
}

// NewResourceAllocator creates a new resource allocator
func NewResourceAllocator(clients *AzureClients, resourceQueries *ResourceGraphQueries) *ResourceAllocator {
	return &ResourceAllocator{
		clients:         clients,
		resourceQueries: resourceQueries,
		qemuManager:     NewQEMUManager(clients),
	}
}

// AllocateResourcesForUser finds and allocates a free instance and volume for a user
func (ra *ResourceAllocator) AllocateResourcesForUser(ctx context.Context, userID string) (*AllocatedResources, error) {
	// Find available resources
	instance, volume, err := ra.findAvailableResources(ctx)
	if err != nil {
		return nil, err
	}

	// Perform allocation steps with rollback on failure
	if err := ra.performAllocation(ctx, instance, volume, userID); err != nil {
		return nil, err
	}

	// Get instance IP and start QEMU
	instanceIP, err := ra.finalizeAllocation(ctx, instance, volume)
	if err != nil {
		ra.rollbackAllocation(ctx, instance.ResourceID, volume.ResourceID)
		return nil, err
	}

	slog.Info("resources allocated", "instanceID", instance.ResourceID, "volumeID", volume.ResourceID, "userID", userID)

	return &AllocatedResources{
		InstanceID: instance.ResourceID,
		VolumeID:   volume.ResourceID,
		InstanceIP: instanceIP,
	}, nil
}

// AllocateResourcesForUserWithBox finds a free instance and creates a new volume from golden snapshot for a user with a specific box name
func (ra *ResourceAllocator) AllocateResourcesForUserWithBox(ctx context.Context, userID, boxName string) (*AllocatedResources, error) {
	// Find available instance
	freeInstances, err := ra.resourceQueries.GetInstancesByStatus(ctx, ResourceStatusFree)
	if err != nil {
		return nil, fmt.Errorf("failed to query free instances: %w", err)
	}
	if len(freeInstances) == 0 {
		return nil, fmt.Errorf("no free instances available")
	}
	instance := freeInstances[0]

	// Create volume from golden snapshot
	volume, err := ra.createVolumeFromGoldenSnapshot(ctx, userID, boxName)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume from golden snapshot: %w", err)
	}

	// Perform allocation steps with rollback on failure
	if err := ra.performAllocationWithBox(ctx, instance, *volume, userID, boxName); err != nil {
		// Clean up the volume we just created
		if deleteErr := DeleteVolume(ctx, ra.clients, ra.clients.ResourceGroupName, volume.Name); deleteErr != nil {
			slog.Warn("Failed to cleanup created volume after allocation failure", "volumeName", volume.Name, "error", deleteErr)
		}
		return nil, err
	}

	// Get instance IP and start QEMU
	instanceIP, err := ra.finalizeAllocation(ctx, instance, ResourceInfo{ResourceID: volume.VolumeID})
	if err != nil {
		ra.rollbackAllocation(ctx, instance.ResourceID, volume.VolumeID)
		return nil, err
	}

	slog.Info("resources allocated with box", "instanceID", instance.ResourceID, "volumeID", volume.VolumeID, "userID", userID, "boxName", boxName)

	return &AllocatedResources{
		InstanceID: instance.ResourceID,
		VolumeID:   volume.VolumeID,
		InstanceIP: instanceIP,
	}, nil
}

// findAvailableResources queries for available instances and volumes
func (ra *ResourceAllocator) findAvailableResources(ctx context.Context) (ResourceInfo, ResourceInfo, error) {
	// Get free instances
	freeInstances, err := ra.resourceQueries.GetInstancesByStatus(ctx, ResourceStatusFree)
	if err != nil {
		return ResourceInfo{}, ResourceInfo{}, fmt.Errorf("failed to query free instances: %w", err)
	}
	if len(freeInstances) == 0 {
		return ResourceInfo{}, ResourceInfo{}, fmt.Errorf("no free instances available")
	}

	// Get free volumes
	freeVolumes, err := ra.resourceQueries.GetVolumesByStatus(ctx, ResourceStatusFree)
	if err != nil {
		return ResourceInfo{}, ResourceInfo{}, fmt.Errorf("failed to query free volumes: %w", err)
	}
	if len(freeVolumes) == 0 {
		return ResourceInfo{}, ResourceInfo{}, fmt.Errorf("no free volumes available")
	}

	return freeInstances[0], freeVolumes[0], nil
}

// performAllocation marks resources as allocated and attaches volume
func (ra *ResourceAllocator) performAllocation(ctx context.Context, instance, volume ResourceInfo, userID string) error {
	// Mark instance as connected and set userID
	if err := UpdateInstanceStatusAndUser(ctx, ra.clients, instance.ResourceID, ResourceStatusConnected, userID); err != nil {
		return fmt.Errorf("failed to mark instance as connected: %w", err)
	}

	// Mark volume as attached and set userID
	if err := UpdateVolumeStatusAndUser(ctx, ra.clients, volume.ResourceID, ResourceStatusAttached, userID); err != nil {
		ra.rollbackInstanceStatus(ctx, instance.ResourceID)
		return fmt.Errorf("failed to mark volume as attached: %w", err)
	}

	// Attach volume to instance
	if err := AttachVolumeToInstance(ctx, ra.clients, instance.ResourceID, volume.ResourceID); err != nil {
		ra.rollbackAllocation(ctx, instance.ResourceID, volume.ResourceID)
		return fmt.Errorf("failed to attach volume to instance: %w", err)
	}

	return nil
}

// finalizeAllocation gets IP and starts QEMU
func (ra *ResourceAllocator) finalizeAllocation(ctx context.Context, instance, volume ResourceInfo) (string, error) {
	// Get instance IP
	instanceIP, err := GetInstancePrivateIP(ctx, ra.clients, instance.ResourceID)
	if err != nil {
		return "", fmt.Errorf("failed to get instance IP: %w", err)
	}

	// Start QEMU with attached volume
	if err := ra.qemuManager.StartQEMUWithVolume(ctx, instanceIP, volume.ResourceID); err != nil {
		return "", fmt.Errorf("failed to start QEMU: %w", err)
	}

	return instanceIP, nil
}

// rollbackInstanceStatus rolls back instance status with error logging
func (ra *ResourceAllocator) rollbackInstanceStatus(ctx context.Context, instanceID string) {
	if rollbackErr := UpdateInstanceStatus(ctx, ra.clients, instanceID, ResourceStatusFree); rollbackErr != nil {
		slog.Warn("Failed to rollback instance status", "error", rollbackErr)
	}
}

// rollbackAllocation rolls back both instance and volume status
func (ra *ResourceAllocator) rollbackAllocation(ctx context.Context, instanceID, volumeID string) {
	ra.rollbackInstanceStatus(ctx, instanceID)
	if rollbackErr := UpdateVolumeStatus(ctx, ra.clients, volumeID, ResourceStatusFree); rollbackErr != nil {
		slog.Warn("Failed to rollback volume status", "error", rollbackErr)
	}
}

// ReleaseResources marks allocated resources as free and stops QEMU
func (ra *ResourceAllocator) ReleaseResources(ctx context.Context, instanceID, volumeID string) error {
	// Get instance IP for QEMU operations
	instanceIP, err := GetInstancePrivateIP(ctx, ra.clients, instanceID)
	if err != nil {
		slog.Warn("Failed to get instance IP for cleanup", "instanceID", instanceID, "error", err)
	} else {
		// Stop QEMU (best effort)
		if err := ra.qemuManager.StopQEMU(ctx, instanceIP); err != nil {
			slog.Warn("Failed to stop QEMU during cleanup", "instanceIP", instanceIP, "error", err)
		}
	}

	// Mark resources as free
	var errs []error

	if err := UpdateInstanceStatus(ctx, ra.clients, instanceID, ResourceStatusFree); err != nil {
		errs = append(errs, fmt.Errorf("failed to free instance: %w", err))
	}

	if err := UpdateVolumeStatus(ctx, ra.clients, volumeID, ResourceStatusFree); err != nil {
		errs = append(errs, fmt.Errorf("failed to free volume: %w", err))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	slog.Info("resources released", "instanceID", instanceID, "volumeID", volumeID)
	return nil
}

// createVolumeFromGoldenSnapshot creates a new volume from the golden snapshot
func (ra *ResourceAllocator) createVolumeFromGoldenSnapshot(ctx context.Context, userID, boxName string) (*VolumeInfo, error) {
	// Get the golden snapshot
	goldenSnapshot, err := CreateGoldenSnapshotIfNotExists(ctx, ra.clients, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to ensure golden snapshot exists: %w", err)
	}

	// Generate unique volume name
	namer := NewResourceNamer(ra.clients.Suffix)
	volumeID := uuid.New().String()
	volumeName := namer.VolumePoolDiskName(volumeID)

	// Create volume tags
	now := time.Now().UTC()
	tags := VolumeTags{
		Role:      ResourceRoleVolume,
		Status:    ResourceStatusFree,
		CreatedAt: now.Format(time.RFC3339),
		LastUsed:  now.Format(time.RFC3339),
		VolumeID:  volumeID,
		UserID:    userID,
		BoxName:   boxName,
	}

	// Create volume from snapshot
	volumeInfo, err := CreateVolumeFromSnapshot(ctx, ra.clients, ra.clients.ResourceGroupName, volumeName, goldenSnapshot.ResourceID, tags)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume from snapshot: %w", err)
	}

	return volumeInfo, nil
}

// performAllocationWithBox marks resources as allocated with box name and attaches volume
func (ra *ResourceAllocator) performAllocationWithBox(ctx context.Context, instance ResourceInfo, volume VolumeInfo, userID, boxName string) error {
	// Mark instance as connected and set userID
	if err := UpdateInstanceStatusAndUser(ctx, ra.clients, instance.ResourceID, ResourceStatusConnected, userID); err != nil {
		return fmt.Errorf("failed to mark instance as connected: %w", err)
	}

	// Mark volume as attached and set userID and boxName
	if err := UpdateVolumeStatusUserAndBox(ctx, ra.clients, volume.VolumeID, ResourceStatusAttached, userID, boxName); err != nil {
		ra.rollbackInstanceStatus(ctx, instance.ResourceID)
		return fmt.Errorf("failed to mark volume as attached: %w", err)
	}

	// Attach volume to instance
	if err := AttachVolumeToInstance(ctx, ra.clients, instance.ResourceID, volume.VolumeID); err != nil {
		ra.rollbackAllocation(ctx, instance.ResourceID, volume.VolumeID)
		return fmt.Errorf("failed to attach volume to instance: %w", err)
	}

	return nil
}
