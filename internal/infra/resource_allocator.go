package infra

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// AllocateResourcesForUser finds an existing volume for a user and box, then allocates a new instance for it
func (ra *ResourceAllocator) AllocateResourcesForUser(ctx context.Context, userID, boxName string) (*AllocatedResources, error) {
	// Find existing volume by userID and boxName
	existingVolumes, err := ra.resourceQueries.GetVolumesByUserAndBox(ctx, userID, boxName)
	if err != nil {
		return nil, fmt.Errorf("failed to query existing volumes: %w", err)
	}
	if len(existingVolumes) == 0 {
		return nil, fmt.Errorf("no existing box named '%s' found for user", boxName)
	}
	volume := existingVolumes[0]

	// Find available instance
	freeInstances, err := ra.resourceQueries.GetInstancesByStatus(ctx, ResourceStatusFree)
	if err != nil {
		return nil, fmt.Errorf("failed to query free instances: %w", err)
	}
	if len(freeInstances) == 0 {
		return nil, fmt.Errorf("no free instances available")
	}
	instance := freeInstances[0]

	// Mark instance as connected and set userID
	if err := UpdateInstanceStatusAndUser(ctx, ra.clients, instance.ResourceID, ResourceStatusConnected, userID); err != nil {
		return nil, fmt.Errorf("failed to mark instance as connected: %w", err)
	}

	// Mark volume as attached and set userID and boxName
	if err := UpdateVolumeStatusUserAndBox(ctx, ra.clients, volume.ResourceID, ResourceStatusAttached, userID, boxName); err != nil {
		ra.rollbackInstanceStatus(ctx, instance.ResourceID)
		return nil, fmt.Errorf("failed to mark volume as attached: %w", err)
	}

	// Attach volume to instance
	if err := AttachVolumeToInstance(ctx, ra.clients, instance.ResourceID, volume.ResourceID); err != nil {
		ra.rollbackAllocation(ctx, instance.ResourceID, volume.ResourceID)
		return nil, fmt.Errorf("failed to attach volume to instance: %w", err)
	}

	// Get instance IP and start QEMU
	instanceIP, err := ra.finalizeAllocation(ctx, instance, volume)
	if err != nil {
		ra.rollbackAllocation(ctx, instance.ResourceID, volume.ResourceID)
		return nil, err
	}

	slog.Info("existing resources allocated", "instanceID", instance.ResourceID, "volumeID", volume.ResourceID, "userID", userID, "boxName", boxName)

	return &AllocatedResources{
		InstanceID: instance.ResourceID,
		VolumeID:   volume.ResourceID,
		InstanceIP: instanceIP,
	}, nil
}

// ReserveVolumeForUser reserves a free volume for a user with a specific box name
func (ra *ResourceAllocator) ReserveVolumeForUser(ctx context.Context, userID, boxName string) (string, error) {
	// Find available volume from pool
	freeVolumes, err := ra.resourceQueries.GetVolumesByStatus(ctx, ResourceStatusFree)
	if err != nil {
		return "", fmt.Errorf("failed to query free volumes: %w", err)
	}
	if len(freeVolumes) == 0 {
		return "", fmt.Errorf("no free volumes available - please try again in a few minutes while the system creates more capacity")
	}
	volume := freeVolumes[0]

	// Mark volume as attached and set userID and boxName (reserved for user)
	if err := UpdateVolumeStatusUserAndBox(ctx, ra.clients, volume.ResourceID, ResourceStatusAttached, userID, boxName); err != nil {
		return "", fmt.Errorf("failed to reserve volume: %w", err)
	}

	slog.Info("volume reserved", "volumeID", volume.ResourceID, "userID", userID, "boxName", boxName)
	return volume.ResourceID, nil
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
