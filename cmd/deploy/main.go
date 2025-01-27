package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"shellbox/internal/infra"
)

func readSSHKey(path string) (string, error) {
	expandedPath := filepath.Clean(os.ExpandEnv(path))
	key, err := os.ReadFile(expandedPath)
	if err != nil {
		return "", fmt.Errorf("reading SSH key: %w", err)
	}
	return string(key), nil
}

func main() {
	ctx := context.Background()

	if len(os.Args) < 2 {
		log.Fatal("resource group suffix argument is required")
	}
	suffix := os.Args[1]

	clients, err := infra.NewAzureClients(suffix)
	if err != nil {
		log.Fatal(err)
	}

	rgName := clients.ResourceGroupName
	log.Printf("using resource group: %s", rgName)

	log.Println("Current configuration:")
	fmt.Println(infra.FormatConfig(suffix))

	log.Println("upserting networking infra")
	if err := infra.CreateNetworkInfrastructure(ctx, clients); err != nil {
		log.Fatal(err)
	}

	log.Println("done upserting")
	pubKey, err := readSSHKey("$HOME/.ssh/id_ed25519.pub")
	if err != nil {
		log.Fatalf("could not load ssh pub key: %s", err)
	}

	log.Println("creating bastion")
	if err := infra.DeployBastion(ctx, clients, &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("infrastructure deployment complete")
}
