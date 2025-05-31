package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/sshserver"
	"shellbox/internal/sshutil"
)

func main() {
	logger := infra.NewLogger()
	infra.SetDefaultLogger()

	logger.Info("starting shellbox server")

	if len(os.Args) < 2 {
		logger.Error("resource group suffix argument is required")
		os.Exit(1)
	}
	suffix := os.Args[1]

	logger.Info("current configuration", "config", infra.FormatConfig(suffix))

	clients := infra.NewAzureClients(suffix, false)

	// Create network infrastructure first
	infra.CreateNetworkInfrastructure(context.Background(), clients, false)

	// Ensure SSH key pair exists
	keyPath := infra.BastionSSHKeyPath
	_, publicKey, err := sshutil.LoadKeyPair(keyPath)
	if err != nil {
		logger.Error("failed to load SSH key pair", "error", err)
		os.Exit(1)
	}
	logger.Info("using SSH key pair", "path", keyPath)
	logger.Info("loaded public key", "key", publicKey)

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
		logger.Warn("Failed to log server start event", "error", err)
	}

	vmConfig := &infra.VMConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  publicKey,
		VMSize:        "Standard_D8s_v3",
	}

	// Use development pool configuration for now
	poolConfig := infra.NewDevPoolConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := infra.NewBoxPool(clients, vmConfig, poolConfig)
	go pool.MaintainPool(ctx)

	// Start SSH server
	sshServer, err := sshserver.New(infra.BastionSSHPort, clients)
	if err != nil {
		logger.Error("Failed to create SSH server", "error", err)
		os.Exit(1)
	}
	go func() {
		if err := sshServer.Run(); err != nil {
			logger.Error("SSH server error", "error", err)
		}
	}()

	// Keep main running
	select {}
}
