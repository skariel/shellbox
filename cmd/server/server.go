package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/sshserver"
	"shellbox/internal/sshutil"
)

func main() {
	log.Println("starting shellbox server")

	if len(os.Args) < 2 {
		log.Fatal("resource group suffix argument is required")
	}
	suffix := os.Args[1]

	log.Println("Current configuration:")
	fmt.Println(infra.FormatConfig(suffix))

	clients := infra.NewAzureClients(suffix, false)

	// Create network infrastructure first
	infra.CreateNetworkInfrastructure(context.Background(), clients, false)

	// Ensure SSH key pair exists
	keyPath := "/home/shellbox/.ssh/id_rsa"
	_, publicKey, err := sshutil.LoadKeyPair(keyPath)
	if err != nil {
		log.Fatalf("failed to load SSH key pair: %v", err)
	}
	log.Printf("using SSH key pair at: %s", keyPath)
	log.Printf("public key: %q", publicKey)

	// Log server start event
	now := time.Now()
	startEvent := infra.EventLogEntity{
		PartitionKey: now.Format("2006-01-02"),
		RowKey:       fmt.Sprintf("%s_server_start", now.Format("20060102T150405")),
		Timestamp:    now,
		EventType:    "server_start",
		Details:      fmt.Sprintf(`{"suffix":"%s"}`, suffix),
	}
	if err := infra.WriteEventLog(context.Background(), clients, startEvent); err != nil {
		log.Printf("Failed to log server start event: %v", err)
	}

	config := &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  publicKey,
		VMSize:        "Standard_D8s_v3",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := infra.NewBoxPool(clients, config)
	go pool.MaintainPool(ctx)

	// Start SSH server
	sshServer, err := sshserver.New(infra.BastionSSHPort, clients)
	if err != nil {
		log.Fatalf("Failed to create SSH server: %v", err)
	}
	go func() {
		if err := sshServer.Run(); err != nil {
			log.Printf("SSH server error: %v", err)
		}
	}()

	// Keep main running
	select {}
}
