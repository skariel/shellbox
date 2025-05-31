package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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
func CreateTableStorageResources(ctx context.Context, clients *AzureClients, accountName string) TableStorageResult {
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
		Location: to.Ptr(location),
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

	tableNames := []string{tableEventLog, tableResourceRegistry}
	for _, tableName := range tableNames {
		_, err = tablesClient.CreateTable(ctx, tableName, nil)
		if err != nil {
			result.Error = fmt.Errorf("failed to create table %s: %w", tableName, err)
			return result
		}
	}

	return result
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

// WriteEventLog writes an entry to the EventLog table
func WriteEventLog(ctx context.Context, clients *AzureClients, event EventLogEntity) error {
	if clients.TableClient == nil {
		return fmt.Errorf("table client not available")
	}

	entityMap := map[string]interface{}{
		"PartitionKey": event.PartitionKey,
		"RowKey":       event.RowKey,
		"Timestamp":    event.Timestamp,
		"EventType":    event.EventType,
	}

	if event.SessionID != "" {
		entityMap["SessionID"] = event.SessionID
	}
	if event.BoxID != "" {
		entityMap["BoxID"] = event.BoxID
	}
	if event.UserKey != "" {
		entityMap["UserKey"] = event.UserKey
	}
	if event.Details != "" {
		entityMap["Details"] = event.Details
	}

	tableClient := clients.TableClient.NewClient(tableEventLog)
	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}
	_, err = tableClient.AddEntity(ctx, entityBytes, nil)
	return err
}

// WriteResourceRegistry writes an entry to the ResourceRegistry table
func WriteResourceRegistry(ctx context.Context, clients *AzureClients, resource ResourceRegistryEntity) error {
	if clients.TableClient == nil {
		return fmt.Errorf("table client not available")
	}

	entityMap := map[string]interface{}{
		"PartitionKey": resource.PartitionKey,
		"RowKey":       resource.RowKey,
		"Timestamp":    resource.Timestamp,
		"Status":       resource.Status,
		"CreatedAt":    resource.CreatedAt,
		"LastActivity": resource.LastActivity,
	}

	if resource.VMName != "" {
		entityMap["VMName"] = resource.VMName
	}
	if resource.Metadata != "" {
		entityMap["Metadata"] = resource.Metadata
	}

	tableClient := clients.TableClient.NewClient(tableResourceRegistry)
	entityBytes, err := json.Marshal(entityMap)
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}
	_, err = tableClient.AddEntity(ctx, entityBytes, nil)
	return err
}
