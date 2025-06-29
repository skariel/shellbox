package main

import (
	"context"
	"fmt"
	"os"
	"shellbox/internal/infra"
	"shellbox/internal/sshserver"
	"shellbox/internal/sshutil"
	"time"
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

	// Load SSH key from local filesystem (copied during deployment)
	_, publicKey, err := sshutil.LoadKeyPair()
	if err != nil {
		logger.Error("failed to load SSH key from file", "error", err)
		os.Exit(1)
	}

	logger.Info("using SSH key from file", "path", sshutil.SSHKeyPath)
	logger.Info("loaded public key", "key", publicKey)

	// Create golden snapshot if it doesn't exist
	logger.Info("ensuring golden snapshot exists")
	goldenSnapshot, err := infra.CreateGoldenSnapshotIfNotExists(context.Background(), clients)
	if err != nil {
		logger.Error("failed to create golden snapshot", "error", err)
		os.Exit(1)
	}
	logger.Info("golden snapshots ready", "dataSnapshot", goldenSnapshot.DataSnapshotName, "osImage", goldenSnapshot.OSImageName, "dataSizeGB", goldenSnapshot.DataSizeGB, "osSizeGB", goldenSnapshot.OSSizeGB)

	// Log server start event
	now := time.Now()
	startEvent := infra.EventLogEntity{
		PartitionKey: now.Format("2006-01-02"),
		RowKey:       fmt.Sprintf("%s_server_start", now.Format("20060102T150405")),
		Timestamp:    now,
		EventType:    "server_start",
		Details:      fmt.Sprintf(`{"suffix":%q}`, suffix),
	}
	if err := infra.WriteEventLog(context.Background(), clients, &startEvent); err != nil {
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

	logger.Info("starting pool management")
	pool := infra.NewBoxPool(clients, vmConfig, poolConfig, goldenSnapshot)
	go pool.MaintainPool(ctx)

	// Start SSH server
	sshServer, err := sshserver.New(infra.BastionSSHPort, clients)
	if err != nil {
		logger.Error("Failed to create SSH server", "error", err)
		cancel()
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
