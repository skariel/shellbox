package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/google/uuid"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestTableStorageCreationAndIdempotency(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	namer := env.GetResourceNamer()
	storageAccountName := namer.SharedStorageAccountName()
	tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

	test.LogTestProgress(t, "creating table storage resources (first time)", "accountName", storageAccountName, "tableNames", tableNames)

	// Create table storage resources first time
	result1 := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	if result1.Error != nil {
		t.Fatalf("should create table storage resources without error: %v", result1.Error)
	}
	if result1.ConnectionString == "" {
		t.Fatalf("should return valid connection string")
	}

	// Verify connection string format
	if !strings.Contains(result1.ConnectionString, "DefaultEndpointsProtocol=https") {
		t.Errorf("connection string should use HTTPS")
	}
	if !strings.Contains(result1.ConnectionString, fmt.Sprintf("AccountName=%s", storageAccountName)) {
		t.Errorf("connection string should contain account name")
	}
	if !strings.Contains(result1.ConnectionString, "EndpointSuffix=core.windows.net") {
		t.Errorf("connection string should contain endpoint suffix")
	}

	test.LogTestProgress(t, "verifying table client creation from connection string")

	// Test that we can create a client from the connection string
	tableClient1, err := aztables.NewServiceClientFromConnectionString(result1.ConnectionString, nil)
	if err != nil {
		t.Fatalf("should be able to create table client from connection string: %v", err)
	}
	if tableClient1 == nil {
		t.Fatalf("table client should not be nil")
	}

	test.LogTestProgress(t, "verifying tables were created")

	// Verify that required tables exist by attempting to query them
	expectedTables := tableNames
	for _, tableName := range expectedTables {
		specificTableClient := tableClient1.NewClient(tableName)

		// Try to list entities to verify table exists (will return empty list if table is empty)
		pager := specificTableClient.NewListEntitiesPager(nil)
		_, err := pager.NextPage(ctx)
		if err != nil {
			t.Errorf("table %s should exist and be queryable: %v", tableName, err)
		}
	}

	test.LogTestProgress(t, "testing table storage idempotency (second creation)")

	// Create table storage second time (should be idempotent)
	result2 := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	if result2.Error != nil {
		t.Fatalf("second creation should succeed (idempotent): %v", result2.Error)
	}

	// Connection strings should be the same
	if result1.ConnectionString != result2.ConnectionString {
		t.Errorf("connection strings should be identical")
	}

	// Verify tables still exist and are functional after second creation
	tableClient2, err := aztables.NewServiceClientFromConnectionString(result2.ConnectionString, nil)
	if err != nil {
		t.Fatalf("should create table client after idempotent creation: %v", err)
	}

	env.Clients.TableStorageConnectionString = result2.ConnectionString
	env.Clients.TableClient = tableClient2

	// Test writing to tables after second creation to verify functionality
	testEntity := infra.EventLogEntity{
		PartitionKey: "idempotency-test",
		RowKey:       "test-event",
		Timestamp:    time.Now().UTC(),
		EventType:    "idempotency_test",
		Details:      "Testing table storage creation and idempotency",
	}

	err = infra.WriteEventLog(ctx, env.Clients, testEntity)
	if err != nil {
		t.Errorf("should be able to write to tables after idempotent creation: %v", err)
	}

	test.LogTestProgress(t, "table storage creation and idempotency test completed")
}

func TestTableStorageEntityOperations(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Set up table storage
	namer := env.GetResourceNamer()
	storageAccountName := namer.SharedStorageAccountName()
	tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

	result := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	if result.Error != nil {
		t.Fatalf("should create table storage resources: %v", result.Error)
	}

	// Set up table client in the environment
	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	if err != nil {
		t.Fatalf("should create table client: %v", err)
	}
	env.Clients.TableClient = tableClient

	test.LogTestProgress(t, "testing EventLog entity operations")

	// Test EventLog entity operations
	t.Run("EventLogOperations", func(t *testing.T) {
		sessionID := uuid.New().String()
		boxID := uuid.New().String()

		eventEntity := infra.EventLogEntity{
			PartitionKey: "test-partition",
			RowKey:       fmt.Sprintf("test-event-%s", uuid.New().String()),
			Timestamp:    time.Now().UTC(),
			EventType:    "box_created",
			SessionID:    sessionID,
			BoxID:        boxID,
			UserKey:      "test-user-key",
			Details:      "Integration test event",
		}

		// Write event log
		err := infra.WriteEventLog(ctx, env.Clients, eventEntity)
		if err != nil {
			t.Fatalf("should write event log without error: %v", err)
		}

		// Verify entity was written by querying it back
		eventLogClient := tableClient.NewClient(namer.EventLogTableName())

		// Get the entity
		response, err := eventLogClient.GetEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
		if err != nil {
			t.Fatalf("should be able to retrieve written event log: %v", err)
		}

		var retrievedEntity infra.EventLogEntity
		err = json.Unmarshal(response.Value, &retrievedEntity)
		if err != nil {
			t.Fatalf("should be able to unmarshal retrieved entity: %v", err)
		}

		// Verify entity fields
		if eventEntity.PartitionKey != retrievedEntity.PartitionKey {
			t.Errorf("partition key should match: expected %s, got %s", eventEntity.PartitionKey, retrievedEntity.PartitionKey)
		}
		if eventEntity.RowKey != retrievedEntity.RowKey {
			t.Errorf("row key should match: expected %s, got %s", eventEntity.RowKey, retrievedEntity.RowKey)
		}
		if eventEntity.EventType != retrievedEntity.EventType {
			t.Errorf("event type should match: expected %s, got %s", eventEntity.EventType, retrievedEntity.EventType)
		}
		if eventEntity.SessionID != retrievedEntity.SessionID {
			t.Errorf("session ID should match: expected %s, got %s", eventEntity.SessionID, retrievedEntity.SessionID)
		}
		if eventEntity.BoxID != retrievedEntity.BoxID {
			t.Errorf("box ID should match: expected %s, got %s", eventEntity.BoxID, retrievedEntity.BoxID)
		}
		if eventEntity.UserKey != retrievedEntity.UserKey {
			t.Errorf("user key should match: expected %s, got %s", eventEntity.UserKey, retrievedEntity.UserKey)
		}
		if eventEntity.Details != retrievedEntity.Details {
			t.Errorf("details should match: expected %s, got %s", eventEntity.Details, retrievedEntity.Details)
		}
	})

	test.LogTestProgress(t, "testing ResourceRegistry entity operations")

	// Test ResourceRegistry entity operations
	t.Run("ResourceRegistryOperations", func(t *testing.T) {
		now := time.Now().UTC()

		resourceEntity := infra.ResourceRegistryEntity{
			PartitionKey: "resource-partition",
			RowKey:       fmt.Sprintf("test-resource-%s", uuid.New().String()),
			Timestamp:    now,
			Status:       "active",
			VMName:       "test-vm-integration",
			CreatedAt:    now.Add(-1 * time.Hour),
			LastActivity: now.Add(-5 * time.Minute),
			Metadata:     `{"type": "integration-test", "cpu": 2, "memory": "4GB"}`,
		}

		// Write resource registry entry
		err := infra.WriteResourceRegistry(ctx, env.Clients, resourceEntity)
		if err != nil {
			t.Fatalf("should write resource registry without error: %v", err)
		}

		// Verify entity was written by querying it back
		registryClient := tableClient.NewClient(namer.ResourceRegistryTableName())

		// Get the entity
		response, err := registryClient.GetEntity(ctx, resourceEntity.PartitionKey, resourceEntity.RowKey, nil)
		if err != nil {
			t.Fatalf("should be able to retrieve written resource registry entry: %v", err)
		}

		var retrievedEntity infra.ResourceRegistryEntity
		err = json.Unmarshal(response.Value, &retrievedEntity)
		if err != nil {
			t.Fatalf("should be able to unmarshal retrieved entity: %v", err)
		}

		// Verify entity fields
		if resourceEntity.PartitionKey != retrievedEntity.PartitionKey {
			t.Errorf("partition key should match: expected %s, got %s", resourceEntity.PartitionKey, retrievedEntity.PartitionKey)
		}
		if resourceEntity.RowKey != retrievedEntity.RowKey {
			t.Errorf("row key should match: expected %s, got %s", resourceEntity.RowKey, retrievedEntity.RowKey)
		}
		if resourceEntity.Status != retrievedEntity.Status {
			t.Errorf("status should match: expected %s, got %s", resourceEntity.Status, retrievedEntity.Status)
		}
		if resourceEntity.VMName != retrievedEntity.VMName {
			t.Errorf("VM name should match: expected %s, got %s", resourceEntity.VMName, retrievedEntity.VMName)
		}
		if resourceEntity.Metadata != retrievedEntity.Metadata {
			t.Errorf("metadata should match: expected %s, got %s", resourceEntity.Metadata, retrievedEntity.Metadata)
		}
	})
}

func TestTableStorageQueryOperations(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Set up table storage
	namer := env.GetResourceNamer()
	storageAccountName := namer.SharedStorageAccountName()
	tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

	result := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	if result.Error != nil {
		t.Fatalf("should create table storage resources: %v", result.Error)
	}

	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	if err != nil {
		t.Fatalf("should create table client: %v", err)
	}
	env.Clients.TableClient = tableClient

	test.LogTestProgress(t, "creating multiple entities for query testing")

	// Create multiple entities for querying
	partitionKey := "query-test-partition"
	entities := []infra.EventLogEntity{
		{
			PartitionKey: partitionKey,
			RowKey:       "event-001",
			Timestamp:    time.Now().UTC(),
			EventType:    "box_created",
			SessionID:    "session-1",
			BoxID:        "box-1",
			UserKey:      "user-1",
			Details:      "First test event",
		},
		{
			PartitionKey: partitionKey,
			RowKey:       "event-002",
			Timestamp:    time.Now().UTC(),
			EventType:    "box_connected",
			SessionID:    "session-1",
			BoxID:        "box-1",
			UserKey:      "user-1",
			Details:      "Second test event",
		},
		{
			PartitionKey: partitionKey,
			RowKey:       "event-003",
			Timestamp:    time.Now().UTC(),
			EventType:    "box_deleted",
			SessionID:    "session-2",
			BoxID:        "box-2",
			UserKey:      "user-2",
			Details:      "Third test event",
		},
	}

	// Write all entities
	for _, entity := range entities {
		err := infra.WriteEventLog(ctx, env.Clients, entity)
		if err != nil {
			t.Fatalf("should write entity %s: %v", entity.RowKey, err)
		}
	}

	test.LogTestProgress(t, "testing query operations")

	eventLogClient := tableClient.NewClient(namer.EventLogTableName())

	// Test querying all entities in partition
	filter := fmt.Sprintf("PartitionKey eq '%s'", partitionKey)
	listOptions := &aztables.ListEntitiesOptions{
		Filter: &filter,
	}

	pager := eventLogClient.NewListEntitiesPager(listOptions)
	var retrievedEntities []infra.EventLogEntity

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			t.Fatalf("should get page without error: %v", err)
		}

		for _, entity := range page.Entities {
			var eventEntity infra.EventLogEntity
			err := json.Unmarshal(entity, &eventEntity)
			if err != nil {
				t.Fatalf("should unmarshal entity: %v", err)
			}
			retrievedEntities = append(retrievedEntities, eventEntity)
		}
	}

	// Verify we got all entities
	if len(retrievedEntities) != 3 {
		t.Errorf("should retrieve all 3 entities, got %d", len(retrievedEntities))
	}

	// Verify specific entities
	retrievedByRowKey := make(map[string]infra.EventLogEntity)
	for _, entity := range retrievedEntities {
		retrievedByRowKey[entity.RowKey] = entity
	}

	for _, originalEntity := range entities {
		retrieved, exists := retrievedByRowKey[originalEntity.RowKey]
		if !exists {
			t.Errorf("should find entity with row key %s", originalEntity.RowKey)
		}
		if originalEntity.EventType != retrieved.EventType {
			t.Errorf("event type should match for %s: expected %s, got %s", originalEntity.RowKey, originalEntity.EventType, retrieved.EventType)
		}
		if originalEntity.SessionID != retrieved.SessionID {
			t.Errorf("session ID should match for %s: expected %s, got %s", originalEntity.RowKey, originalEntity.SessionID, retrieved.SessionID)
		}
		if originalEntity.BoxID != retrieved.BoxID {
			t.Errorf("box ID should match for %s: expected %s, got %s", originalEntity.RowKey, originalEntity.BoxID, retrieved.BoxID)
		}
	}

	test.LogTestProgress(t, "testing filtered query operations")

	// Test filtered query (specific session)
	sessionFilter := fmt.Sprintf("PartitionKey eq '%s' and SessionID eq 'session-1'", partitionKey)
	sessionListOptions := &aztables.ListEntitiesOptions{
		Filter: &sessionFilter,
	}

	sessionPager := eventLogClient.NewListEntitiesPager(sessionListOptions)
	var sessionEntities []infra.EventLogEntity

	for sessionPager.More() {
		page, err := sessionPager.NextPage(ctx)
		if err != nil {
			t.Fatalf("should get filtered page without error: %v", err)
		}

		for _, entity := range page.Entities {
			var eventEntity infra.EventLogEntity
			err := json.Unmarshal(entity, &eventEntity)
			if err != nil {
				t.Fatalf("should unmarshal filtered entity: %v", err)
			}
			sessionEntities = append(sessionEntities, eventEntity)
		}
	}

	// Should only get entities from session-1
	if len(sessionEntities) != 2 {
		t.Errorf("should retrieve only 2 entities for session-1, got %d", len(sessionEntities))
	}
	for _, entity := range sessionEntities {
		if entity.SessionID != "session-1" {
			t.Errorf("all entities should be from session-1, got %s", entity.SessionID)
		}
	}
}

func TestTableStorageUpdateOperations(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Set up table storage
	namer := env.GetResourceNamer()
	storageAccountName := namer.SharedStorageAccountName()
	tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

	result := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	if result.Error != nil {
		t.Fatalf("should create table storage resources: %v", result.Error)
	}

	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	if err != nil {
		t.Fatalf("should create table client: %v", err)
	}
	env.Clients.TableClient = tableClient

	test.LogTestProgress(t, "testing entity update operations")

	registryClient := tableClient.NewClient(namer.ResourceRegistryTableName())

	// Create initial entity
	now := time.Now().UTC()
	resourceEntity := infra.ResourceRegistryEntity{
		PartitionKey: "update-test-partition",
		RowKey:       "update-test-resource",
		Timestamp:    now,
		Status:       "inactive",
		VMName:       "test-vm-update",
		CreatedAt:    now.Add(-2 * time.Hour),
		LastActivity: now.Add(-1 * time.Hour),
		Metadata:     `{"initial": "data"}`,
	}

	// Write initial entity
	err = infra.WriteResourceRegistry(ctx, env.Clients, resourceEntity)
	if err != nil {
		t.Fatalf("should write initial entity: %v", err)
	}

	// Update the entity
	resourceEntity.Status = "active"
	resourceEntity.LastActivity = now
	resourceEntity.Metadata = `{"updated": "data", "cpu": 4}`

	// Use update function for updating existing entity
	err = infra.UpdateResourceRegistry(ctx, env.Clients, resourceEntity)
	if err != nil {
		t.Fatalf("should update entity without error: %v", err)
	}

	test.LogTestProgress(t, "verifying entity was updated")

	// Retrieve and verify updated entity
	response, err := registryClient.GetEntity(ctx, resourceEntity.PartitionKey, resourceEntity.RowKey, nil)
	if err != nil {
		t.Fatalf("should retrieve updated entity: %v", err)
	}

	var updatedEntity infra.ResourceRegistryEntity
	err = json.Unmarshal(response.Value, &updatedEntity)
	if err != nil {
		t.Fatalf("should unmarshal updated entity: %v", err)
	}

	// Verify updates
	if updatedEntity.Status != "active" {
		t.Errorf("status should be updated: expected 'active', got %s", updatedEntity.Status)
	}
	if updatedEntity.Metadata != `{"updated": "data", "cpu": 4}` {
		t.Errorf("metadata should be updated: expected %s, got %s", `{"updated": "data", "cpu": 4}`, updatedEntity.Metadata)
	}
	if updatedEntity.VMName != resourceEntity.VMName {
		t.Errorf("VM name should remain unchanged: expected %s, got %s", resourceEntity.VMName, updatedEntity.VMName)
	}
}

func TestTableStorageDeleteOperations(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Set up table storage
	namer := env.GetResourceNamer()
	storageAccountName := namer.SharedStorageAccountName()
	tableNames := []string{namer.EventLogTableName(), namer.ResourceRegistryTableName()}

	result := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	if result.Error != nil {
		t.Fatalf("should create table storage resources: %v", result.Error)
	}

	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	if err != nil {
		t.Fatalf("should create table client: %v", err)
	}
	env.Clients.TableClient = tableClient

	test.LogTestProgress(t, "testing entity delete operations")

	eventLogClient := tableClient.NewClient(namer.EventLogTableName())

	// Create entity to delete
	eventEntity := infra.EventLogEntity{
		PartitionKey: "delete-test-partition",
		RowKey:       "delete-test-event",
		Timestamp:    time.Now().UTC(),
		EventType:    "test_event",
		SessionID:    "delete-session",
		BoxID:        "delete-box",
		UserKey:      "delete-user",
		Details:      "Entity to be deleted",
	}

	// Write entity
	err = infra.WriteEventLog(ctx, env.Clients, eventEntity)
	if err != nil {
		t.Fatalf("should write entity to be deleted: %v", err)
	}

	// Verify entity exists
	_, err = eventLogClient.GetEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
	if err != nil {
		t.Fatalf("entity should exist before deletion: %v", err)
	}

	test.LogTestProgress(t, "deleting entity")

	// Delete entity
	_, err = eventLogClient.DeleteEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
	if err != nil {
		t.Fatalf("should delete entity without error: %v", err)
	}

	test.LogTestProgress(t, "verifying entity was deleted")

	// Verify entity is deleted
	_, err = eventLogClient.GetEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
	if err == nil {
		t.Errorf("should not be able to retrieve deleted entity")
	}
}

func TestTableStorageErrorHandling(t *testing.T) {
	t.Parallel()

	env := test.SetupTestEnvironment(t)
	defer env.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	test.LogTestProgress(t, "testing table storage error handling")

	// Test 1: Operations without table client
	clientsWithoutTable := &infra.AzureClients{
		TableClient: nil,
	}

	eventEntity := infra.EventLogEntity{
		PartitionKey: "error-test",
		RowKey:       "error-event",
		Timestamp:    time.Now().UTC(),
		EventType:    "error_test",
	}

	err := infra.WriteEventLog(ctx, clientsWithoutTable, eventEntity)
	if err == nil {
		t.Errorf("should error when table client is not available")
	}
	if err != nil && !strings.Contains(err.Error(), "table client not available") {
		t.Errorf("should have appropriate error message, got: %v", err)
	}

	// Test 2: Invalid storage account name
	invalidAccountName := "invalid-account-name-with-special-chars!"
	invalidTableNames := []string{"InvalidTable"}
	result := infra.CreateTableStorageResources(ctx, env.Clients, invalidAccountName, invalidTableNames)
	if result.Error == nil {
		t.Errorf("should error with invalid storage account name")
	}

	test.LogTestProgress(t, "error handling tests completed")
}
