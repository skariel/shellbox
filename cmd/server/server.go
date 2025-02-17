package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	clients := infra.NewAzureClients(suffix)

	// Create network infrastructure first
	infra.CreateNetworkInfrastructure(context.Background(), clients)

	// Ensure SSH key pair exists
	keyPath := "/home/shellbox/.ssh/id_rsa"
	_, publicKey, err := sshutil.LoadKeyPair(keyPath)
	if err != nil {
		log.Fatalf("failed to load SSH key pair: %v", err)
	}
	log.Printf("using SSH key pair at: %s", keyPath)
	log.Printf("public key: %q", publicKey)

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
	sshServer := sshserver.New(infra.BastionSSHPort)
	go func() {
		if err := sshServer.Run(); err != nil {
			log.Printf("SSH server error: %v", err)
		}
	}()

	// Keep main running
	select {}
}
