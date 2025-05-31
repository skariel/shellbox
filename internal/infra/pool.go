package infra

import (
	"context"
	"fmt"
	"log"
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
				log.Printf("creating %d boxes to maintain pool size", boxesToCreate)

				var wg sync.WaitGroup
				for i := 0; i < boxesToCreate; i++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						boxID, err := CreateBox(ctx, p.clients, p.config)
						if err != nil {
							log.Printf("failed to create box: %v", err)
							return
						}

						p.mu.Lock()
						p.boxes[boxID] = "ready"
						p.mu.Unlock()

						log.Printf("created box with ID: %s", boxID)

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
							log.Printf("Failed to log box create event: %v", err)
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
							log.Printf("Failed to log resource registry entry: %v", err)
						}
					}()
				}
				wg.Wait()
			}
		}
	}
}
