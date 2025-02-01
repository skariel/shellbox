package sshutil

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// EnsureKeyPair ensures an SSH key pair exists at the specified path
// If the key doesn't exist, it generates a new one
// Returns the public key string in either case
func EnsureKeyPair(keyPath string) (string, error) {
	expandedPath := filepath.Clean(os.ExpandEnv(keyPath))

	// Check if key already exists
	if _, err := os.Stat(expandedPath); os.IsNotExist(err) {
		_, publicKey, err := GenerateKeyPair(expandedPath)
		if err != nil {
			return "", fmt.Errorf("generating new SSH key pair: %w", err)
		}
		return publicKey, nil
	}

	// Load existing public key
	pubKeyBytes, err := os.ReadFile(expandedPath + ".pub")
	if err != nil {
		return "", fmt.Errorf("reading existing public key: %w", err)
	}
	return strings.TrimSpace(string(pubKeyBytes)), nil
}

// GenerateKeyPair creates a new SSH key pair and saves the private key to the specified path
func GenerateKeyPair(keyPath string) (privateKey string, publicKey string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", fmt.Errorf("generating key pair: %w", err)
	}

	// Generate private key PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	privateKeyString := string(pem.EncodeToMemory(privateKeyPEM))

	// Generate public key in SSH format
	sshPublicKey, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("creating ssh public key: %w", err)
	}
	publicKeyString := string(ssh.MarshalAuthorizedKey(sshPublicKey))
	// Remove any trailing newline that might be present
	publicKeyString = strings.TrimSpace(publicKeyString)

	// Save private key to file
	if err := os.WriteFile(keyPath, []byte(privateKeyString), 0600); err != nil {
		return "", "", fmt.Errorf("writing private key file: %w", err)
	}

	return privateKeyString, publicKeyString, nil
}

// CopyFile copies a file to a remote host using scp
func CopyFile(ctx context.Context, localPath, remotePath, username, hostname string) error {
	scpDest := fmt.Sprintf("%s@%s:%s", username, hostname, remotePath)
	cmd := exec.CommandContext(ctx, "scp", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=4", localPath, scpDest)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// ExecuteCommand executes a command on a remote host using SSH
func ExecuteCommand(ctx context.Context, command, username, hostname string) error {
	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=4",
		fmt.Sprintf("%s@%s", username, hostname),
		command)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}
