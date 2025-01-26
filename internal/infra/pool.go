package infra

import (
	"context"
	"log"
	"sync"
	"time"
)

const (
	targetPoolSize = 30
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

				for i := 0; i < boxesToCreate; i++ {
					boxID, err := CreateBox(ctx, p.clients, p.config)
					if err != nil {
						log.Printf("failed to create box: %v", err)
						continue
					}

					p.mu.Lock()
					p.boxes[boxID] = "ready"
					p.mu.Unlock()

					log.Printf("created box with ID: %s", boxID)
				}
			}
		}
	}
}
