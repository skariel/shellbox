package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"shellbox/internal/infra"
	"shellbox/internal/ssh"
)

func main() {
	log.Println("starting shellbox server")

	keyPath := "/home/ubuntu/.ssh/shellbox_id_rsa"
	// Generate SSH key pair
	_, publicKey, err := ssh.GenerateKeyPair(keyPath)
	if err != nil {
		log.Fatalf("failed to generate SSH keys: %v", err)
	}
	log.Printf("generated SSH key pair and saved private key to: %s", keyPath)

	clients, err := infra.NewAzureClients()
	if err != nil {
		log.Fatalf("failed to create Azure clients: %v", err)
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
