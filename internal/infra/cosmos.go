package infra

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Container represents a CosmosDB container with its partition key
type Container struct {
	Name         string
	PartitionKey string
	Throughput   int
}

// CosmosDBResult contains the result of the CosmosDB creation operation
type CosmosDBResult struct {
	ConnectionString string
	Error            error
}

// CreateCosmosDBResources creates a CosmosDB account, database, and containers
// Parameters:
// - resourceGroup: Azure resource group name
// - region: Azure region (e.g., "westus")
// - accountName: CosmosDB account name (must be globally unique)
// - databaseName: CosmosDB database name
// - containers: List of containers to create with their partition keys
func CreateCosmosDBResources(resourceGroup, region, accountName, databaseName string, containers []Container) CosmosDBResult {
	result := CosmosDBResult{}

	// Step 1: Create CosmosDB account
	cmd := exec.Command("az", "cosmosdb", "create",
		"--name", accountName,
		"--resource-group", resourceGroup,
		"--kind", "GlobalDocumentDB",
		"--default-consistency-level", "Session",
		"--locations", fmt.Sprintf("regionName=%s", region))

	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("failed to create CosmosDB account: %v, output: %s", err, string(output))
		return result
	}

	// Step 2: Create database
	cmd = exec.Command("az", "cosmosdb", "sql", "database", "create",
		"--account-name", accountName,
		"--resource-group", resourceGroup,
		"--name", databaseName)

	output, err = cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("failed to create database: %v, output: %s", err, string(output))
		return result
	}

	// Step 3: Create containers
	for _, container := range containers {
		cmd = exec.Command("az", "cosmosdb", "sql", "container", "create",
			"--account-name", accountName,
			"--database-name", databaseName,
			"--name", container.Name,
			"--resource-group", resourceGroup,
			"--partition-key-path", container.PartitionKey,
			"--throughput", fmt.Sprintf("%d", container.Throughput))

		output, err = cmd.CombinedOutput()
		if err != nil {
			result.Error = fmt.Errorf("failed to create container %s: %v, output: %s", container.Name, err, string(output))
			return result
		}
	}

	// Step 4: Get connection string
	cmd = exec.Command("az", "cosmosdb", "keys", "list",
		"--name", accountName,
		"--resource-group", resourceGroup,
		"--type", "connection-strings",
		"--query", "connectionStrings[?description=='Primary SQL Connection String'].connectionString",
		"-o", "tsv")

	output, err = cmd.CombinedOutput()
	if err != nil {
		result.Error = fmt.Errorf("failed to get connection string: %v, output: %s", err, string(output))
		return result
	}

	result.ConnectionString = strings.TrimSpace(string(output))
	return result
}
