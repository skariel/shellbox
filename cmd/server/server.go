package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"shellbox/internal/infra"
	"shellbox/internal/sshutil"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
)

func waitForRoleAssignment(ctx context.Context, cred *azidentity.ManagedIdentityCredential) error {
	opts := infra.DefaultRetryOptions()
	opts.Operation = "verify role assignment"
	opts.Timeout = 5 * time.Minute
	opts.Interval = 100 * time.Second

	_, err := infra.RetryWithTimeout(ctx, opts, func(ctx context.Context) (bool, error) {
		client, err := armsubscriptions.NewClient(cred, nil)
		if err != nil {
			return false, err // retry with error
		}
		pager := client.NewListPager(nil)
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, err // retry with error
		}
		if len(page.Value) == 0 {
			return false, fmt.Errorf("no subscriptions found") // retry with specific error
		}
		return true, nil
	})
	return err
}

func main() {
	log.Println("starting shellbox server")

	// Check required arguments
	if len(os.Args) < 2 {
		log.Fatal("resource group suffix argument is required")
	}
	suffix := os.Args[1]

	log.Println("Current configuration:")
	fmt.Println(infra.FormatConfig(suffix))

	// Wait for role assignment to propagate
	cred, err := azidentity.NewManagedIdentityCredential(nil)
	if err != nil {
		log.Fatalf("failed to create credential: %v", err)
	}

	log.Println("waiting for role assignment to propagate...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := waitForRoleAssignment(ctx, cred); err != nil {
		log.Fatalf("role assignment not ready after timeout: %v", err)
	}
	clients := infra.NewAzureClients(suffix)
	log.Println("role assignment active")

	// Create network infrastructure first
	infra.CreateNetworkInfrastructure(context.Background(), clients)

	// Generate SSH key pair
	// TODO: generate only if not existing
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

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	pool := infra.NewBoxPool(clients, config)
	pool.MaintainPool(ctx)
}
