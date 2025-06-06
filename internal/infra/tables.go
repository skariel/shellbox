package infra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/storage/armstorage"
)

// TableStorageResult contains the result of the Table Storage creation operation
type TableStorageResult struct {
	ConnectionString string
	Error            error
}

// CreateTableStorageResources creates a storage account and tables
func CreateTableStorageResources(ctx context.Context, clients *AzureClients, accountName string, tableNames []string) TableStorageResult {
	result := TableStorageResult{}

	storageClient, err := armstorage.NewAccountsClient(clients.SubscriptionID, clients.Cred, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create storage client: %w", err)
		return result
	}

	// Ensure storage account exists
	if err := ensureStorageAccountExists(ctx, storageClient, clients.ResourceGroupName, accountName); err != nil {
		result.Error = err
		return result
	}

	// Get connection string
	connectionString, err := getStorageConnectionString(ctx, storageClient, clients.ResourceGroupName, accountName)
	if err != nil {
		result.Error = err
		return result
	}
	result.ConnectionString = connectionString

	// Create tables
	if err := createTables(ctx, connectionString, tableNames); err != nil {
		result.Error = err
		return result
	}

	return result
}

// ensureStorageAccountExists checks if storage account exists and creates it if needed
func ensureStorageAccountExists(ctx context.Context, storageClient *armstorage.AccountsClient, resourceGroupName, accountName string) error {
	// Check if storage account already exists in our resource group
	_, err := storageClient.GetProperties(ctx, resourceGroupName, accountName, nil)
	if err == nil {
		return nil // Already exists
	}

	// Storage account doesn't exist, check name availability
	checkAvailability, err := storageClient.CheckNameAvailability(ctx, armstorage.AccountCheckNameAvailabilityParameters{
		Name: to.Ptr(accountName),
		Type: to.Ptr("Microsoft.Storage/storageAccounts"),
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to check storage account name availability: %w", err)
	}

	if !*checkAvailability.NameAvailable {
		return fmt.Errorf("storage account name '%s' is not available: %s", accountName, *checkAvailability.Message)
	}

	// Create the storage account
	poller, err := storageClient.BeginCreate(ctx, resourceGroupName, accountName, armstorage.AccountCreateParameters{
		SKU: &armstorage.SKU{
			Name: to.Ptr(armstorage.SKUNameStandardLRS),
		},
		Kind:     to.Ptr(armstorage.KindStorageV2),
		Location: to.Ptr(Location),
		Properties: &armstorage.AccountPropertiesCreateParameters{
			AllowBlobPublicAccess: to.Ptr(false),
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("failed to create storage account: %w", err)
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to wait for storage account creation: %w", err)
	}

	return nil
}

// getStorageConnectionString retrieves the connection string for a storage account
func getStorageConnectionString(ctx context.Context, storageClient *armstorage.AccountsClient, resourceGroupName, accountName string) (string, error) {
	var keysResponse armstorage.AccountsClientListKeysResponse

	// Retry getting storage keys in case the storage account is still provisioning
	err := RetryOperation(ctx, func(ctx context.Context) error {
		var err error
		keysResponse, err = storageClient.ListKeys(ctx, resourceGroupName, accountName, nil)
		return err
	}, 5*time.Minute, 10*time.Second, "get storage account keys")
	if err != nil {
		return "", fmt.Errorf("failed to get storage keys: %w", err)
	}

	if len(keysResponse.Keys) == 0 {
		return "", fmt.Errorf("no storage keys found")
	}

	key := *keysResponse.Keys[0].Value
	return fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;EndpointSuffix=core.windows.net", accountName, key), nil
}

// createTables creates the specified tables in the storage account
func createTables(ctx context.Context, connectionString string, tableNames []string) error {
	tablesClient, err := aztables.NewServiceClientFromConnectionString(connectionString, nil)
	if err != nil {
		return fmt.Errorf("failed to create tables client: %w", err)
	}

	// Require table names to be explicitly provided
	if len(tableNames) == 0 {
		return fmt.Errorf("table names must be provided - cannot create tables without explicit names")
	}

	for _, tableName := range tableNames {
		_, err = tablesClient.CreateTable(ctx, tableName, nil)
		if err != nil {
			// Check if this is a "table already exists" error (idempotent operation)
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == 409 && respErr.ErrorCode == "TableAlreadyExists" {
				continue // Table already exists, this is fine
			}
			return fmt.Errorf("failed to create table %s: %w", tableName, err)
		}
	}

	return nil
}

// EventLogEntity represents an entry in the EventLog table
type EventLogEntity struct {
	PartitionKey string    `json:"PartitionKey"`
	RowKey       string    `json:"RowKey"`
	Timestamp    time.Time `json:"Timestamp"`
	EventType    string    `json:"EventType"`
	SessionID    string    `json:"SessionID,omitempty"`
	BoxID        string    `json:"BoxID,omitempty"`
	UserKey      string    `json:"UserKey,omitempty"`
	Details      string    `json:"Details,omitempty"`
}

// ResourceRegistryEntity represents an entry in the ResourceRegistry table
type ResourceRegistryEntity struct {
	PartitionKey string    `json:"PartitionKey"`
	RowKey       string    `json:"RowKey"`
	Timestamp    time.Time `json:"Timestamp"`
	Status       string    `json:"Status"`
	VMName       string    `json:"VMName,omitempty"`
	CreatedAt    time.Time `json:"CreatedAt"`
	LastActivity time.Time `json:"LastActivity"`
	Metadata     string    `json:"Metadata,omitempty"`
}

// writeTableEntity is a generic function for writing entities to Azure Tables
func writeTableEntity(ctx context.Context, clients *AzureClients, tableName string, entity interface{}) error {
	if clients.TableClient == nil {
		return fmt.Errorf("table client not available")
	}

	tableClient := clients.TableClient.NewClient(tableName)
	entityBytes, err := json.Marshal(entity)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}
	_, err = tableClient.AddEntity(ctx, entityBytes, nil)
	if err != nil {
		return fmt.Errorf("failed to add entity to table %s: %w", tableName, err)
	}
	return nil
}

// WriteEventLog writes an entry to the EventLog table
func WriteEventLog(ctx context.Context, clients *AzureClients, event EventLogEntity) error {
	namer := NewResourceNamer(clients.Suffix)
	tableName := namer.EventLogTableName()
	return writeTableEntity(ctx, clients, tableName, event)
}

// WriteResourceRegistry writes an entry to the ResourceRegistry table
func WriteResourceRegistry(ctx context.Context, clients *AzureClients, resource ResourceRegistryEntity) error {
	namer := NewResourceNamer(clients.Suffix)
	tableName := namer.ResourceRegistryTableName()
	return writeTableEntity(ctx, clients, tableName, resource)
}

// CleanupTestTables deletes test tables with the given suffix (for test cleanup)
func CleanupTestTables(ctx context.Context, clients *AzureClients, suffix string) error {
	if clients.TableClient == nil {
		return fmt.Errorf("table client not available")
	}

	namer := NewResourceNamer(suffix)
	tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

	for _, tableName := range tableNames {
		_, err := clients.TableClient.DeleteTable(ctx, tableName, nil)
		if err != nil {
			// Log but don't fail on cleanup errors
			return fmt.Errorf("failed to delete table %s: %w", tableName, err)
		}
	}

	return nil
}
