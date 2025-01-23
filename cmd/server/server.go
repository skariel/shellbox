package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/ssh"
)

func waitForManagedIdentity(timeout time.Duration) (*infra.AzureClients, error) {
	opts := &infra.RetryOptions{
		Timeout:   timeout,
		Interval:  5 * time.Second,
		Operation: "managed identity initialization",
	}

	return infra.RetryWithTimeout(context.Background(), opts, func(ctx context.Context) (*infra.AzureClients, error) {
		clients, err := infra.NewAzureClients()
		if err != nil {
			return nil, err
		}

		// Test the clients by trying to list resource groups
		pager := clients.ResourceClient.NewListPager(nil)
		_, err = pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		return clients, nil
	})
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
