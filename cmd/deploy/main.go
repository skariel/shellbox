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

	clients, err := infra.NewAzureClients()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("cleaning up old resource groups")
	if err := infra.CleanupOldResourceGroups(ctx, clients); err != nil {
		log.Printf("cleanup failed: %v", err)
	}

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
	if err := infra.DeployBastion(ctx, clients, &infra.BastionConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	}); err != nil {
		log.Fatal(err)
	}

	log.Println("creating test boxes")

	// Create first box
	box1ID, err := infra.CreateBox(ctx, clients, &infra.BoxConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("created box1 with ID: %s", box1ID)

	// Create second box
	box2ID, err := infra.CreateBox(ctx, clients, &infra.BoxConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("created box2 with ID: %s", box2ID)
}
