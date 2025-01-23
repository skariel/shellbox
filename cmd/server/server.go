package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/ssh"
)

func waitForManagedIdentity(timeout time.Duration) (*infra.AzureClients, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return nil, fmt.Errorf("timeout waiting for managed identity: %w", lastErr)
			}
			return nil, fmt.Errorf("timeout waiting for managed identity")
		case <-ticker.C:
			clients, err := infra.NewAzureClients()
			if err == nil {
				// Test the clients by trying to list resource groups
				pager := clients.ResourceClient.NewListPager(nil)
				_, err = pager.NextPage(ctx)
				if err == nil {
					log.Println("managed identity is ready")
					return clients, nil
				}
			}
			lastErr = err
			log.Printf("waiting for managed identity to be ready: %v", err)
		}
	}
}

func main() {
	log.Println("starting shellbox server")

	keyPath := "/home/shellbox/.ssh/id_rsa"
	// Generate SSH key pair
	_, publicKey, err := ssh.GenerateKeyPair(keyPath)
	if err != nil {
		log.Fatalf("failed to generate SSH keys: %v", err)
	}
	log.Printf("generated SSH key pair and saved private key to: %s", keyPath)

	// Wait for managed identity to be fully ready before proceeding
	clients, err := waitForManagedIdentity(2 * time.Minute)
	if err != nil {
		log.Fatalf("failed waiting for managed identity: %v", err)
	}

	config := &infra.BoxConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  publicKey,
		VMSize:        "Standard_B2ms",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := infra.NewBoxPool(clients, config)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("received signal %v, initiating shutdown", sig)
		cancel()
	}()

	pool.MaintainPool(ctx)
}
