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

// LoadKeyPair loads or creates an SSH key pair, using the private key as the source of truth.
func LoadKeyPair(keyPath string) (privateKey, publicKey string, err error) {
	expandedPath := filepath.Clean(os.ExpandEnv(keyPath))

	// Try to read private key
	privKeyData, err := os.ReadFile(expandedPath)
	if os.IsNotExist(err) {
		// Generate new RSA key pair
		// Ensure directory exists
		dir := filepath.Dir(expandedPath)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", "", fmt.Errorf("creating directory %s: %w", dir, err)
		}

		// Generate new RSA key pair
		key, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return "", "", fmt.Errorf("generating key pair: %w", err)
		}

		// Create private key PEM
		privateKeyPEM := &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(key),
		}
		privateKey = string(pem.EncodeToMemory(privateKeyPEM))

		// Save private key
		if err := os.WriteFile(expandedPath, []byte(privateKey), 0o600); err != nil {
			return "", "", fmt.Errorf("writing private key file: %w", err)
		}

		privKeyData = []byte(privateKey)
	} else if err != nil {
		return "", "", fmt.Errorf("reading private key: %w", err)
	}

	// Parse private key
	block, _ := pem.Decode(privKeyData)
	if block == nil {
		return "", "", fmt.Errorf("failed to decode PEM block from private key")
	}

	var signer ssh.Signer
	var parseErr error

	// Try parsing with different formats
	signer, parseErr = ssh.ParsePrivateKey(privKeyData)
	if parseErr != nil {
		return "", "", fmt.Errorf("parsing private key: %w", parseErr)
	}

	publicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	privateKey = string(privKeyData)

	return privateKey, publicKey, nil
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

func ExecuteCommandWithOutput(ctx context.Context, command, username, hostname string) (string, error) {
	cmd := exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=4",
		fmt.Sprintf("%s@%s", username, hostname),
		command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%w: %s", err, string(output))
	}
	return string(output), nil
}
