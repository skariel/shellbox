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
	"strings"

	"golang.org/x/crypto/ssh"
)

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
	publicKey, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("creating ssh public key: %w", err)
	}
	publicKeyString := string(ssh.MarshalAuthorizedKey(publicKey))
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
