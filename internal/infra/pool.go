package infra

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
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

// NewDefaultPoolConfig creates a production pool configuration
func NewDefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MinFreeInstances:  DefaultMinFreeInstances,
		MaxFreeInstances:  DefaultMaxFreeInstances,
		MaxTotalInstances: DefaultMaxTotalInstances,
		MinFreeVolumes:    DefaultMinFreeVolumes,
		MaxFreeVolumes:    DefaultMaxFreeVolumes,
		MaxTotalVolumes:   DefaultMaxTotalVolumes,
		CheckInterval:     DefaultCheckInterval,
		ScaleDownCooldown: DefaultScaleDownCooldown,
	}
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
	mu         sync.RWMutex
	boxes      map[string]string // boxID -> status
	clients    *AzureClients
	vmConfig   *VMConfig
	poolConfig PoolConfig
}

func NewBoxPool(clients *AzureClients, vmConfig *VMConfig, poolConfig PoolConfig) *BoxPool {
	return &BoxPool{
		boxes:      make(map[string]string),
		clients:    clients,
		vmConfig:   vmConfig,
		poolConfig: poolConfig,
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
			p.mu.Lock()
			currentSize := len(p.boxes)
			p.mu.Unlock()

			if currentSize < p.poolConfig.MinFreeInstances {
				boxesToCreate := p.poolConfig.MinFreeInstances - currentSize
				slog.Info("creating boxes to maintain pool size", "count", boxesToCreate)

				var wg sync.WaitGroup
				for i := 0; i < boxesToCreate; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						boxID, err := CreateBox(ctx, p.clients, p.vmConfig)
						if err != nil {
							slog.Error("failed to create box", "error", err)
							return
						}

						p.mu.Lock()
						p.boxes[boxID] = "ready"
						p.mu.Unlock()

						slog.Info("created box", "boxID", boxID)

						// Log box creation event
						now := time.Now()
						createEvent := EventLogEntity{
							PartitionKey: now.Format("2006-01-02"),
							RowKey:       fmt.Sprintf("%s_box_create", now.Format("20060102T150405")),
							Timestamp:    now,
							EventType:    "box_create",
							BoxID:        boxID,
							Details:      `{"status":"ready"}`,
						}
						if err := WriteEventLog(ctx, p.clients, createEvent); err != nil {
							slog.Warn("Failed to log box create event", "error", err)
						}

						// Log resource registry entry
						resourceEntry := ResourceRegistryEntity{
							PartitionKey: "box",
							RowKey:       boxID,
							Timestamp:    now,
							Status:       "ready",
							CreatedAt:    now,
							LastActivity: now,
							Metadata:     fmt.Sprintf(`{"vm_size":"%s"}`, p.vmConfig.VMSize),
						}
						if err := WriteResourceRegistry(ctx, p.clients, resourceEntry); err != nil {
							slog.Warn("Failed to log resource registry entry", "error", err)
						}
					}()
				}
				wg.Wait()
			}
		}
	}
}
