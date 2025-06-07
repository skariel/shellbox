package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// ResourceInfo represents basic information about an Azure resource
type ResourceInfo struct {
	ID         string
	Name       string
	Location   string
	Tags       map[string]string
	LastUsed   *time.Time
	CreatedAt  *time.Time
	Status     string
	Role       string
	ResourceID string // For volumes, this is the volumeID; for instances, this is instanceID
}

// ResourceCounts represents counts of resources by status
type ResourceCounts struct {
	Free      int
	Connected int
	Attached  int
	Total     int
}

// ResourceGraphQueries provides centralized Azure Resource Graph query operations
type ResourceGraphQueries struct {
	client         *armresourcegraph.Client
	subscriptionID string
	resourceGroup  string
}

// NewResourceGraphQueries creates a new Resource Graph query handler
func NewResourceGraphQueries(client *armresourcegraph.Client, subscriptionID, resourceGroup string) *ResourceGraphQueries {
	return &ResourceGraphQueries{
		client:         client,
		subscriptionID: subscriptionID,
		resourceGroup:  resourceGroup,
	}
}

// KQL query templates
const (
	// Count resources by status
	queryCountByStatus = `Resources
| where type =~ '%s'
| where tags['%s'] =~ '%s'
| where resourceGroup =~ '%s'
| summarize count() by tostring(tags['%s'])`

	// Get resources by status with details
	queryResourcesByStatus = `Resources
| where type =~ '%s'
| where tags['%s'] =~ '%s'
| where tags['%s'] =~ '%s'
| where resourceGroup =~ '%s'
| project name, id, tags, location`

	// Get oldest free resources for scale-down
	queryOldestFreeResources = `Resources
| where type =~ '%s'
| where tags['%s'] =~ '%s'
| where tags['%s'] =~ 'free'
| where resourceGroup =~ '%s'
| project name, id, tags, location, lastused=todatetime(tags['%s'])
| order by lastused asc
| take %d`

	// Get all resources of a specific role
	queryResourcesByRole = `Resources
| where type =~ '%s'
| where tags['%s'] =~ '%s'
| where resourceGroup =~ '%s'
| project name, id, tags, location`
)

// CountInstancesByStatus returns count of instances grouped by status
func (rq *ResourceGraphQueries) CountInstancesByStatus(ctx context.Context) (*ResourceCounts, error) {
	query := fmt.Sprintf(queryCountByStatus,
		AzureResourceTypeVM,
		TagKeyRole,
		ResourceRoleInstance,
		rq.resourceGroup,
		TagKeyStatus)

	return rq.executeCountQuery(ctx, query)
}

// CountVolumesByStatus returns count of volumes grouped by status
func (rq *ResourceGraphQueries) CountVolumesByStatus(ctx context.Context) (*ResourceCounts, error) {
	query := fmt.Sprintf(queryCountByStatus,
		AzureResourceTypeDisk,
		TagKeyRole,
		ResourceRoleVolume,
		rq.resourceGroup,
		TagKeyStatus)

	return rq.executeCountQuery(ctx, query)
}

// GetInstancesByStatus returns instances with specific status
func (rq *ResourceGraphQueries) GetInstancesByStatus(ctx context.Context, status string) ([]ResourceInfo, error) {
	query := fmt.Sprintf(queryResourcesByStatus,
		AzureResourceTypeVM,
		TagKeyRole,
		ResourceRoleInstance,
		TagKeyStatus,
		status,
		rq.resourceGroup)

	return rq.executeResourceQuery(ctx, query)
}

// GetVolumesByStatus returns volumes with specific status
func (rq *ResourceGraphQueries) GetVolumesByStatus(ctx context.Context, status string) ([]ResourceInfo, error) {
	query := fmt.Sprintf(queryResourcesByStatus,
		AzureResourceTypeDisk,
		TagKeyRole,
		ResourceRoleVolume,
		TagKeyStatus,
		status,
		rq.resourceGroup)

	return rq.executeResourceQuery(ctx, query)
}

// GetOldestFreeInstances returns oldest free instances for scale-down decisions
func (rq *ResourceGraphQueries) GetOldestFreeInstances(ctx context.Context, limit int) ([]ResourceInfo, error) {
	query := fmt.Sprintf(queryOldestFreeResources,
		AzureResourceTypeVM,
		TagKeyRole,
		ResourceRoleInstance,
		TagKeyStatus,
		rq.resourceGroup,
		TagKeyLastUsed,
		limit)

	return rq.executeResourceQuery(ctx, query)
}

// GetOldestFreeVolumes returns oldest free volumes for scale-down decisions
func (rq *ResourceGraphQueries) GetOldestFreeVolumes(ctx context.Context, limit int) ([]ResourceInfo, error) {
	query := fmt.Sprintf(queryOldestFreeResources,
		AzureResourceTypeDisk,
		TagKeyRole,
		ResourceRoleVolume,
		TagKeyStatus,
		rq.resourceGroup,
		TagKeyLastUsed,
		limit)

	return rq.executeResourceQuery(ctx, query)
}

// GetAllInstances returns all instances regardless of status
func (rq *ResourceGraphQueries) GetAllInstances(ctx context.Context) ([]ResourceInfo, error) {
	query := fmt.Sprintf(queryResourcesByRole,
		AzureResourceTypeVM,
		TagKeyRole,
		ResourceRoleInstance,
		rq.resourceGroup)

	return rq.executeResourceQuery(ctx, query)
}

// GetAllVolumes returns all volumes regardless of status
func (rq *ResourceGraphQueries) GetAllVolumes(ctx context.Context) ([]ResourceInfo, error) {
	query := fmt.Sprintf(queryResourcesByRole,
		AzureResourceTypeDisk,
		TagKeyRole,
		ResourceRoleVolume,
		rq.resourceGroup)

	return rq.executeResourceQuery(ctx, query)
}

// executeCountQuery executes a count query and returns ResourceCounts
func (rq *ResourceGraphQueries) executeCountQuery(ctx context.Context, query string) (*ResourceCounts, error) {
	slog.Debug("executing Resource Graph count query", "query", query)

	result, err := rq.client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: to.Ptr(query),
		Subscriptions: []*string{
			to.Ptr(rq.subscriptionID),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("resource Graph count query failed for resource group %s: %w", rq.resourceGroup, err)
	}

	counts := &ResourceCounts{}
	// Parse the aggregation results
	if dataArray, ok := result.Data.([]interface{}); ok {
		for _, item := range dataArray {
			// Handle object format from Resource Graph aggregation
			if rowMap, ok := item.(map[string]interface{}); ok {
				// Extract status from the column named after the tag
				var status string
				var count int

				// Look for status in the tags column
				if statusVal, exists := rowMap["tags_shellbox:status"]; exists && statusVal != nil {
					status = fmt.Sprintf("%v", statusVal)
				}

				// Look for count in the count_ column
				if countVal, exists := rowMap["count_"]; exists && countVal != nil {
					if countFloat, ok := countVal.(float64); ok {
						count = int(countFloat)
					}
				}

				if status != "" && count > 0 {
					switch status {
					case ResourceStatusFree:
						counts.Free = count
					case ResourceStatusConnected:
						counts.Connected = count
					case ResourceStatusAttached:
						counts.Attached = count
					}
					counts.Total += count
				}
			}
		}
	}

	slog.Debug("Resource Graph count query result",
		"free", counts.Free,
		"connected", counts.Connected,
		"attached", counts.Attached,
		"total", counts.Total)

	return counts, nil
}

// executeResourceQuery executes a resource listing query and returns ResourceInfo slice
func (rq *ResourceGraphQueries) executeResourceQuery(ctx context.Context, query string) ([]ResourceInfo, error) {
	slog.Debug("executing Resource Graph resource query", "query", query)

	result, err := rq.client.Resources(ctx, armresourcegraph.QueryRequest{
		Query: to.Ptr(query),
		Subscriptions: []*string{
			to.Ptr(rq.subscriptionID),
		},
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("resource Graph resource query failed for resource group %s: %w", rq.resourceGroup, err)
	}

	var resources []ResourceInfo

	// Parse the resource results
	if result.Data != nil {
		if dataArray, ok := result.Data.([]interface{}); ok {
			for _, item := range dataArray {
				if resourceMap, ok := item.(map[string]interface{}); ok {
					resource := ParseResourceInfo(resourceMap)
					if resource != nil {
						resources = append(resources, *resource)
					}
				}
			}
		}
	}

	slog.Debug("Resource Graph resource query result", "count", len(resources))
	return resources, nil
}

// parseResourceInfo converts Resource Graph result map to ResourceInfo struct
func ParseResourceInfo(resourceMap map[string]interface{}) *ResourceInfo {
	resource := &ResourceInfo{}

	ParseBasicFields(resource, resourceMap)
	ParseTags(resource, resourceMap)
	ParseProjectedFields(resource, resourceMap)

	return resource
}

// parseBasicFields extracts basic resource fields
func ParseBasicFields(resource *ResourceInfo, resourceMap map[string]interface{}) {
	if name, ok := resourceMap["name"].(string); ok {
		resource.Name = name
	}
	if id, ok := resourceMap["id"].(string); ok {
		resource.ID = id
	}
	if location, ok := resourceMap["location"].(string); ok {
		resource.Location = location
	}
}

// parseTags extracts and processes resource tags
func ParseTags(resource *ResourceInfo, resourceMap map[string]interface{}) {
	tagsInterface, ok := resourceMap["tags"]
	if !ok {
		return
	}

	tagsMap, ok := tagsInterface.(map[string]interface{})
	if !ok {
		return
	}

	resource.Tags = make(map[string]string)
	for k, v := range tagsMap {
		if vStr, ok := v.(string); ok {
			resource.Tags[k] = vStr
		}
	}

	extractTagValues(resource)
	parseTimestamps(resource)
	extractResourceID(resource)
}

// extractTagValues extracts specific tag values
func extractTagValues(resource *ResourceInfo) {
	resource.Status = resource.Tags[TagKeyStatus]
	resource.Role = resource.Tags[TagKeyRole]
}

// parseTimestamps parses timestamp strings from tags
func parseTimestamps(resource *ResourceInfo) {
	if createdStr := resource.Tags[TagKeyCreated]; createdStr != "" {
		if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
			resource.CreatedAt = &created
		}
	}
	if lastUsedStr := resource.Tags[TagKeyLastUsed]; lastUsedStr != "" {
		if lastUsed, err := time.Parse(time.RFC3339, lastUsedStr); err == nil {
			resource.LastUsed = &lastUsed
		}
	}
}

// extractResourceID extracts resource ID from tags (instanceID or volumeID)
func extractResourceID(resource *ResourceInfo) {
	if instanceID := resource.Tags["instanceID"]; instanceID != "" {
		resource.ResourceID = instanceID
	}
	if volumeID := resource.Tags["volumeID"]; volumeID != "" {
		resource.ResourceID = volumeID
	}
}

// parseProjectedFields parses fields projected by specific queries
func ParseProjectedFields(resource *ResourceInfo, resourceMap map[string]interface{}) {
	// Parse lastused from query projection (for oldest resource queries)
	if lastUsedInterface, ok := resourceMap["lastused"]; ok {
		if lastUsedStr, ok := lastUsedInterface.(string); ok {
			if lastUsed, err := time.Parse(time.RFC3339, lastUsedStr); err == nil {
				resource.LastUsed = &lastUsed
			}
		}
	}
}
