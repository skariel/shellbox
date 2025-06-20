package infra

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// waitForVolumeInResourceGraph waits for a volume to be visible in Resource Graph with expected tags
func waitForVolumeInResourceGraph(ctx context.Context, clients *AzureClients, volumeID string, expectedTags VolumeTags) error {
	// Create resource graph queries client
	rq := NewResourceGraphQueries(clients.ResourceGraphClient, clients.SubscriptionID, clients.ResourceGroupName)

	// Define the verification operation
	verifyOperation := func(ctx context.Context) error {
		slog.Debug("Checking Resource Graph for volume", "volumeID", volumeID, "expectedStatus", expectedTags.Status)

		// Get all volumes with the expected status
		volumes, err := rq.GetVolumesByStatus(ctx, expectedTags.Status)
		if err != nil {
			return fmt.Errorf("querying volumes: %w", err)
		}

		// Check if our volume is in the results
		for _, volume := range volumes {
			if volume.Tags[TagKeyVolumeID] == volumeID {
				// Verify all expected tags are present
				if volume.Tags[TagKeyRole] == expectedTags.Role &&
					volume.Tags[TagKeyStatus] == expectedTags.Status &&
					volume.Tags[TagKeyCreated] == expectedTags.CreatedAt &&
					volume.Tags[TagKeyLastUsed] == expectedTags.LastUsed {
					slog.Info("Volume visible in Resource Graph", "volumeID", volumeID)
					return nil
				}
			}
		}

		// Volume not found yet
		return fmt.Errorf("volume %s not yet visible in Resource Graph (checked %d volumes with status %s)", volumeID, len(volumes), expectedTags.Status)
	}

	// Use RetryOperation with a 2-minute timeout and 5-second intervals
	const (
		timeout  = 2 * time.Minute
		interval = 5 * time.Second
	)

	return RetryOperation(ctx, verifyOperation, timeout, interval, "wait for volume in Resource Graph")
}

// waitForVolumeTagsInResourceGraph waits for a volume's tags to be updated in Resource Graph
func waitForVolumeTagsInResourceGraph(ctx context.Context, clients *AzureClients, volumeID string, expectedTags map[string]string) error {
	// Create resource graph queries client
	rq := NewResourceGraphQueries(clients.ResourceGraphClient, clients.SubscriptionID, clients.ResourceGroupName)

	// Define the verification operation
	verifyOperation := func(ctx context.Context) error {
		slog.Debug("Checking Resource Graph for volume tag updates", "volumeID", volumeID, "expectedStatus", expectedTags[TagKeyStatus])

		// Get all volumes with the expected status
		volumes, err := rq.GetVolumesByStatus(ctx, expectedTags[TagKeyStatus])
		if err != nil {
			return fmt.Errorf("querying volumes: %w", err)
		}

		// Check if our volume is in the results with expected tags
		for _, volume := range volumes {
			if volume.Tags[TagKeyVolumeID] == volumeID {
				// Verify all expected tags are present
				allTagsMatch := true
				for key, expectedValue := range expectedTags {
					if volume.Tags[key] != expectedValue {
						allTagsMatch = false
						break
					}
				}

				if allTagsMatch {
					slog.Info("Volume tags updated in Resource Graph", "volumeID", volumeID)
					return nil
				}
			}
		}

		// Volume with expected tags not found yet
		return fmt.Errorf("volume %s with updated tags not yet visible in Resource Graph (checked %d volumes with status %s)", volumeID, len(volumes), expectedTags[TagKeyStatus])
	}

	// Use RetryOperation with a 2-minute timeout and 5-second intervals
	const (
		timeout  = 2 * time.Minute
		interval = 5 * time.Second
	)

	return RetryOperation(ctx, verifyOperation, timeout, interval, "wait for volume tags in Resource Graph")
}

// waitForInstanceTagsInResourceGraph waits for an instance's tags to be updated in Resource Graph
func waitForInstanceTagsInResourceGraph(ctx context.Context, clients *AzureClients, instanceID string, expectedTags map[string]string) error {
	// Create resource graph queries client
	rq := NewResourceGraphQueries(clients.ResourceGraphClient, clients.SubscriptionID, clients.ResourceGroupName)

	// Define the verification operation
	verifyOperation := func(ctx context.Context) error {
		slog.Debug("Checking Resource Graph for instance tag updates", "instanceID", instanceID, "expectedStatus", expectedTags[TagKeyStatus])

		// Get all instances with the expected status
		instances, err := rq.GetInstancesByStatus(ctx, expectedTags[TagKeyStatus])
		if err != nil {
			return fmt.Errorf("querying instances: %w", err)
		}

		// Check if our instance is in the results with expected tags
		for _, instance := range instances {
			if instance.Tags[TagKeyInstanceID] == instanceID {
				// Verify all expected tags are present
				allTagsMatch := true
				for key, expectedValue := range expectedTags {
					if instance.Tags[key] != expectedValue {
						allTagsMatch = false
						break
					}
				}

				if allTagsMatch {
					slog.Info("Instance tags updated in Resource Graph", "instanceID", instanceID)
					return nil
				}
			}
		}

		// Instance with expected tags not found yet
		return fmt.Errorf("instance %s with updated tags not yet visible in Resource Graph (checked %d instances with status %s)", instanceID, len(instances), expectedTags[TagKeyStatus])
	}

	// Use RetryOperation with a 2-minute timeout and 5-second intervals
	const (
		timeout  = 2 * time.Minute
		interval = 5 * time.Second
	)

	return RetryOperation(ctx, verifyOperation, timeout, interval, "wait for instance tags in Resource Graph")
}
