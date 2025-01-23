package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
	"shellbox/internal/infra"
)

const (
	targetPoolSize = 2
	checkInterval  = 1 * time.Minute
)

type BoxPool struct {
	mu      sync.RWMutex
	boxes   map[string]string // boxID -> status
	clients *infra.AzureClients
	config  *infra.BoxConfig
}

func NewBoxPool(clients *infra.AzureClients, config *infra.BoxConfig) *BoxPool {
	return &BoxPool{
		boxes:   make(map[string]string),
		clients: clients,
		config:  config,
	}
}

func (p *BoxPool) maintainPool(ctx context.Context) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.mu.Lock()
			currentSize := len(p.boxes)
			p.mu.Unlock()

			if currentSize < targetPoolSize {
				boxesToCreate := targetPoolSize - currentSize
				log.Printf("creating %d boxes to maintain pool size", boxesToCreate)

				for i := 0; i < boxesToCreate; i++ {
					boxID, err := infra.CreateBox(ctx, p.clients, p.config)
					if err != nil {
						log.Printf("failed to create box: %v", err)
						continue
					}

					p.mu.Lock()
					p.boxes[boxID] = "ready"
					p.mu.Unlock()

					log.Printf("created box with ID: %s", boxID)
				}
			}
		}
	}
}

func generateSSHKeyPair() (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Convert private key to PEM format
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	privateKeyStr := string(pem.EncodeToMemory(privateKeyPEM))

	// Generate public key
	publicKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKeyStr := string(ssh.MarshalAuthorizedKey(publicKey))

	return privateKeyStr, publicKeyStr, nil
}

func main() {
	log.Println("starting shellbox server")

	// Generate SSH key pair
	privateKey, publicKey, err := generateSSHKeyPair()
	if err != nil {
		log.Fatalf("failed to generate SSH keys: %v", err)
	}
	log.Println("generated SSH key pair")
	// TODO: Store keys in vault

	clients, err := infra.NewAzureClients()
	if err != nil {
		log.Fatalf("failed to create Azure clients: %v", err)
	}

	config := &infra.BoxConfig{
		AdminUsername: "shellbox",
		SSHPublicKey:  os.Getenv("SSH_PUBLIC_KEY"),
		VMSize:        "Standard_B2ms",
	}

	if config.SSHPublicKey == "" {
		log.Fatal("SSH_PUBLIC_KEY environment variable not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool := NewBoxPool(clients, config)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("received signal %v, initiating shutdown", sig)
		cancel()
	}()

	pool.maintainPool(ctx)
}
