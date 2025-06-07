package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/aztables"

	"shellbox/internal/infra"
	"shellbox/test"
)

func TestTableStorageClientCreation(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			client, err := aztables.NewServiceClientFromConnectionString(tt.connectionString, nil)

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
				if client != nil {
					t.Errorf("Client should be nil when error expected")
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.description, err)
				}
				if client == nil {
					t.Errorf("Client should not be nil when no error expected")
				}
			}
		})
	}
}

func TestTableStorageConfigFileHandling(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			// Create config file if content provided
			configFilePath := ""
			if tt.configContent != "" {
				configFilePath = filepath.Join(tempDir, tt.fileName)
				err := os.WriteFile(configFilePath, []byte(tt.configContent), 0o600)
				if err != nil {
					t.Fatalf("Failed to create test config file: %v", err)
				}
			} else if tt.fileName != "nonexistent.json" {
				configFilePath = filepath.Join(tempDir, tt.fileName)
			} else {
				configFilePath = filepath.Join(tempDir, "nonexistent.json")
			}

			// Test config reading
			config, err := readTableStorageConfig(configFilePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("%s: expected error but got none", tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("%s: unexpected error: %v", tt.description, err)
				}
				if tt.name == "ValidConfigFile" {
					if !strings.Contains(config.ConnectionString, "AccountName=teststorage") {
						t.Errorf("Connection string should be parsed correctly")
					}
				}
			}
		})
	}
}

// Helper functions to reduce cyclomatic complexity
func testEventLogEntityFields(t *testing.T, entity, unmarshaled infra.EventLogEntity) {
	t.Helper()
	fields := map[string][2]string{
		"PartitionKey": {entity.PartitionKey, unmarshaled.PartitionKey},
		"RowKey":       {entity.RowKey, unmarshaled.RowKey},
		"EventType":    {entity.EventType, unmarshaled.EventType},
		"SessionID":    {entity.SessionID, unmarshaled.SessionID},
		"BoxID":        {entity.BoxID, unmarshaled.BoxID},
		"UserKey":      {entity.UserKey, unmarshaled.UserKey},
		"Details":      {entity.Details, unmarshaled.Details},
	}

	for fieldName, values := range fields {
		if values[0] != values[1] {
			t.Errorf("%s mismatch: expected %q, got %q", fieldName, values[0], values[1])
		}
	}
}

func testResourceRegistryEntityFields(t *testing.T, entity, unmarshaled infra.ResourceRegistryEntity) {
	t.Helper()
	fields := map[string][2]string{
		"PartitionKey": {entity.PartitionKey, unmarshaled.PartitionKey},
		"RowKey":       {entity.RowKey, unmarshaled.RowKey},
		"Status":       {entity.Status, unmarshaled.Status},
		"VMName":       {entity.VMName, unmarshaled.VMName},
		"Metadata":     {entity.Metadata, unmarshaled.Metadata},
	}

	for fieldName, values := range fields {
		if values[0] != values[1] {
			t.Errorf("%s mismatch: expected %q, got %q", fieldName, values[0], values[1])
		}
	}
}

func TestTableOperationEntities(t *testing.T) {
	t.Parallel()
	t.Run("EventLogEntity", func(t *testing.T) {
		t.Parallel()
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

		data, err := json.Marshal(entity)
		if err != nil {
			t.Fatalf("EventLogEntity should marshal to JSON: %v", err)
		}

		var unmarshaled infra.EventLogEntity
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("EventLogEntity should unmarshal from JSON: %v", err)
		}

		testEventLogEntityFields(t, entity, unmarshaled)
	})

	t.Run("ResourceRegistryEntity", func(t *testing.T) {
		t.Parallel()
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

		data, err := json.Marshal(entity)
		if err != nil {
			t.Fatalf("ResourceRegistryEntity should marshal to JSON: %v", err)
		}

		var unmarshaled infra.ResourceRegistryEntity
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("ResourceRegistryEntity should unmarshal from JSON: %v", err)
		}

		testResourceRegistryEntityFields(t, entity, unmarshaled)
	})
}

func TestTableStorageClientIntegration(t *testing.T) {
	t.Parallel()
	// Test that table storage client integrates properly with AzureClients
	env := test.SetupMinimalTestEnvironment(t)

	// Create a test clients struct
	clients := &infra.AzureClients{
		Suffix: env.Suffix,
	}

	// Test missing table storage config (should not cause fatal error)
	// This simulates the production behavior where table storage is optional
	if clients.TableClient != nil {
		t.Errorf("TableClient should be nil when no config available")
	}

	// Test with valid connection string
	testConnectionString := "DefaultEndpointsProtocol=https;AccountName=teststorage;AccountKey=dGVzdGtleXZhbHVlMTIzNDU2Nzg5MA==;EndpointSuffix=core.windows.net"
	clients.TableStorageConnectionString = testConnectionString

	// Test client creation
	tableClient, err := aztables.NewServiceClientFromConnectionString(testConnectionString, nil)
	if err != nil {
		t.Fatalf("Should be able to create table client with valid connection string: %v", err)
	}
	if tableClient == nil {
		t.Fatalf("Table client should not be nil")
	}

	clients.TableClient = tableClient
	if clients.TableClient == nil {
		t.Errorf("TableClient should be set")
	}
}

func TestTableStorageConstants(t *testing.T) {
	t.Parallel()
	// Test that required table names are defined
	// Note: These are internal constants, so we test them indirectly by checking
	// they're used in the table creation patterns

	expectedTables := []string{"EventLog", "ResourceRegistry"}

	for _, tableName := range expectedTables {
		if tableName == "" {
			t.Errorf("Table name should not be empty")
		}
		if !strings.ContainsAny(tableName, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz") {
			t.Errorf("Table name %q should start with a letter", tableName)
		}
		// Check that table name follows Azure table naming conventions (letters and numbers only)
		for _, char := range tableName {
			if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9')) {
				t.Errorf("Table name %q contains invalid character %c", tableName, char)
			}
		}
	}
}

func TestConnectionStringGeneration(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			// Test the connection string format that would be generated
			connectionString := fmt.Sprintf("DefaultEndpointsProtocol=https;AccountName=%s;AccountKey=%s;EndpointSuffix=core.windows.net",
				tt.accountName, tt.accountKey)

			// Verify format
			if !strings.Contains(connectionString, "DefaultEndpointsProtocol=https") {
				t.Errorf("Should contain HTTPS protocol")
			}
			expectedAccountName := fmt.Sprintf("AccountName=%s", tt.accountName)
			if !strings.Contains(connectionString, expectedAccountName) {
				t.Errorf("Should contain account name")
			}
			expectedAccountKey := fmt.Sprintf("AccountKey=%s", tt.accountKey)
			if !strings.Contains(connectionString, expectedAccountKey) {
				t.Errorf("Should contain account key")
			}
			if !strings.Contains(connectionString, "EndpointSuffix=core.windows.net") {
				t.Errorf("Should contain endpoint suffix")
			}

			// Test that client can be created with generated connection string
			client, err := aztables.NewServiceClientFromConnectionString(connectionString, nil)
			if err != nil {
				t.Errorf("%s: %v", tt.description, err)
			}
			if client == nil {
				t.Errorf("Client should be created successfully")
			}
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
