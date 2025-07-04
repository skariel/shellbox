package infra

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Pool configuration constants for production
const (
	DefaultMinFreeInstances  = 5
	DefaultMaxFreeInstances  = 10
	DefaultMaxTotalInstances = 100
	DefaultMinFreeVolumes    = 20
	DefaultMaxFreeVolumes    = 50
	DefaultMaxTotalVolumes   = 500
	DefaultCheckInterval     = 1 * time.Minute
	DefaultScaleDownCooldown = 10 * time.Minute
)

// Pool configuration constants for development
const (
	DevMinFreeInstances  = 1
	DevMaxFreeInstances  = 2
	DevMaxTotalInstances = 5
	DevMinFreeVolumes    = 2
	DevMaxFreeVolumes    = 5
	DevMaxTotalVolumes   = 20
	DevCheckInterval     = 30 * time.Second
	DevScaleDownCooldown = 2 * time.Minute
)

// PoolConfig holds configuration for dual pool management (instances + volumes)
type PoolConfig struct {
	// Instance pool settings
	MinFreeInstances  int
	MaxFreeInstances  int
	MaxTotalInstances int

	// Volume pool settings
	MinFreeVolumes  int
	MaxFreeVolumes  int
	MaxTotalVolumes int

	// Timing settings
	CheckInterval     time.Duration
	ScaleDownCooldown time.Duration
}

// NewDevPoolConfig creates a development pool configuration
func NewDevPoolConfig() PoolConfig {
	return PoolConfig{
		MinFreeInstances:  DevMinFreeInstances,
		MaxFreeInstances:  DevMaxFreeInstances,
		MaxTotalInstances: DevMaxTotalInstances,
		MinFreeVolumes:    DevMinFreeVolumes,
		MaxFreeVolumes:    DevMaxFreeVolumes,
		MaxTotalVolumes:   DevMaxTotalVolumes,
		CheckInterval:     DevCheckInterval,
		ScaleDownCooldown: DevScaleDownCooldown,
	}
}

type BoxPool struct {
	mu              sync.RWMutex
	clients         *AzureClients
	vmConfig        *VMConfig
	poolConfig      PoolConfig
	resourceQueries *ResourceGraphQueries
	goldenSnapshot  *GoldenSnapshotInfo
	lastScaleDown   time.Time // Track last scale down to enforce cooldown
}

func NewBoxPool(clients *AzureClients, vmConfig *VMConfig, poolConfig PoolConfig, goldenSnapshot *GoldenSnapshotInfo) *BoxPool {
	resourceQueries := NewResourceGraphQueries(
		clients.ResourceGraphClient,
		clients.SubscriptionID,
		clients.ResourceGroupName,
	)

	return &BoxPool{
		clients:         clients,
		vmConfig:        vmConfig,
		poolConfig:      poolConfig,
		resourceQueries: resourceQueries,
		goldenSnapshot:  goldenSnapshot,
	}
}

func (p *BoxPool) MaintainPool(ctx context.Context) {
	ticker := time.NewTicker(p.poolConfig.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Maintain both instance and volume pools
			p.maintainInstancePool(ctx)
			p.maintainVolumePool(ctx)
		}
	}
}

func (p *BoxPool) maintainInstancePool(ctx context.Context) {
	counts, err := p.resourceQueries.CountInstancesByStatus(ctx)
	if err != nil {
		slog.Error("failed to get instance counts", "error", err)
		return
	}

	slog.Info("instance pool status",
		"free", counts.Free,
		"connected", counts.Connected,
		"total", counts.Total)

	if counts.Free < p.poolConfig.MinFreeInstances {
		p.scaleUpInstances(ctx, counts.Free)
	} else if counts.Free > p.poolConfig.MaxFreeInstances {
		p.scaleDownInstances(ctx, counts.Free)
	}
}

func (p *BoxPool) maintainVolumePool(ctx context.Context) {
	counts, err := p.resourceQueries.CountVolumesByStatus(ctx)
	if err != nil {
		slog.Error("failed to get volume counts", "error", err)
		return
	}

	slog.Debug("volume pool status",
		"free", counts.Free,
		"attached", counts.Attached,
		"total", counts.Total)

	if counts.Free < p.poolConfig.MinFreeVolumes {
		p.scaleUpVolumes(ctx, counts.Free)
	} else if counts.Free > p.poolConfig.MaxFreeVolumes {
		p.scaleDownVolumes(ctx, counts.Free)
	}
}

func (p *BoxPool) scaleUpInstances(ctx context.Context, currentSize int) {
	instancesToCreate := max(0, p.poolConfig.MinFreeInstances-currentSize)
	slog.Info("creating instances to maintain pool size", "count", instancesToCreate)

	var wg sync.WaitGroup
	for i := 0; i < instancesToCreate; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Create VM config with golden OS image
			vmConfig := *p.vmConfig // Copy the base config
			if p.goldenSnapshot != nil && p.goldenSnapshot.OSImageResourceID != "" {
				vmConfig.OSImageID = p.goldenSnapshot.OSImageResourceID
			}
			instanceID, err := CreateInstance(ctx, p.clients, &vmConfig)
			if err != nil {
				slog.Error("failed to create instance", "error", err)
				return
			}

			slog.Info("created instance", "instanceID", instanceID)

			// Log instance creation event
			now := time.Now()
			createEvent := EventLogEntity{
				PartitionKey: now.Format("2006-01-02"),
				RowKey:       fmt.Sprintf("%s_instance_create", now.Format("20060102T150405")),
				Timestamp:    now,
				EventType:    EventTypeInstanceCreate,
				BoxID:        instanceID,
				Details:      fmt.Sprintf(`{"status":%q}`, ResourceStatusFree),
			}
			if err := WriteEventLog(ctx, p.clients, &createEvent); err != nil {
				slog.Warn("Failed to log instance create event", "error", err)
			}

			// Log resource registry entry
			resourceEntry := ResourceRegistryEntity{
				PartitionKey: ResourceRoleInstance,
				RowKey:       instanceID,
				Timestamp:    now,
				Status:       ResourceStatusFree,
				CreatedAt:    now,
				LastActivity: now,
				Metadata:     fmt.Sprintf(`{"vm_size":%q}`, p.vmConfig.VMSize),
			}
			if err := WriteResourceRegistry(ctx, p.clients, &resourceEntry); err != nil {
				slog.Warn("Failed to log resource registry entry", "error", err)
			}
		}()
	}
	wg.Wait()
}

func (p *BoxPool) scaleDownInstances(ctx context.Context, currentSize int) {
	// Check if enough time has passed since last scale down
	p.mu.Lock()
	if time.Since(p.lastScaleDown) < p.poolConfig.ScaleDownCooldown {
		p.mu.Unlock()
		slog.Info("skipping instance scale down due to cooldown",
			"timeRemaining", p.poolConfig.ScaleDownCooldown-time.Since(p.lastScaleDown))
		return
	}
	p.mu.Unlock()

	instancesToRemove := currentSize - p.poolConfig.MaxFreeInstances
	slog.Info("removing excess instances from pool", "count", instancesToRemove)

	// Get oldest free running instances to remove
	oldestInstances, err := p.resourceQueries.GetOldestFreeRunningInstances(ctx, instancesToRemove)
	if err != nil {
		slog.Error("failed to get oldest free running instances", "error", err)
		return
	}

	// Delete instances
	var wg sync.WaitGroup
	for i := range oldestInstances {
		wg.Add(1)
		go func(idx int) {
			inst := oldestInstances[idx]
			defer wg.Done()
			namer := NewResourceNamer(p.clients.Suffix)
			vmName := namer.BoxVMName(inst.ResourceID)
			resourceGroup := namer.ResourceGroup()

			err := DeleteInstance(ctx, p.clients, resourceGroup, vmName)
			if err != nil {
				slog.Error("failed to delete instance", "instanceID", inst.ResourceID, "error", err)
				return
			}

			slog.Info("deleted instance", "instanceID", inst.ResourceID)

			// Log instance deletion event
			now := time.Now()
			deleteEvent := EventLogEntity{
				PartitionKey: now.Format("2006-01-02"),
				RowKey:       fmt.Sprintf("%s_instance_delete", now.Format("20060102T150405")),
				Timestamp:    now,
				EventType:    EventTypeInstanceDelete,
				BoxID:        inst.ResourceID,
				Details:      `{"reason":"pool_shrink"}`,
			}
			if err := WriteEventLog(ctx, p.clients, &deleteEvent); err != nil {
				slog.Warn("Failed to log instance delete event", "error", err)
			}
		}(i)
	}
	wg.Wait()

	// Update last scale down time
	p.mu.Lock()
	p.lastScaleDown = time.Now()
	p.mu.Unlock()
}

func (p *BoxPool) scaleUpVolumes(ctx context.Context, currentSize int) {
	volumesToCreate := max(0, p.poolConfig.MinFreeVolumes-currentSize)
	slog.Info("creating volumes to maintain pool size", "count", volumesToCreate)

	var wg sync.WaitGroup
	for i := 0; i < volumesToCreate; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			namer := NewResourceNamer(p.clients.Suffix)
			volumeID := uuid.New().String()
			volumeName := namer.VolumePoolDiskName(volumeID)

			now := time.Now().UTC()
			tags := VolumeTags{
				Role:      ResourceRoleVolume,
				Status:    ResourceStatusFree,
				CreatedAt: now.Format(time.RFC3339),
				LastUsed:  now.Format(time.RFC3339),
				VolumeID:  volumeID,
			}

			_, err := CreateVolumeFromSnapshot(ctx, p.clients, p.clients.ResourceGroupName,
				volumeName, p.goldenSnapshot.DataSnapshotResourceID, &tags)
			if err != nil {
				slog.Error("failed to create volume from golden snapshot", "error", err)
				return
			}

			slog.Info("created volume from golden snapshot", "volumeID", volumeID)

			// Log volume creation event
			createEvent := EventLogEntity{
				PartitionKey: now.Format("2006-01-02"),
				RowKey:       fmt.Sprintf("%s_volume_create_%s", now.Format("20060102T150405.000"), volumeID),
				Timestamp:    now,
				EventType:    EventTypeVolumeCreate,
				BoxID:        volumeID,
				Details:      fmt.Sprintf(`{"status":%q,"size_gb":%d}`, ResourceStatusFree, DefaultVolumeSizeGB),
			}
			if err := WriteEventLog(ctx, p.clients, &createEvent); err != nil {
				slog.Warn("Failed to log volume create event", "error", err)
			}

			// Log resource registry entry
			resourceEntry := ResourceRegistryEntity{
				PartitionKey: ResourceRoleVolume,
				RowKey:       volumeID,
				Timestamp:    now,
				Status:       ResourceStatusFree,
				CreatedAt:    now,
				LastActivity: now,
				Metadata:     fmt.Sprintf(`{"size_gb":%d}`, DefaultVolumeSizeGB),
			}
			if err := WriteResourceRegistry(ctx, p.clients, &resourceEntry); err != nil {
				slog.Warn("Failed to log resource registry entry", "error", err)
			}
		}()
	}
	wg.Wait()
}

func (p *BoxPool) scaleDownVolumes(ctx context.Context, currentSize int) {
	// Check if enough time has passed since last scale down
	p.mu.Lock()
	if time.Since(p.lastScaleDown) < p.poolConfig.ScaleDownCooldown {
		p.mu.Unlock()
		slog.Info("skipping volume scale down due to cooldown",
			"timeRemaining", p.poolConfig.ScaleDownCooldown-time.Since(p.lastScaleDown))
		return
	}
	p.mu.Unlock()

	volumesToRemove := currentSize - p.poolConfig.MaxFreeVolumes
	slog.Info("removing excess volumes from pool", "count", volumesToRemove)

	// Get oldest free volumes to remove
	oldestVolumes, err := p.resourceQueries.GetOldestFreeVolumes(ctx, volumesToRemove)
	if err != nil {
		slog.Error("failed to get oldest free volumes", "error", err)
		return
	}

	// Delete volumes
	var wg sync.WaitGroup
	for i := range oldestVolumes {
		wg.Add(1)
		go func(idx int) {
			vol := oldestVolumes[idx]
			defer wg.Done()

			err := DeleteVolume(ctx, p.clients, p.clients.ResourceGroupName, vol.Name)
			if err != nil {
				slog.Error("failed to delete volume", "volumeID", vol.ResourceID, "error", err)
				return
			}

			slog.Info("deleted volume", "volumeID", vol.ResourceID)

			// Log volume deletion event
			now := time.Now()
			deleteEvent := EventLogEntity{
				PartitionKey: now.Format("2006-01-02"),
				RowKey:       fmt.Sprintf("%s_volume_delete", now.Format("20060102T150405")),
				Timestamp:    now,
				EventType:    EventTypeVolumeDelete,
				BoxID:        vol.ResourceID,
				Details:      `{"reason":"pool_shrink"}`,
			}
			if err := WriteEventLog(ctx, p.clients, &deleteEvent); err != nil {
				slog.Warn("Failed to log volume delete event", "error", err)
			}
		}(i)
	}
	wg.Wait()

	// Update last scale down time
	p.mu.Lock()
	p.lastScaleDown = time.Now()
	p.mu.Unlock()
}
