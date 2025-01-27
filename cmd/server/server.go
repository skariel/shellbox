package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
)

func main() {
	log.Println("starting shellbox server")

	// Check required arguments
	if len(os.Args) < 2 {
		log.Fatal("resource group suffix argument is required")
	}
	suffix := os.Args[1]

	log.Println("Current configuration:")
	fmt.Println(infra.FormatConfig(suffix))

	clients, err := infra.NewAzureClients(suffix)
	if err != nil {
		log.Fatal(err)
	}

	// Generate SSH key pair
	keyPath := "/home/shellbox/.ssh/id_rsa"
	_, publicKey, err := sshutil.GenerateKeyPair(keyPath)
	if err != nil {
		log.Fatalf("failed to generate SSH keys: %v", err)
	}
	log.Printf("generated SSH key pair and saved private key to: %s", keyPath)
	log.Printf("public key: %q", publicKey)

	config := &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  publicKey,
		VMSize:        "Standard_D8s_v3",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := infra.NewBoxPool(clients, config)
	pool.MaintainPool(ctx)
}
