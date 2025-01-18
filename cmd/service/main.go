package main

import (
	"context"
	"log"

	"shellbox/internal/infra"
)

func main() {
	ctx := context.Background()

	clients, err := infra.NewAzureClients()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("upserting networking infra")
	if err := infra.CreateNetworkInfrastructure(ctx, clients); err != nil {
		log.Fatal(err)
	}

	log.Printf("done upserting")
}
