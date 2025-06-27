package main

import (
	"context"
	"log/slog"
	"os"
	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
)

func main() {
	infra.SetDefaultLogger()

	ctx := context.Background()

	if len(os.Args) < 2 {
		slog.Error("resource group suffix argument is required")
		os.Exit(1)
	}
	suffix := os.Args[1]

	clients := infra.NewAzureClients(suffix, true)

	rgName := clients.ResourceGroupName
	slog.Info("using resource group", "name", rgName)

	slog.Info("current configuration", "config", infra.FormatConfig(suffix))

	slog.Info("upserting networking infra")
	infra.CreateNetworkInfrastructure(ctx, clients, true)

	slog.Info("done upserting")
	_, pubKey, err := sshutil.LoadKeyPair()
	if err != nil {
		slog.Error("could not load ssh pub key", "error", err)
		os.Exit(1)
	}

	slog.Info("creating bastion")
	bastionIP := infra.DeployBastion(ctx, clients, &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	})

	slog.Info("infrastructure deployment complete", "bastionIP", bastionIP)
}
