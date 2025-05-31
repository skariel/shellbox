package main

import (
	"context"
	"os"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"
)

func main() {
	logger := infra.NewLogger()
	infra.SetDefaultLogger()

	ctx := context.Background()

	if len(os.Args) < 2 {
		logger.Error("resource group suffix argument is required")
		os.Exit(1)
	}
	suffix := os.Args[1]

	clients := infra.NewAzureClients(suffix, true)

	rgName := clients.ResourceGroupName
	logger.Info("using resource group", "name", rgName)

	logger.Info("current configuration", "config", infra.FormatConfig(suffix))

	logger.Info("upserting networking infra")
	infra.CreateNetworkInfrastructure(ctx, clients, true)

	logger.Info("done upserting")
	_, pubKey, err := sshutil.LoadKeyPair(infra.DeploymentSSHKeyPath)
	if err != nil {
		logger.Error("could not load ssh pub key", "error", err)
		os.Exit(1)
	}

	logger.Info("creating bastion")
	bastionIP := infra.DeployBastion(ctx, clients, &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  pubKey,
		VMSize:        "Standard_B2ms",
	})

	logger.Info("infrastructure deployment complete", "bastion_ip", bastionIP)
}
