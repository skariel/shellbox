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
	ctx := context.Background()

	if len(os.Args) < 2 {
		log.Fatal("resource group suffix argument is required")
	}
	suffix := os.Args[1]

	clients := infra.NewAzureClients(suffix)

	rgName := clients.ResourceGroupName
	log.Printf("using resource group: %s", rgName)

	log.Println("current configuration:")
	fmt.Println(infra.FormatConfig(suffix))

	log.Println("upserting networking infra")
	infra.CreateNetworkInfrastructure(ctx, clients)

	log.Println("done upserting")
	_, pubKey, err := sshutil.LoadKeyPair("$HOME/.ssh/id_ed25519")
	if err != nil {
		log.Fatalf("could not load ssh pub key: %s", err)
	}

	log.Println("creating bastion")
	bastionIP := infra.DeployBastion(ctx, clients, &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	})

	log.Println("infrastructure deployment complete")
	log.Printf("bastion IP: %s", bastionIP)
}
