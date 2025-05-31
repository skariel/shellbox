package infra

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	targetPoolSize = 1
	checkInterval  = 1 * time.Minute
)

type BoxPool struct {
	mu      sync.RWMutex
	boxes   map[string]string // boxID -> status
	clients *AzureClients
	config  *VMConfig
}

func NewBoxPool(clients *AzureClients, config *VMConfig) *BoxPool {
	return &BoxPool{
		boxes:   make(map[string]string),
		clients: clients,
		config:  config,
	}
}

func (p *BoxPool) MaintainPool(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			currentSize := len(p.boxes)
			p.mu.Unlock()

			if currentSize < targetPoolSize {
				boxesToCreate := targetPoolSize - currentSize
				slog.Info("creating boxes to maintain pool size", "count", boxesToCreate)

				var wg sync.WaitGroup
				for i := 0; i < boxesToCreate; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						boxID, err := CreateBox(ctx, p.clients, p.config)
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
							Metadata:     fmt.Sprintf(`{"vm_size":"%s"}`, p.config.VMSize),
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
