package infra

import (
	"fmt"
	"os/exec"
	"strings"
)

// Table represents an Azure Table Storage table
type Table struct {
	Name string
}

// TableStorageResult contains the result of the Table Storage creation operation
type TableStorageResult struct {
	ConnectionString string
	Error            error
}

// CreateTableStorageResources creates a Storage Account and tables
// Parameters:
// - resourceGroup: Azure resource group name
// - region: Azure region (e.g., "westus")
// - accountName: Storage account name (must be globally unique)
// - tables: List of tables to create
func CreateTableStorageResources(resourceGroup, region, accountName string, tables []Table) TableStorageResult {
	result := TableStorageResult{}

	// Step 1: Create Storage Account
	cmd := exec.Command("az", "storage", "account", "create",
		"--name", accountName,
		"--resource-group", resourceGroup,
		"--location", region,
		"--sku", "Standard_LRS",
		"--kind", "StorageV2")

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("failed to create Storage Account: %v, output: %s", err, string(output))
		return result
	}

	// Step 2: Get connection string
	cmd = exec.Command("az", "storage", "account", "show-connection-string",
		"--name", accountName,
		"--resource-group", resourceGroup,
		"--output", "tsv")

	output, err = cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("failed to get connection string: %v, output: %s", err, string(output))
		return result
	}

	result.ConnectionString = strings.TrimSpace(string(output))

	// Step 3: Create tables
	for _, table := range tables {
		cmd = exec.Command("az", "storage", "table", "create",
			"--name", table.Name,
			"--account-name", accountName,
			"--connection-string", result.ConnectionString)

		output, err = cmd.CombinedOutput()
		if err != nil {
			result.Error = fmt.Errorf("failed to create table %s: %v, output: %s", table.Name, err, string(output))
			return result
		}
	}

	return result
}
