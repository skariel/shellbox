package infra

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"golang.org/x/crypto/ssh"
)

// ensureKeyVaultExists creates a Key Vault in the golden resource group if it doesn't exist
func ensureKeyVaultExists(ctx context.Context, clients *AzureClients) error {
	slog.Info("Ensuring Key Vault exists", "name", KeyVaultName, "resourceGroup", GoldenSnapshotResourceGroup)

	// Check if Key Vault already exists
	_, err := clients.KeyVaultClient.Get(ctx, GoldenSnapshotResourceGroup, KeyVaultName, nil)
	if err == nil {
		slog.Info("Key Vault already exists", "name", KeyVaultName)
		return nil
	}

	// Create Key Vault
	slog.Info("Creating Key Vault", "name", KeyVaultName)

	// Get tenant ID from Azure CLI or environment
	tenantID := os.Getenv("AZURE_TENANT_ID")
	if tenantID == "" {
		// Try to get from Azure CLI
		cmd := exec.Command("az", "account", "show", "--query", "tenantId", "-o", "tsv")
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get tenant ID: %w", err)
		}
		tenantID = strings.TrimSpace(string(output))
	}

	vault := armkeyvault.VaultCreateOrUpdateParameters{
		Location: to.Ptr(Location),
		Properties: &armkeyvault.VaultProperties{
			SKU: &armkeyvault.SKU{
				Family: to.Ptr(armkeyvault.SKUFamilyA),
				Name:   to.Ptr(armkeyvault.SKUNameStandard),
			},
			TenantID:                  to.Ptr(tenantID),
			AccessPolicies:            []*armkeyvault.AccessPolicyEntry{},
			EnableSoftDelete:          to.Ptr(true),
			SoftDeleteRetentionInDays: to.Ptr(int32(7)),
			EnablePurgeProtection:     to.Ptr(true), // Required by Azure policy
			EnableRbacAuthorization:   to.Ptr(true), // Use RBAC instead of access policies
			NetworkACLs: &armkeyvault.NetworkRuleSet{
				DefaultAction: to.Ptr(armkeyvault.NetworkRuleActionAllow), // Allow all for now, restrict later
				Bypass:        to.Ptr(armkeyvault.NetworkRuleBypassOptionsAzureServices),
			},
		},
	}

	poller, err := clients.KeyVaultClient.BeginCreateOrUpdate(ctx, GoldenSnapshotResourceGroup, KeyVaultName, vault, nil)
	if err != nil {
		return fmt.Errorf("failed to create Key Vault: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, &DefaultPollOptions)
	if err != nil {
		return fmt.Errorf("failed to wait for Key Vault creation: %w", err)
	}

	slog.Info("Key Vault created successfully", "name", KeyVaultName)
	return nil
}

// getKeyVaultURI returns the Key Vault URI
func getKeyVaultURI() string {
	return fmt.Sprintf("https://%s.vault.azure.net", KeyVaultName)
}

// initializeSecretsClient creates a secrets client for the Key Vault
func initializeSecretsClient(clients *AzureClients) error {
	if clients.SecretsClient != nil {
		return nil
	}

	vaultURI := getKeyVaultURI()
	secretsClient, err := azsecrets.NewClient(vaultURI, clients.Cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create secrets client: %w", err)
	}

	clients.SecretsClient = secretsClient
	return nil
}

// ensureBastionSSHKey generates or retrieves the bastion SSH key pair from Key Vault
func ensureBastionSSHKey(ctx context.Context, clients *AzureClients) (privateKey, publicKey string, err error) {
	slog.Info("Ensuring bastion SSH key exists in Key Vault")

	// Initialize secrets client if not already done
	if err := initializeSecretsClient(clients); err != nil {
		return "", "", fmt.Errorf("failed to initialize secrets client: %w", err)
	}

	// Try to get existing private key
	privateKeyResp, err := clients.SecretsClient.GetSecret(ctx, BastionSSHKeySecretName, "", nil)
	if err == nil {
		// Key exists, also get public key
		publicKeyResp, err := clients.SecretsClient.GetSecret(ctx, BastionSSHPublicKeySecretName, "", nil)
		if err != nil {
			return "", "", fmt.Errorf("failed to get public key from vault: %w", err)
		}

		slog.Info("Retrieved existing SSH key pair from Key Vault")
		return *privateKeyResp.Value, *publicKeyResp.Value, nil
	}

	// Generate new SSH key pair
	slog.Info("Generating new SSH key pair for bastion")

	key, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Create private key PEM
	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	privateKeyStr := string(pem.EncodeToMemory(privateKeyPEM))

	// Create public key
	sshPublicKey, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH public key: %w", err)
	}
	publicKeyStr := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublicKey)))

	// Store private key in Key Vault
	privateKeyParams := azsecrets.SetSecretParameters{
		Value: to.Ptr(privateKeyStr),
		SecretAttributes: &azsecrets.SecretAttributes{
			Enabled: to.Ptr(true),
		},
	}

	_, err = clients.SecretsClient.SetSecret(ctx, BastionSSHKeySecretName, privateKeyParams, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to store private key in vault: %w", err)
	}

	// Store public key in Key Vault
	publicKeyParams := azsecrets.SetSecretParameters{
		Value: to.Ptr(publicKeyStr),
		SecretAttributes: &azsecrets.SecretAttributes{
			Enabled: to.Ptr(true),
		},
	}

	_, err = clients.SecretsClient.SetSecret(ctx, BastionSSHPublicKeySecretName, publicKeyParams, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to store public key in vault: %w", err)
	}

	slog.Info("Stored new SSH key pair in Key Vault")
	return privateKeyStr, publicKeyStr, nil
}

// GetBastionSSHKeyFromVault retrieves the bastion SSH key pair from Key Vault
func GetBastionSSHKeyFromVault(ctx context.Context, clients *AzureClients) (privateKey, publicKey string, err error) {
	// Initialize secrets client if not already done
	if err := initializeSecretsClient(clients); err != nil {
		return "", "", fmt.Errorf("failed to initialize secrets client: %w", err)
	}

	// Get private key
	privateKeyResp, err := clients.SecretsClient.GetSecret(ctx, BastionSSHKeySecretName, "", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get private key from vault: %w", err)
	}

	// Get public key
	publicKeyResp, err := clients.SecretsClient.GetSecret(ctx, BastionSSHPublicKeySecretName, "", nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get public key from vault: %w", err)
	}

	return *privateKeyResp.Value, *publicKeyResp.Value, nil
}

// WriteBastionSSHKeyToFile writes the SSH key to the local filesystem (for bastion use)
func WriteBastionSSHKeyToFile(privateKey string) error {
	// Write private key to file
	if err := os.MkdirAll("/home/shellbox/.ssh", 0o700); err != nil {
		return fmt.Errorf("failed to create SSH directory: %w", err)
	}

	if err := os.WriteFile(BastionSSHKeyPath, []byte(privateKey), 0o600); err != nil {
		return fmt.Errorf("failed to write private key to file: %w", err)
	}

	slog.Info("Written SSH key to filesystem", "path", BastionSSHKeyPath)
	return nil
}
