package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, result1.Error, "should create table storage resources without error")
	require.NotEmpty(t, result1.ConnectionString, "should return valid connection string")

	// Verify connection string format
	assert.Contains(t, result1.ConnectionString, "DefaultEndpointsProtocol=https", "connection string should use HTTPS")
	assert.Contains(t, result1.ConnectionString, fmt.Sprintf("AccountName=%s", storageAccountName), "connection string should contain account name")
	assert.Contains(t, result1.ConnectionString, "EndpointSuffix=core.windows.net", "connection string should contain endpoint suffix")

	test.LogTestProgress(t, "verifying table client creation from connection string")

	// Test that we can create a client from the connection string
	tableClient1, err := aztables.NewServiceClientFromConnectionString(result1.ConnectionString, nil)
	require.NoError(t, err, "should be able to create table client from connection string")
	require.NotNil(t, tableClient1, "table client should not be nil")

	test.LogTestProgress(t, "verifying tables were created")

	// Verify that required tables exist by attempting to query them
	expectedTables := tableNames
	for _, tableName := range expectedTables {
		specificTableClient := tableClient1.NewClient(tableName)

		// Try to list entities to verify table exists (will return empty list if table is empty)
		pager := specificTableClient.NewListEntitiesPager(nil)
		_, err := pager.NextPage(ctx)
		assert.NoError(t, err, "table %s should exist and be queryable", tableName)
	}

	test.LogTestProgress(t, "testing table storage idempotency (second creation)")

	// Create table storage second time (should be idempotent)
	result2 := infra.CreateTableStorageResources(ctx, env.Clients, storageAccountName, tableNames)
	require.NoError(t, result2.Error, "second creation should succeed (idempotent)")

	// Connection strings should be the same
	assert.Equal(t, result1.ConnectionString, result2.ConnectionString, "connection strings should be identical")

	// Verify tables still exist and are functional after second creation
	tableClient2, err := aztables.NewServiceClientFromConnectionString(result2.ConnectionString, nil)
	require.NoError(t, err, "should create table client after idempotent creation")

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
	assert.NoError(t, err, "should be able to write to tables after idempotent creation")

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
	require.NoError(t, result.Error, "should create table storage resources")

	// Set up table client in the environment
	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	require.NoError(t, err, "should create table client")
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
		require.NoError(t, err, "should write event log without error")

		// Verify entity was written by querying it back
		eventLogClient := tableClient.NewClient(namer.EventLogTableName())

		// Get the entity
		response, err := eventLogClient.GetEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
		require.NoError(t, err, "should be able to retrieve written event log")

		var retrievedEntity infra.EventLogEntity
		err = json.Unmarshal(response.Value, &retrievedEntity)
		require.NoError(t, err, "should be able to unmarshal retrieved entity")

		// Verify entity fields
		assert.Equal(t, eventEntity.PartitionKey, retrievedEntity.PartitionKey, "partition key should match")
		assert.Equal(t, eventEntity.RowKey, retrievedEntity.RowKey, "row key should match")
		assert.Equal(t, eventEntity.EventType, retrievedEntity.EventType, "event type should match")
		assert.Equal(t, eventEntity.SessionID, retrievedEntity.SessionID, "session ID should match")
		assert.Equal(t, eventEntity.BoxID, retrievedEntity.BoxID, "box ID should match")
		assert.Equal(t, eventEntity.UserKey, retrievedEntity.UserKey, "user key should match")
		assert.Equal(t, eventEntity.Details, retrievedEntity.Details, "details should match")
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
		require.NoError(t, err, "should write resource registry without error")

		// Verify entity was written by querying it back
		registryClient := tableClient.NewClient(namer.ResourceRegistryTableName())

		// Get the entity
		response, err := registryClient.GetEntity(ctx, resourceEntity.PartitionKey, resourceEntity.RowKey, nil)
		require.NoError(t, err, "should be able to retrieve written resource registry entry")

		var retrievedEntity infra.ResourceRegistryEntity
		err = json.Unmarshal(response.Value, &retrievedEntity)
		require.NoError(t, err, "should be able to unmarshal retrieved entity")

		// Verify entity fields
		assert.Equal(t, resourceEntity.PartitionKey, retrievedEntity.PartitionKey, "partition key should match")
		assert.Equal(t, resourceEntity.RowKey, retrievedEntity.RowKey, "row key should match")
		assert.Equal(t, resourceEntity.Status, retrievedEntity.Status, "status should match")
		assert.Equal(t, resourceEntity.VMName, retrievedEntity.VMName, "VM name should match")
		assert.Equal(t, resourceEntity.Metadata, retrievedEntity.Metadata, "metadata should match")
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
	require.NoError(t, result.Error, "should create table storage resources")

	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	require.NoError(t, err, "should create table client")
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
		require.NoError(t, err, "should write entity %s", entity.RowKey)
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
		require.NoError(t, err, "should get page without error")

		for _, entity := range page.Entities {
			var eventEntity infra.EventLogEntity
			err := json.Unmarshal(entity, &eventEntity)
			require.NoError(t, err, "should unmarshal entity")
			retrievedEntities = append(retrievedEntities, eventEntity)
		}
	}

	// Verify we got all entities
	assert.Len(t, retrievedEntities, 3, "should retrieve all 3 entities")

	// Verify specific entities
	retrievedByRowKey := make(map[string]infra.EventLogEntity)
	for _, entity := range retrievedEntities {
		retrievedByRowKey[entity.RowKey] = entity
	}

	for _, originalEntity := range entities {
		retrieved, exists := retrievedByRowKey[originalEntity.RowKey]
		assert.True(t, exists, "should find entity with row key %s", originalEntity.RowKey)
		assert.Equal(t, originalEntity.EventType, retrieved.EventType, "event type should match for %s", originalEntity.RowKey)
		assert.Equal(t, originalEntity.SessionID, retrieved.SessionID, "session ID should match for %s", originalEntity.RowKey)
		assert.Equal(t, originalEntity.BoxID, retrieved.BoxID, "box ID should match for %s", originalEntity.RowKey)
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
		require.NoError(t, err, "should get filtered page without error")

		for _, entity := range page.Entities {
			var eventEntity infra.EventLogEntity
			err := json.Unmarshal(entity, &eventEntity)
			require.NoError(t, err, "should unmarshal filtered entity")
			sessionEntities = append(sessionEntities, eventEntity)
		}
	}

	// Should only get entities from session-1
	assert.Len(t, sessionEntities, 2, "should retrieve only 2 entities for session-1")
	for _, entity := range sessionEntities {
		assert.Equal(t, "session-1", entity.SessionID, "all entities should be from session-1")
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
	require.NoError(t, result.Error, "should create table storage resources")

	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	require.NoError(t, err, "should create table client")
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
	require.NoError(t, err, "should write initial entity")

	// Update the entity
	resourceEntity.Status = "active"
	resourceEntity.LastActivity = now
	resourceEntity.Metadata = `{"updated": "data", "cpu": 4}`

	// Use existing infra function to update
	err = infra.WriteResourceRegistry(ctx, env.Clients, resourceEntity)
	require.NoError(t, err, "should update entity without error")

	test.LogTestProgress(t, "verifying entity was updated")

	// Retrieve and verify updated entity
	response, err := registryClient.GetEntity(ctx, resourceEntity.PartitionKey, resourceEntity.RowKey, nil)
	require.NoError(t, err, "should retrieve updated entity")

	var updatedEntity infra.ResourceRegistryEntity
	err = json.Unmarshal(response.Value, &updatedEntity)
	require.NoError(t, err, "should unmarshal updated entity")

	// Verify updates
	assert.Equal(t, "active", updatedEntity.Status, "status should be updated")
	assert.Equal(t, `{"updated": "data", "cpu": 4}`, updatedEntity.Metadata, "metadata should be updated")
	assert.Equal(t, resourceEntity.VMName, updatedEntity.VMName, "VM name should remain unchanged")
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
	require.NoError(t, result.Error, "should create table storage resources")

	env.Clients.TableStorageConnectionString = result.ConnectionString
	tableClient, err := aztables.NewServiceClientFromConnectionString(result.ConnectionString, nil)
	require.NoError(t, err, "should create table client")
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
	require.NoError(t, err, "should write entity to be deleted")

	// Verify entity exists
	_, err = eventLogClient.GetEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
	require.NoError(t, err, "entity should exist before deletion")

	test.LogTestProgress(t, "deleting entity")

	// Delete entity
	_, err = eventLogClient.DeleteEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
	require.NoError(t, err, "should delete entity without error")

	test.LogTestProgress(t, "verifying entity was deleted")

	// Verify entity is deleted
	_, err = eventLogClient.GetEntity(ctx, eventEntity.PartitionKey, eventEntity.RowKey, nil)
	assert.Error(t, err, "should not be able to retrieve deleted entity")
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
	assert.Error(t, err, "should error when table client is not available")
	assert.Contains(t, err.Error(), "table client not available", "should have appropriate error message")

	// Test 2: Invalid storage account name
	invalidAccountName := "invalid-account-name-with-special-chars!"
	invalidTableNames := []string{"InvalidTable"}
	result := infra.CreateTableStorageResources(ctx, env.Clients, invalidAccountName, invalidTableNames)
	assert.Error(t, result.Error, "should error with invalid storage account name")

	test.LogTestProgress(t, "error handling tests completed")
}
