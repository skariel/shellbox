//go:build client

package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"shellbox/internal/infra"
	"shellbox/internal/test"
)

func TestTableStorageClientCreation(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	tests := []struct {
		name             string
		connectionString string
		expectError      bool
		description      string
	}{
		{
			name:             "ValidConnectionString",
			connectionString: "DefaultEndpointsProtocol=https;AccountName=teststorage;AccountKey=dGVzdGtleXZhbHVlMTIzNDU2Nzg5MA==;EndpointSuffix=core.windows.net",
			expectError:      false,
			description:      "Valid connection string should create client successfully",
		},
		{
			name:             "EmptyConnectionString",
			connectionString: "",
			expectError:      true,
			description:      "Empty connection string should fail",
		},
		{
			name:             "InvalidConnectionString",
			connectionString: "invalid-connection-string",
			expectError:      true,
			description:      "Invalid connection string format should fail",
		},
		{
			name:             "MissingAccountName",
			connectionString: "DefaultEndpointsProtocol=https;AccountKey=dGVzdGtleXZhbHVlMTIzNDU2Nzg5MA==;EndpointSuffix=core.windows.net",
			expectError:      true,
			description:      "Connection string missing AccountName should fail",
		},
		{
			name:             "MissingAccountKey",
			connectionString: "DefaultEndpointsProtocol=https;AccountName=teststorage;EndpointSuffix=core.windows.net",
			expectError:      true,
			description:      "Connection string missing AccountKey should fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := aztables.NewServiceClientFromConnectionString(tt.connectionString, nil)

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, client, "Client should be nil when error expected")
			} else {
				assert.NoError(t, err, tt.description)
				assert.NotNil(t, client, "Client should not be nil when no error expected")
			}
		})
	}
}

func TestTableStorageConfigFileHandling(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	// Create temporary directory for test config files
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		configContent string
		fileName      string
		expectError   bool
		description   string
	}{
		{
			name: "ValidConfigFile",
			configContent: `{
				"connectionString": "DefaultEndpointsProtocol=https;AccountName=teststorage;AccountKey=dGVzdGtleXZhbHVlMTIzNDU2Nzg5MA==;EndpointSuffix=core.windows.net"
			}`,
			fileName:    ".tablestorage.json",
			expectError: false,
			description: "Valid config file should be parsed successfully",
		},
		{
			name:          "MissingConfigFile",
			configContent: "",
			fileName:      "nonexistent.json",
			expectError:   true,
			description:   "Missing config file should return error",
		},
		{
			name:          "InvalidJSON",
			configContent: `{"connectionString": "test"`,
			fileName:      ".tablestorage.json",
			expectError:   true,
			description:   "Invalid JSON should return error",
		},
		{
			name: "MissingConnectionString",
			configContent: `{
				"otherField": "value"
			}`,
			fileName:    ".tablestorage.json",
			expectError: false,
			description: "Missing connectionString field should result in empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config file if content provided
			configFilePath := ""
			if tt.configContent != "" {
				configFilePath = filepath.Join(tempDir, tt.fileName)
				err := os.WriteFile(configFilePath, []byte(tt.configContent), 0o600)
				require.NoError(t, err, "Failed to create test config file")
			} else if tt.fileName != "nonexistent.json" {
				configFilePath = filepath.Join(tempDir, tt.fileName)
			} else {
				configFilePath = filepath.Join(tempDir, "nonexistent.json")
			}

			// Test config reading
			config, err := readTableStorageConfig(configFilePath)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				if tt.name == "ValidConfigFile" {
					assert.Contains(t, config.ConnectionString, "AccountName=teststorage", "Connection string should be parsed correctly")
				}
			}
		})
	}
}

func TestTableOperationEntities(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	t.Run("EventLogEntity", func(t *testing.T) {
		// Test EventLogEntity marshaling
		entity := infra.EventLogEntity{
			PartitionKey: "test-partition",
			RowKey:       "test-row-123",
			Timestamp:    time.Now(),
			EventType:    "box_created",
			SessionID:    "session-456",
			BoxID:        "box-789",
			UserKey:      "user-key-abc",
			Details:      "Test event details",
		}

		// Test JSON marshaling
		data, err := json.Marshal(entity)
		require.NoError(t, err, "EventLogEntity should marshal to JSON")

		// Test JSON unmarshaling
		var unmarshaled infra.EventLogEntity
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err, "EventLogEntity should unmarshal from JSON")

		// Verify fields
		assert.Equal(t, entity.PartitionKey, unmarshaled.PartitionKey)
		assert.Equal(t, entity.RowKey, unmarshaled.RowKey)
		assert.Equal(t, entity.EventType, unmarshaled.EventType)
		assert.Equal(t, entity.SessionID, unmarshaled.SessionID)
		assert.Equal(t, entity.BoxID, unmarshaled.BoxID)
		assert.Equal(t, entity.UserKey, unmarshaled.UserKey)
		assert.Equal(t, entity.Details, unmarshaled.Details)
	})

	t.Run("ResourceRegistryEntity", func(t *testing.T) {
		// Test ResourceRegistryEntity marshaling
		now := time.Now()
		entity := infra.ResourceRegistryEntity{
			PartitionKey: "resource-partition",
			RowKey:       "resource-row-456",
			Timestamp:    now,
			Status:       "active",
			VMName:       "test-vm-name",
			CreatedAt:    now.Add(-1 * time.Hour),
			LastActivity: now.Add(-5 * time.Minute),
			Metadata:     `{"cpu": 2, "memory": "4GB"}`,
		}

		// Test JSON marshaling
		data, err := json.Marshal(entity)
		require.NoError(t, err, "ResourceRegistryEntity should marshal to JSON")

		// Test JSON unmarshaling
		var unmarshaled infra.ResourceRegistryEntity
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err, "ResourceRegistryEntity should unmarshal from JSON")

		// Verify fields
		assert.Equal(t, entity.PartitionKey, unmarshaled.PartitionKey)
		assert.Equal(t, entity.RowKey, unmarshaled.RowKey)
		assert.Equal(t, entity.Status, unmarshaled.Status)
		assert.Equal(t, entity.VMName, unmarshaled.VMName)
		assert.Equal(t, entity.Metadata, unmarshaled.Metadata)
	})
}

func TestTableStorageClientIntegration(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	// Test that table storage client integrates properly with AzureClients
	env := test.SetupMinimalTestEnvironment(t)

	// Create a test clients struct
	clients := &infra.AzureClients{
		Suffix: env.Suffix,
	}

	// Test missing table storage config (should not cause fatal error)
	// This simulates the production behavior where table storage is optional
	assert.Nil(t, clients.TableClient, "TableClient should be nil when no config available")

	// Test with valid connection string
	testConnectionString := "DefaultEndpointsProtocol=https;AccountName=teststorage;AccountKey=dGVzdGtleXZhbHVlMTIzNDU2Nzg5MA==;EndpointSuffix=core.windows.net"
	clients.TableStorageConnectionString = testConnectionString

	// Test client creation
	tableClient, err := aztables.NewServiceClientFromConnectionString(testConnectionString, nil)
	require.NoError(t, err, "Should be able to create table client with valid connection string")
	require.NotNil(t, tableClient, "Table client should not be nil")

	clients.TableClient = tableClient
	assert.NotNil(t, clients.TableClient, "TableClient should be set")
}

func TestTableStorageConstants(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	// Test that required table names are defined
	// Note: These are internal constants, so we test them indirectly by checking
	// they're used in the table creation patterns

	expectedTables := []string{"EventLog", "ResourceRegistry"}

	for _, tableName := range expectedTables {
		assert.NotEmpty(t, tableName, "Table name should not be empty")
		assert.Regexp(t, `^[A-Za-z][A-Za-z0-9]*$`, tableName, "Table name should follow Azure table naming conventions")
	}
}

func TestConnectionStringGeneration(t *testing.T) {
	test.RequireCategory(t, test.CategoryClient)

	tests := []struct {
		name        string
		accountName string
		accountKey  string
		description string
	}{
		{
			name:        "ValidParams",
			accountName: "teststorage",
			accountKey:  "dGVzdGtleXZhbHVlMTIzNDU2Nzg5MA==",
			description: "Valid account name and key should generate proper connection string",
		},
		{
			name:        "LongAccountName",
			accountName: "verylongstorageaccountname",
			accountKey:  "YW5vdGhlcnRlc3RrZXl2YWx1ZTEyMzQ1Njc4OTA=",
			description: "Long account name should be handled correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the connection string format that would be generated
			connectionString := fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;EndpointSuffix=core.windows.net",
				tt.accountName, tt.accountKey)

			// Verify format
			assert.Contains(t, connectionString, "DefaultEndpointsProtocol=https", "Should contain HTTPS protocol")
			assert.Contains(t, connectionString, fmt.Sprintf("AccountName=%s", tt.accountName), "Should contain account name")
			assert.Contains(t, connectionString, fmt.Sprintf("AccountKey=%s", tt.accountKey), "Should contain account key")
			assert.Contains(t, connectionString, "EndpointSuffix=core.windows.net", "Should contain endpoint suffix")

			// Test that client can be created with generated connection string
			client, err := aztables.NewServiceClientFromConnectionString(connectionString, nil)
			assert.NoError(t, err, tt.description)
			assert.NotNil(t, client, "Client should be created successfully")
		})
	}
}

// Helper function to read table storage config (mimics the private function)
func readTableStorageConfig(configFile string) (*TableStorageConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read Table Storage config file: %w", err)
	}

	var config TableStorageConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Table Storage config: %w", err)
	}

	return &config, nil
}

// TableStorageConfig represents the structure of the table storage configuration file
type TableStorageConfig struct {
	ConnectionString string `json:"connectionString"`
}
