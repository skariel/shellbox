package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"shellbox/internal/infra"
)

func cleanupOldResourceGroups(ctx context.Context, clients *infra.AzureClients) error {
	filter := fmt.Sprintf("startswith(name,'%s')", infra.ResourceGroupPrefix)
	pager := clients.ResourceClient.NewListPager(&filter, nil)

	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing resource groups: %w", err)
		}

		for _, group := range page.Value {
			// Parse timestamp from group name
			parts := strings.Split(*group.Name, "-")
			if len(parts) != 3 {
				continue
			}
			
			timestamp, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				continue
			}

			createTime := time.Unix(timestamp, 0)
			if createTime.Before(cutoff) {
				log.Printf("Deleting old resource group: %s", *group.Name)
				_, err := clients.ResourceClient.BeginDelete(ctx, *group.Name, nil)
				if err != nil {
					log.Printf("Failed to delete resource group %s: %v", *group.Name, err)
				}
			}
		}
	}
	return nil
}

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
	if err := cleanupOldResourceGroups(ctx, clients); err != nil {
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
