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

	// Create storage account
	poller, err := storageClient.BeginCreate(ctx, clients.ResourceGroupName, accountName, armstorage.AccountCreateParameters{
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
		result.Error = fmt.Errorf("failed to create storage account: %w", err)
		return result
	}

	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to wait for storage account creation: %w", err)
		return result
	}

	// Get connection string
	keysResponse, err := storageClient.ListKeys(ctx, clients.ResourceGroupName, accountName, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to get storage keys: %w", err)
		return result
	}

	if len(keysResponse.Keys) == 0 {
		result.Error = fmt.Errorf("no storage keys found")
		return result
	}

	key := *keysResponse.Keys[0].Value
	result.ConnectionString = fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;EndpointSuffix=core.windows.net", accountName, key)

	// Create tables
	tablesClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	if err != nil {
		result.Error = fmt.Errorf("failed to create tables client: %w", err)
		return result
	}

	// Use provided table names, or fall back to legacy defaults if none provided
	if len(tableNames) == 0 {
		tableNames = []string{tableEventLog, tableResourceRegistry}
	}

	for _, tableName := range tableNames {
		_, err = tablesClient.CreateTable(ctx, tableName, nil)
		if err != nil {
			// Check if this is a "table already exists" error (idempotent operation)
			var respErr *azcore.ResponseError
			if errors.As(err, &respErr) && respErr.StatusCode == 409 && respErr.ErrorCode == "TableAlreadyExists" {
				// Table already exists, this is fine for idempotent operations
				continue
			}
			result.Error = fmt.Errorf("failed to create table %s: %w", tableName, err)
			return result
		}
	}

	return result
}

// CreateTableStorageResourcesLegacy creates a storage account with default table names (backward compatibility)
func CreateTableStorageResourcesLegacy(ctx context.Context, clients *AzureClients, accountName string) TableStorageResult {
	return CreateTableStorageResources(ctx, clients, accountName, nil)
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
	return err
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

// WriteEventLogLegacy writes an entry to the EventLog table using legacy table name (backward compatibility)
func WriteEventLogLegacy(ctx context.Context, clients *AzureClients, event EventLogEntity) error {
	return writeTableEntity(ctx, clients, tableEventLog, event)
}

// WriteResourceRegistryLegacy writes an entry to the ResourceRegistry table using legacy table name (backward compatibility)
func WriteResourceRegistryLegacy(ctx context.Context, clients *AzureClients, resource ResourceRegistryEntity) error {
	return writeTableEntity(ctx, clients, tableResourceRegistry, resource)
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
